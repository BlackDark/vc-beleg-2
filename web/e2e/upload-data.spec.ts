import { expect, test } from "@playwright/test";

// The first of the current month is never in the future and its month is still open.
const day = new Date().toISOString().slice(0, 7);

// Runs after flow.spec.ts, which creates the account.
test("a JSON data file creates data receipts to check; invalid files create none", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);
  await page.goto("/receipts");
  const input = page.getByTestId("receipt-input").first();

  // Invalid entries: the whole file is rejected with one line per problem.
  await input.setInputFiles({
    name: "bad.json",
    mimeType: "application/json",
    buffer: Buffer.from(
      JSON.stringify([
        { date: `${day}-01`, total: 5, merchant: "Gültig" },
        { date: "01.01.2031", total: "abc", merchant: "Kaputt" },
      ]),
    ),
  });
  const toast = page.locator("[data-sonner-toast]").first();
  await expect(toast).toContainText("Es wurde nichts importiert");
  await expect(toast).toContainText("Eintrag 2: Das Datum muss JJJJ-MM-TT sein.");
  await expect(toast).toContainText("Eintrag 2: Der Betrag muss eine Dezimalzahl in EUR sein");
  await expect(page).toHaveURL(/\/receipts$/);
  await expect(page.getByText("Gültig")).toHaveCount(0);

  // A valid array: one receipt per entry, to be checked like any other.
  const data = JSON.stringify([
    { date: `${day}-01`, total: 12.5, merchant: "Datenkantine", extra: { Tisch: 4 } },
    { date: `${day}-01`, total: "7.85", merchant: "Datenbäcker" },
  ]);
  await input.setInputFiles({ name: "tag.json", mimeType: "application/json", buffer: Buffer.from(data) });
  await expect(page).toHaveURL(/\/receipts\/\d+/);
  await expect(page.locator("#merchant")).toHaveValue("Datenkantine");
  await expect(page.locator("#total")).toHaveValue("12,50");
  await expect(page.getByText("Aus einer Datendatei erstellt")).toBeVisible();
  await expect(page.getByRole("link", { name: /Datensatz öffnen/ })).toBeVisible();
  // The rendered data receipt is a real image.
  const img = page.getByRole("img").first();
  await expect(img).toBeVisible();
  expect(await img.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth > 0)).toBe(true);
  await page.screenshot({ path: "/tmp/pr30-check-desktop.png" });
  await page.setViewportSize({ width: 360, height: 780 });
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
  await page.screenshot({ path: "/tmp/pr30-check-360.png" });
  await page.setViewportSize({ width: 1280, height: 720 });

  // "Weiter" goes to any pending receipt (other specs leave some), so open the second one directly.
  await page.getByRole("button", { name: "Prüfen & weiter" }).click();
  await expect(page).toHaveURL(/\/receipts(\/\d+)?$/);
  await page.goto("/receipts");
  await page.getByRole("link", { name: /Datenbäcker/ }).click();
  await expect(page.locator("#merchant")).toHaveValue("Datenbäcker");

  // Uploading the same entries again is flagged as a duplicate.
  await page.goto("/receipts");
  await input.setInputFiles({ name: "again.json", mimeType: "application/json", buffer: Buffer.from(data) });
  await expect(page).toHaveURL(/\/receipts\/\d+/);
  await expect(page.getByText("Diese Datei wurde schon einmal hochgeladen")).toBeVisible();

  await page.goto("/receipts");
  await expect(page.getByText("Datenkantine").first()).toBeVisible();
  const overflowList = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflowList).toBeLessThanOrEqual(0);
});
