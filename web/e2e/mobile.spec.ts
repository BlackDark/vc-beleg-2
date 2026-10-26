import { expect, test } from "@playwright/test";

// Runs after the desktop flow, so the account and a closed month exist.
test("every screen fits 360 px without horizontal scrolling", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);
  // Each screen proves it rendered before its width is measured.
  const screens: [string, RegExp][] = [
    ["/", /Dein Essenszuschuss in diesem Monat/],
    ["/receipts", /Keine Belege|zu prüfen|geprüft/],
    ["/month", /Zusammenfassung/],
    ["/reports", /Monatsberichte zum Archivieren/],
    ["/settings", /Firma und Versteuerung/],
    ["/settings?section=account", /^Profil$/],
    ["/settings?section=integrations", /^Mail-Eingang$/],
    ["/settings?section=imports", /^Import-Protokoll$/],
    ["/receipts/1", /Geprüft am/],
  ];
  for (const [path, marker] of screens) {
    await page.goto(path);
    await expect(page.locator("main").getByText(marker).first()).toBeVisible();
    await page.waitForLoadState("networkidle");
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow, path).toBeLessThanOrEqual(0);
  }
  await expect(page.getByRole("navigation", { name: "Hauptnavigation" }).last()).toBeVisible();
});
