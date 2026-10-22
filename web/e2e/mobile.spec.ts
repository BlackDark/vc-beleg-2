import { expect, test } from "@playwright/test";

// Runs after the desktop flow, so the account and a closed month exist.
test("every screen fits 360 px without horizontal scrolling", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);
  for (const path of ["/", "/receipts", "/month", "/reports", "/settings", "/receipts/1"]) {
    await page.goto(path);
    await page.waitForLoadState("networkidle");
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow, path).toBeLessThanOrEqual(0);
  }
  await expect(page.getByRole("navigation", { name: "Hauptnavigation" }).last()).toBeVisible();
});
