import { defineConfig } from "@playwright/test";

// The suite drives the real binary (build/flashheart): run scripts/e2e.sh.
export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  // The stable screenshot set's baselines (visual.spec.mjs), one per name:
  // they are compared only in the pinned container, never per platform. A
  // missing baseline fails; scripts/e2e-visual.sh --update-snapshots writes
  // them.
  snapshotPathTemplate: "{testDir}/__screenshots__/{testFilePath}/{arg}{ext}",
  updateSnapshots: "none",
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
