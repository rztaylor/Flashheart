import { defineConfig } from "@playwright/test";

// The README's screenshots (FH-180), kept apart from the browser suite: its
// spec is not named *.spec.mjs, so playwright.config.mjs never runs it. Run
// scripts/readme-screenshots.sh, which builds the binary and optimises the
// images.
export default defineConfig({
  testDir: ".",
  testMatch: "screenshots.shots.mjs",
  timeout: 180_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,
  workers: 1,
  reporter: "line",
  outputDir: "../../../.cache/playwright-readme-results",
  use: {
    browserName: "chromium",
    headless: true,
    trace: "retain-on-failure",
  },
});
