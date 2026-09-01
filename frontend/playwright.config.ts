import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI
    ? [["html", { outputFolder: "playwright-report", open: "never" }], ["github"]]
    : "list",
  use: {
    baseURL: "http://127.0.0.1:5173/GrowNerve/",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile-chromium", use: { ...devices["Pixel 7"] } },
  ],
  webServer: {
    command: "npm run dev:browser -- --host 127.0.0.1",
    url: "http://127.0.0.1:5173/GrowNerve/",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
