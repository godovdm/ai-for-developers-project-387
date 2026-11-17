import { fireEvent, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { jsonResponse, renderApp } from "../test/utils";

beforeEach(() => {
  window.history.replaceState({}, "", "/");
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.href;
  return input.url;
}

function noContent(): Response {
  return new Response(null, { status: 204 });
}

interface Session {
  required: boolean;
  authenticated: boolean;
}

function stubApi(initial: Session) {
  let session = initial;
  const fetchMock = vi.fn<typeof fetch>().mockImplementation((input, init) => {
    const method = init?.method ?? "GET";
    const url = requestUrl(input);
    if (url.endsWith("/api/v1/owner/session") && method === "GET") {
      return Promise.resolve(jsonResponse(session));
    }
    if (url.endsWith("/api/v1/owner/session") && method === "POST") {
      const body: unknown =
        typeof init?.body === "string" ? JSON.parse(init.body) : {};
      if (
        typeof body === "object" &&
        body !== null &&
        "password" in body &&
        body.password === "correct"
      ) {
        session = { required: true, authenticated: true };
        return Promise.resolve(noContent());
      }
      return Promise.resolve(
        jsonResponse(
          { code: "invalid_credentials", message: "Неверный пароль" },
          401,
        ),
      );
    }
    if (url.endsWith("/api/v1/owner/session") && method === "DELETE") {
      session = { required: true, authenticated: false };
      return Promise.resolve(noContent());
    }
    if (url.endsWith("/api/v1/meeting-types") && method === "GET") {
      return Promise.resolve(jsonResponse({ items: [] }));
    }
    if (url.endsWith("/api/v1/bookings") && method === "GET") {
      return Promise.resolve(jsonResponse({ items: [] }));
    }
    return Promise.resolve(jsonResponse({ message: "not found" }, 404));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

it("redirects to the login page when protection is on and there is no session", async () => {
  stubApi({ required: true, authenticated: false });

  renderApp("/owner");

  expect(
    await screen.findByRole("heading", { name: "Вход владельца" }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe("/owner/login");
  expect(await screen.findByLabelText(/Пароль/)).toBeInTheDocument();
});

it("signs in with the correct password and opens the owner area", async () => {
  const fetchMock = stubApi({ required: true, authenticated: false });

  renderApp("/owner/login");

  fireEvent.change(await screen.findByLabelText(/Пароль/), {
    target: { value: "correct" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Войти" }));

  expect(
    await screen.findByRole("button", { name: "Создать тип встречи" }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe("/owner");

  const postCall = fetchMock.mock.calls.find(
    ([, init]) => init?.method === "POST",
  );
  expect(postCall).toBeDefined();
  const [, init] = postCall ?? [];
  expect(typeof init?.body === "string" ? init.body : "").toContain(
    '"password":"correct"',
  );
  expect(JSON.stringify(init?.headers ?? {})).not.toContain("correct");
  expect(window.location.search).toBe("");
});

it("shows the server message for a wrong password and stays on the login page", async () => {
  stubApi({ required: true, authenticated: false });

  renderApp("/owner/login");

  fireEvent.change(await screen.findByLabelText(/Пароль/), {
    target: { value: "wrong" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Войти" }));

  expect(await screen.findByText("Неверный пароль")).toBeInTheDocument();
  expect(window.location.pathname).toBe("/owner/login");
});

it("explains the demonstration mode on the login page", async () => {
  stubApi({ required: false, authenticated: true });

  renderApp("/owner/login");

  expect(
    await screen.findByText(
      "Пароль не настроен, защита владельческих операций выключена.",
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Перейти в область владельца" }),
  ).toHaveAttribute("href", "/owner");
  expect(screen.queryByLabelText(/Пароль/)).not.toBeInTheDocument();
});

it("opens the owner area in the demonstration mode with a visible warning", async () => {
  stubApi({ required: false, authenticated: true });

  renderApp("/owner");

  expect(
    await screen.findByText(
      "Пароль не настроен, защита владельческих операций выключена.",
    ),
  ).toBeInTheDocument();
  expect(await screen.findByText("Пока нет типов встреч")).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Выйти" }),
  ).not.toBeInTheDocument();
});

it("returns to the login page when an owner operation answers 401", async () => {
  let authenticated = true;
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>().mockImplementation((input, init) => {
      const method = init?.method ?? "GET";
      const url = requestUrl(input);
      if (url.endsWith("/api/v1/owner/session") && method === "GET") {
        return Promise.resolve(jsonResponse({ required: true, authenticated }));
      }
      if (url.endsWith("/api/v1/meeting-types") && method === "GET") {
        return Promise.resolve(jsonResponse({ items: [] }));
      }
      if (url.endsWith("/api/v1/bookings") && method === "GET") {
        authenticated = false;
        return Promise.resolve(
          jsonResponse(
            { code: "unauthorized", message: "Требуется вход" },
            401,
          ),
        );
      }
      return Promise.resolve(jsonResponse({ message: "not found" }, 404));
    }),
  );

  renderApp("/owner");

  expect(
    await screen.findByRole("heading", { name: "Вход владельца" }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe("/owner/login");
  expect(
    screen.queryByText("Не удалось загрузить записи"),
  ).not.toBeInTheDocument();
});

it("signs out and requires the password again", async () => {
  stubApi({ required: true, authenticated: true });

  renderApp("/owner");

  fireEvent.click(await screen.findByRole("button", { name: "Выйти" }));

  expect(
    await screen.findByRole("heading", { name: "Вход владельца" }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe("/owner/login");
  expect(await screen.findByLabelText(/Пароль/)).toBeInTheDocument();
});
