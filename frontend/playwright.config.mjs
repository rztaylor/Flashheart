import { defineConfig } from "@playwright/test";

// The suite drives the real binary (build/flashheart): run scripts/e2e.sh.
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  reporter: "line",
  outputDir: "../.cache/playwright-results",
  use: {
    browserName: "chromium",
    headless: true,
    trace: "retain-on-failure",
  },
});
