import { expect, test } from "@playwright/test";

// Runs after the desktop flow, so the account exists.
test("switching to English translates the UI and keeps the choice", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.goto("/settings");
  await page.getByRole("link", { name: "Integrationen", exact: true }).click();
  await expect(page.getByText("Mail-Eingang", { exact: true })).toBeVisible();
  await expect(page.getByText("KI-Belegleser", { exact: true })).toBeVisible();
  await expect(page.getByText("claude-opus-5-5", { exact: true })).toBeVisible();
  await expect(page.getByText("Niedrig", { exact: true })).toBeVisible();
  await expect(page.getByText("Ordner-Import", { exact: true })).toBeVisible();
  await expect(page.locator("#company")).toHaveCount(0);
  await page.getByRole("link", { name: "Import-Protokoll", exact: true }).click();
  await expect(page.getByText("importiert")).toHaveCount(5);
  await page.goBack();
  await expect(page.getByText("Mail-Eingang", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Konto & Darstellung", exact: true })).toBeVisible();
  await page.getByRole("link", { name: "Konto & Darstellung", exact: true }).click();
  await expect(page.locator("#company")).toHaveCount(0);
  await page.reload();
  await page.selectOption("#language", "en");
  await expect(page.locator("main").getByText("Profile", { exact: true })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");

  await page.goto("/");
  await expect(page.locator("main").getByText("Your meal allowance this month")).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Main navigation" }).first()).toBeAttached();
});
