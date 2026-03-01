import { expect, type Locator, type Page } from "@playwright/test";

export const DEFAULT_ADMIN_EMAIL = "admin@incidenthub.local";
export const DEFAULT_ADMIN_PASSWORD = "ChangeMeNow123!";

export async function uiLogin(page: Page, email: string, password: string) {
  await page.goto("/login");

  await page.getByTestId("login-email").fill(email);
  await page.getByTestId("login-password").fill(password);
  await page.getByTestId("login-submit").click();

  await page.waitForURL(/\/dashboard/);
  await expect(page.getByTestId("dashboard-livestream-widget")).toBeVisible();
}

export function parseEntityIdFromURL(page: Page): string {
  const url = new URL(page.url());
  const parts = url.pathname.split("/").filter(Boolean);
  return String(parts[parts.length - 1] || "").trim();
}

export async function selectRadixOptionByText(trigger: Locator, optionText: string | RegExp) {
  await trigger.click();
  const page = trigger.page();
  const option = page.locator("[role=option]", { hasText: optionText }).first();
  await option.click();
}

export async function selectRadixOptionWithSearch(trigger: Locator, query: string, optionText: string | RegExp) {
  await trigger.click();
  const page = trigger.page();
  await page.locator('input[placeholder*="Search"]').first().fill(query);
  const option = page.locator("[role=option]", { hasText: optionText }).first();
  await option.click();
}
