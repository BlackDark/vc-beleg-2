import { expect, test } from "@playwright/test";
import { receiptPng } from "./helpers";

// Runs after the other desktop specs (file order), so the account exists.
test("undated receipts have their own view and appear in no month", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  await page
    .getByTestId("receipt-input")
    .first()
    .setInputFiles({
      name: "undated.png",
      mimeType: "image/png",
      buffer: receiptPng(77),
    });
  await expect(page).toHaveURL(/\/receipts\/\d+/);

  await page.goto("/receipts");
  const row = page.getByRole("link", { name: /undated\.png/ });
  await expect(page.getByRole("button", { name: "Alle" })).toHaveAttribute("aria-pressed", "true");
  await expect(row).toHaveCount(0);

  await page.getByRole("button", { name: "Ohne Datum" }).click();
  await expect(page).toHaveURL(/status=undated/);
  await expect(row).toBeVisible();
  // The view is not month-scoped, so there is no month switcher.
  await page.reload();
  await expect(row).toBeVisible();

  // The Start card still lists it, and the month summary links here.
  await page.goto("/");
  await expect(page.getByRole("link", { name: /undated\.png/ })).toBeVisible();
  await page.goto("/month");
  await page.getByRole("link", { name: "Ohne Datum anzeigen" }).click();
  await expect(page).toHaveURL(/status=undated/);
  await expect(row).toBeVisible();
});
