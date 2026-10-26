import { expect, test } from "@playwright/test";
import type { Receipt } from "../src/api/gen/types.gen";
import { receiptPng } from "./helpers";

// Public API boundary fixture controls timing and errors; worker behavior is
// covered with real SQLite and a controlled provider in Go app/API tests.
test("queued and processing receipts keep polling, failed readings can be retried", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  const receipt: Receipt = {
    id: 999,
    status: "pending",
    source: "upload",
    number: "",
    archived: false,
    filename: "processing.png",
    mime: "image/png",
    pages: 1,
    date: "",
    total: 0,
    merchant: "",
    note: "",
    mailFrom: "",
    extraction: "queued",
    extracted: {},
    createdAt: new Date().toISOString(),
    checkedAt: "",
    warnings: [],
  };
  let retries = 0;
  await page.route("**/api/receipts**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/receipts/999/preview")
      return route.fulfill({ contentType: "image/png", body: receiptPng(151) });
    if (path === "/api/receipts/999/extract" && route.request().method() === "POST") {
      retries++;
      receipt.extraction = "queued";
      receipt.extracted = {};
      return route.fulfill({ json: receipt });
    }
    if (path === "/api/receipts/999") return route.fulfill({ json: receipt });
    if (path === "/api/receipts") return route.fulfill({ json: [receipt] });
    return route.continue();
  });

  await page.goto("/");
  await expect(page.getByText("wartet auf Verarbeitung")).toBeVisible({ timeout: 5000 });
  receipt.extraction = "processing";
  await expect(page.getByText("wird gelesen …")).toBeVisible();
  await page.goto("/receipts");
  await expect(page.getByText(/wird gelesen …/)).toBeVisible();
  await page.goto("/receipts/999");
  await expect(page.getByText("wird gelesen …")).toBeVisible();
  receipt.extraction = "failed";
  receipt.extracted = { error: "Provider unavailable" };
  await expect(page.getByText("Provider unavailable")).toBeVisible();
  await page.getByRole("button", { name: "Erneut lesen" }).click();
  await expect(page.getByText("wartet auf Verarbeitung")).toBeVisible();
  expect(retries).toBe(1);
  receipt.extraction = "processing";
  await expect(page.getByText("wird gelesen …")).toBeVisible();
  receipt.extraction = "done";
  receipt.date = "2026-10-05";
  receipt.total = 700;
  receipt.merchant = "Shop";
  await expect(page.getByText("Werte vom Beleg gelesen. Bitte prüfen.")).toBeVisible();
  await expect(page.locator("#merchant")).toHaveValue("Shop");
  await expect(page.getByRole("button", { name: "Erneut lesen" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Prüfen & weiter" })).toBeEnabled();
});
