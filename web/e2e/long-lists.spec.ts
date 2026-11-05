import { expect, type Page, test } from "@playwright/test";
import { listMetrics, mockPendingReceipts } from "./helpers";

// Runs after the desktop flow, so the account exists.
test.beforeEach(async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);
});

const startList = (p: Page) => p.getByRole("list", { name: /^Zu prüfen \(\d+\)$/ });
const receiptsList = (p: Page) => p.getByRole("region", { name: "Belege", exact: true });
const screens = [
  { path: "/", list: startList },
  { path: "/receipts", list: receiptsList },
];

for (const { path, list: locate } of screens) {
  test(`${path}: a long receipt list scrolls inside its container`, async ({ page }) => {
    await mockPendingReceipts(page, 12);
    await page.goto(path);
    const some = await listMetrics(page, locate(page));

    await page.unroute("**/api/receipts**");
    await mockPendingReceipts(page, 91);
    await page.goto(path);
    const many = await listMetrics(page, locate(page));

    expect(many.scrollable).toBe(true);
    expect(many.focusable).toBe(true);
    expect(many.overflowY).toBe("auto");
    expect(many.height).toBe(some.height); // the page does not grow with the list
    expect(many.overflowX).toBeLessThanOrEqual(0);

    // Keyboard: focus the list, then PageDown scrolls it.
    const list = locate(page);
    await list.focus();
    await page.keyboard.press("PageDown");
    await expect.poll(() => list.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  });
}

test("/receipts: header, month switcher and filter stay in view with a long list", async ({ page }) => {
  await mockPendingReceipts(page, 91);
  await page.goto("/receipts");
  const m = await listMetrics(page, receiptsList(page));
  expect(m.scrollable).toBe(true);
  await expect(page.getByRole("heading", { name: "Belege", level: 1 })).toBeInViewport();
  await expect(page.getByRole("button", { name: "Alle" })).toBeInViewport();
  await page.screenshot({ path: `/tmp/pr31-receipts-${test.info().project.name}.png` });
});

test("/: few receipts reserve no empty space", async ({ page }) => {
  await mockPendingReceipts(page, 2);
  await page.goto("/");
  const m = await listMetrics(page, startList(page));
  expect(m.scrollable).toBe(false);
  await page.screenshot({ path: `/tmp/pr31-start-${test.info().project.name}.png` });
});
