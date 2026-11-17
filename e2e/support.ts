import { expect, type APIRequestContext, type Page } from "@playwright/test";

// Test-only value shared by the web server and the specs. Deployments must set
// OWNER_PASSWORD explicitly; no production default exists.
export const OWNER_PASSWORD =
  process.env.E2E_OWNER_PASSWORD ?? "e2e-owner-password";

export async function loginOwner(page: Page): Promise<void> {
  await page.goto("/owner/login");
  await page.getByLabel("Пароль").fill(OWNER_PASSWORD);
  await page.getByRole("button", { name: "Войти" }).click();
  await expect(
    page.getByRole("button", { name: "Создать тип встречи" }),
  ).toBeVisible();
}

export async function loginOwnerRequest(
  request: APIRequestContext,
): Promise<void> {
  const response = await request.post("/api/v1/owner/session", {
    data: { password: OWNER_PASSWORD },
  });
  expect(response.status()).toBe(204);
}
