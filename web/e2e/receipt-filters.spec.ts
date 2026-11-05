import { expect, test } from "@playwright/test";
import { receiptPng } from "./helpers";

// Runs after the desktop flow, so the account and its uploaded receipts exist.
test("reading, source and warning filters live in the URL and narrow the list", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  const queries: URLSearchParams[] = [];
  await page.route("**/api/receipts?**", (route) => {
    queries.push(new URL(route.request().url()).searchParams);
    return route.continue();
  });

  await page.goto("/receipts");
  const reading = page.getByRole("combobox", { name: "Auslesen" });
  const source = page.getByRole("combobox", { name: "Quelle" });
  const warnings = page.getByRole("checkbox", { name: "Mit Hinweisen" });
  await expect(reading).toHaveValue("");

  await reading.selectOption("off");
  await expect(page).toHaveURL(/reading=off/);
  await source.selectOption("mail");
  await expect(page).toHaveURL(/source=mail/);
  await warnings.check();
  await expect(page).toHaveURL(/warnings=true/);
  // Nothing was imported by mail: the existing empty state shows.
  await expect(page.getByText("Keine Belege", { exact: false }).first()).toBeVisible();
  await expect.poll(() => queries.at(-1)?.get("source")).toBe("mail");
  expect(queries.at(-1)?.get("reading")).toBe("off");
  expect(queries.at(-1)?.get("warnings")).toBe("true");

  await page.reload();
  await expect(reading).toHaveValue("off");
  await expect(source).toHaveValue("mail");
  await expect(warnings).toBeChecked();

  // Clearing the filters restores the list and the URL.
  await source.selectOption("");
  await warnings.uncheck();
  await reading.selectOption("");
  await expect(page).not.toHaveURL(/reading=|source=|warnings=/);
  await expect.poll(() => queries.at(-1)?.get("source")).toBeNull();
});

// Filters are orthogonal to the status segments: they stay in the URL and the
// request when switching to Ohne Datum (which maps to undated=true, not a status).
test("filters survive switching to the Ohne Datum view", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);

  await page
    .getByTestId("receipt-input")
    .first()
    .setInputFiles({ name: "filter-nodate.png", mimeType: "image/png", buffer: receiptPng(78) });
  await expect(page).toHaveURL(/\/receipts\/\d+/);

  const queries: URLSearchParams[] = [];
  await page.route("**/api/receipts?**", (route) => {
    queries.push(new URL(route.request().url()).searchParams);
    return route.continue();
  });

  await page.goto("/receipts?source=mail&warnings=true");
  await page.getByRole("button", { name: "Ohne Datum" }).click();
  await expect(page).toHaveURL(/status=undated/);
  await expect(page).toHaveURL(/source=mail/);
  await expect(page).toHaveURL(/warnings=true/);
  await expect(page.getByRole("combobox", { name: "Quelle" })).toHaveValue("mail");
  await expect(page.getByRole("checkbox", { name: "Mit Hinweisen" })).toBeChecked();
  await expect.poll(() => queries.at(-1)?.get("undated")).toBe("true");
  expect(queries.at(-1)?.get("source")).toBe("mail");
  expect(queries.at(-1)?.get("warnings")).toBe("true");
  expect(queries.at(-1)?.get("status")).toBeNull();
  expect(queries.at(-1)?.get("month")).toBeNull();

  // The undated uploads were not received by mail, so mail narrows them away; upload shows them.
  await expect(page.getByRole("link", { name: /filter-nodate\.png/ })).toHaveCount(0);
  await page.getByRole("combobox", { name: "Quelle" }).selectOption("upload");
  await expect(page.getByRole("link", { name: /filter-nodate\.png/ })).toBeVisible();
});
