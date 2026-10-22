import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.E2E_PORT ?? 18090);
// Local sandboxes may ship a different Chromium build than this Playwright version.
const executablePath = process.env.PW_CHROMIUM_PATH || undefined;

export default defineConfig({
  testDir: "./e2e",
  timeout: 120_000,
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    locale: "de-DE",
    timezoneId: "Europe/Berlin",
    trace: "retain-on-failure",
    launchOptions: { executablePath },
  },
  projects: [
    {
      name: "desktop",
      use: { ...devices["Desktop Chrome"], locale: "de-DE", launchOptions: { executablePath } },
      testIgnore: /mobile\.spec\.ts/,
    },
    {
      name: "mobile",
      // 360 px is the narrowest supported width.
      use: {
        ...devices["Pixel 7"],
        viewport: { width: 360, height: 780 },
        locale: "de-DE",
        launchOptions: { executablePath },
      },
      testMatch: /mobile\.spec\.ts/,
      dependencies: ["desktop"],
    },
  ],
  webServer: {
    command: `sh e2e/serve.sh ${port}`,
    url: `http://127.0.0.1:${port}/healthz`,
    reuseExistingServer: false,
    timeout: 120_000,
    stdout: "pipe",
  },
});
