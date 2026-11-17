import { expect, test } from "@playwright/test";
import { loginOwner } from "./support";

test("owner area asks for the password when there is no session", async ({
  page,
}) => {
  await page.goto("/owner");

  await expect(page).toHaveURL(/\/owner\/login$/);
  await expect(
    page.getByRole("heading", { name: "Вход владельца" }),
  ).toBeVisible();
  await expect(page.getByLabel("Пароль")).toBeVisible();
});

test("owner API rejects requests without a session", async ({ request }) => {
  const bookings = await request.get("/api/v1/bookings");
  expect(bookings.status()).toBe(401);
  expect(await bookings.json()).toMatchObject({ code: "unauthorized" });

  const created = await request.post("/api/v1/meeting-types", {
    data: {
      id: "unauthorized-attempt",
      name: "Без входа",
      description: "",
      durationMinutes: 30,
    },
  });
  expect(created.status()).toBe(401);

  const meetingTypes = await request.get("/api/v1/meeting-types");
  expect(meetingTypes.status()).toBe(200);
  expect(await meetingTypes.text()).not.toContain("unauthorized-attempt");
});

test("owner signs out and loses access", async ({ page }) => {
  await loginOwner(page);

  await page.getByRole("button", { name: "Выйти" }).click();
  await expect(page).toHaveURL(/\/owner\/login$/);
  await expect(page.getByLabel("Пароль")).toBeVisible();

  await page.goto("/owner");
  await expect(page).toHaveURL(/\/owner\/login$/);
});
