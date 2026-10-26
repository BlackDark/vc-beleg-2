import { expect, test } from "@playwright/test";

// Uses the account created by the desktop flow; network service results are controlled.
test("service connection checks report progress, success and failure", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#username", "max");
  await page.fill("#password", "a-long-password");
  await page.getByRole("button", { name: "Anmelden" }).click();
  await expect(page).toHaveURL(/\/$/);
  await page.route("**/api/reader-status", (route) =>
    route.fulfill({
      json: {
        enabled: true,
        provider: "openai",
        model: "test-model",
        endpoint: "https://example.com/v1",
        effort: "low",
        fallback: false,
        timeoutSeconds: 180,
        sequential: true,
      },
    }),
  );
  let checks = 0;
  await page.route("**/api/services/reader/check", async (route) => {
    expect(route.request().method()).toBe("POST");
    checks++;
    await new Promise((resolve) => setTimeout(resolve, 150));
    await route.fulfill({
      json: { ok: checks === 1, durationMs: 12, errorCode: checks === 1 ? "" : "failed" },
    });
  });
  await page.goto("/settings?section=integrations");
  const button = page.getByRole("button", { name: "KI-Verbindung testen", exact: true });
  await expect(button).toBeVisible();
  await button.click();
  await expect(button).toBeDisabled();
  await expect(page.getByText("Verbindung erfolgreich", { exact: true })).toBeVisible();
  await expect(button).toBeEnabled();
  await button.click();
  await expect(page.getByText(/Verbindung fehlgeschlagen/)).toBeVisible();
  expect(checks).toBe(2);
});
