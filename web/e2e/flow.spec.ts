import { expect, test } from "@playwright/test";
import { receiptPng } from "./helpers";

// Last month always lies completely in the past, so it can be closed.
const now = new Date();
const last = new Date(Date.UTC(now.getFullYear(), now.getMonth() - 1, 1));
const month = last.toISOString().slice(0, 7);

test("setup, receipts, check, close month and download the report", async ({ page }) => {
  page.on("dialog", (d) => d.accept());

  await page.goto("/");
  await expect(page).toHaveURL(/\/setup/);
  await page.fill("#token", "e2e-setup-token");
  await page.fill("#name", "Max Muster");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.click("button[type=submit]");

  await expect(page).toHaveURL(/\/settings/);
  await page.fill("#company", "Muster GmbH");
  await page
    .locator("form")
    .filter({ has: page.locator("#company") })
    .getByRole("button", { name: "Speichern" })
    .click();
  await expect(page.getByText("Gespeichert").first()).toBeVisible();

  // A previously checked receipt can be deleted until it is archived.
  await page.goto("/");
  await page
    .getByTestId("receipt-input")
    .first()
    .setInputFiles({
      name: "wrong.png",
      mimeType: "image/png",
      buffer: receiptPng(99),
    });
  await expect(page).toHaveURL(/\/receipts\/\d+/);
  const wrongReceipt = page.url();
  await page.fill("#date", `${month}-04`);
  await page.fill("#total", "5,00");
  await page.getByRole("button", { name: "Prüfen & weiter" }).click();
  await expect(page).toHaveURL(/\/receipts$/);
  await page.goto(wrongReceipt);
  await expect(page.getByRole("button", { name: "Beleg löschen", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: /Zurück auf/ }).click();
  await expect(page.getByRole("button", { name: "Prüfen & weiter" })).toBeVisible();
  page.removeAllListeners("dialog");
  page.once("dialog", (d) => d.dismiss());
  await page.getByRole("button", { name: "Beleg löschen", exact: true }).click();
  await expect(page).toHaveURL(wrongReceipt);
  await expect(page.locator("#date")).toHaveValue(`${month}-04`);
  page.on("dialog", (d) => d.accept());
  await page.getByRole("button", { name: "Beleg löschen", exact: true }).click();
  await expect(page).toHaveURL(/\/receipts$/);

  await page.goto("/");
  await page
    .getByTestId("receipt-input")
    .first()
    .setInputFiles(
      [1, 2, 3].map((n) => ({ name: `bon-${n}.png`, mimeType: "image/png", buffer: receiptPng(n) })),
    );
  await expect(page).toHaveURL(/\/receipts\/\d+/);

  const checkedReceipts: string[] = [];
  for (const [day, total] of [
    ["05", "7,85"],
    ["06", "9,10"],
    ["07", "6,00"],
  ]) {
    await expect(page.locator("#date")).toBeVisible();
    checkedReceipts.push(page.url());
    await page.fill("#date", `${month}-${day}`);
    await page.fill("#total", total!);
    await page.fill("#merchant", "REWE");
    await page.getByRole("button", { name: "Prüfen & weiter" }).click();
    await expect(page.getByText("Beleg geprüft").first()).toBeVisible();
  }
  await expect(page).toHaveURL(/\/receipts$/);

  // 7,67 + 7,67 + 6,00 (the receipt caps the allowance)
  await page.goto(`/month/${month}`);
  await expect(page.getByText("21,34 €").first()).toBeVisible();
  await page.getByRole("button", { name: "Monat abschließen" }).click();
  await expect(page.getByText("abgeschlossen").first()).toBeVisible();

  const download = page.waitForEvent("download");
  await page.getByRole("link", { name: /Bericht herunterladen \(v1\)/ }).click();
  const file = await (await download).createReadStream();
  const head = await new Promise<string>((resolve) =>
    file.once("data", (c) => resolve(String(c).slice(0, 5))),
  );
  expect(head).toBe("%PDF-");
  expect((await download).suggestedFilename()).toBe(`Essenszuschuss-${month}-v1.pdf`);

  await page.goto(checkedReceipts[0]!);
  await expect(page.getByRole("button", { name: "Beleg löschen", exact: true })).toBeDisabled();
  await expect(page.getByText(/archivierten Monatsbericht enthalten/)).toBeVisible();

  await page.goto("/reports");
  await expect(page.getByText(/v1 ·/)).toBeVisible();

  // All four uploads remain in the import log, including the deleted receipt.
  await page.goto("/settings");
  await expect(page.getByText("Import-Protokoll")).toBeVisible();
  await expect(page.getByText("importiert")).toHaveCount(4);
});
