import { expect, test } from "@playwright/test";
import { receiptPng } from "./helpers";

// Two months back stays open after the desktop flow; the first Tuesday
// after the 7th is a working day in every month (no BW holiday falls there).
const now = new Date();
const first = new Date(Date.UTC(now.getFullYear(), now.getMonth() - 2, 8));
while (first.getUTCDay() !== 2) first.setUTCDate(first.getUTCDate() + 1);
const date = first.toISOString().slice(0, 10);
const month = date.slice(0, 7);
const shown = `${date.slice(8, 10)}.${date.slice(5, 7)}.${date.slice(0, 4)}`;

test("proving working days counts a day until it is marked as vacation", async ({ page }) => {
  page.on("dialog", (d) => d.accept());
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  await page
    .getByTestId("receipt-input")
    .first()
    .setInputFiles({ name: "bon.png", mimeType: "image/png", buffer: receiptPng(9) });
  await expect(page.locator("#date")).toBeVisible();
  await page.fill("#date", date);
  await page.fill("#total", "8,00");
  await page.getByRole("button", { name: "Prüfen & weiter" }).click();
  await expect(page.getByText("Beleg geprüft").first()).toBeVisible();

  await page.goto(`/month/${month}`);
  // The box follows the server's answer, so click and wait for it.
  await page.getByLabel("Arbeitstage nachweisen").click();
  await expect(page.getByLabel("Arbeitstage nachweisen")).toBeChecked();
  await expect(page.getByText(/1 von \d+ Arbeitstagen/)).toBeVisible();
  await expect(page.getByText("7,67 €").first()).toBeVisible();

  await page.getByRole("button", { name: `${shown}: Arbeitstag` }).click();
  await expect(page.getByRole("button", { name: `${shown}: Urlaub` })).toBeVisible();
  await expect(page.getByText("Kein Arbeitstag, nicht angesetzt")).toBeVisible();
  await expect(page.getByText(/0 von \d+ Arbeitstagen/)).toBeVisible();
});
