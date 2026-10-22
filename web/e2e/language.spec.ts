import { expect, test } from "@playwright/test";

// Runs after the desktop flow, so the account exists.
test("switching to English translates the UI and keeps the choice", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.goto("/settings");
  await page.selectOption("#language", "en");
  await expect(page.locator("main").getByText("Company and taxation")).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");

  await page.goto("/");
  await expect(page.locator("main").getByText("Your meal allowance this month")).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Main navigation" }).first()).toBeAttached();
});
