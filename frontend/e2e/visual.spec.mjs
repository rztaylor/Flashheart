import { readdir, utimes } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  launch,
  makeSandbox,
  open,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// The stable screenshot set (D42): views of the demo board compared with
// committed baselines, so a visual change fails until the baselines are
// updated on purpose. Fonts and rendering differ between machines, so the
// comparison runs only in the pinned Playwright container
// (scripts/e2e-visual.sh, which CI runs too); scripts/e2e.sh skips it.
test.skip(
  !process.env.FLASHHEART_VISUAL,
  "screenshots are compared in the pinned container: scripts/e2e-visual.sh",
);
// Not serial: one changed view should not hide the others' diffs. A failure
// starts a new worker, whose beforeAll serves the board again.

// Every time the page shows is relative to these: tickets were last
// written the day before the browser's fixed now.
const written = new Date("2026-09-30T12:00:00Z");
const now = new Date("2026-10-01T09:00:00Z");

let sandbox;
let server;
let context;
let page;

// settle sets every file's modification time, which the board shows as
// when a ticket last changed.
async function settle(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) await settle(path);
    await utimes(path, written, written);
  }
}

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  await settle(demo);
  sandbox.root = demo;
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext({ locale: "en-GB", timezoneId: "UTC" });
  page = await context.newPage();
  await page.clock.setFixedTime(now);
  await page.goto(url);
  await page.waitForURL((current) => current.pathname === "/");
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

// compare shows hash at a size and theme and compares the page with its
// baseline once shown is visible, at rest: no focus ring, no animation.
async function compare(
  name,
  hash,
  shown,
  { width = 1280, theme = "light" } = {},
) {
  await open(page, hash, {
    width,
    height: width === 1280 ? 800 : 1080,
    theme,
  });
  await expect(shown(page)).toBeVisible();
  await expect(page.locator("[data-skeleton]")).toHaveCount(0);
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement)
      document.activeElement.blur();
  });
  await expect(page).toHaveScreenshot(`${name}-${width}-${theme}.png`, {
    // The board root is a new temporary folder each run.
    mask: [page.getByText(sandbox.root, { exact: true })],
  });
}

const column = (page) => page.getByRole("region", { name: "Backlog" });
const heading = (page) => page.locator("h1").first();

for (const theme of ["light", "dark"]) {
  for (const width of [1280, 1920]) {
    test(`the board, ${width} ${theme}`, async () => {
      await compare("board", "#/p/flashheart/board", column, { width, theme });
    });
  }
  test(`a ticket in the side panel, ${theme}`, async () => {
    await compare(
      "panel",
      "#/p/flashheart/board?t=FH-5",
      (page) => page.getByRole("complementary", { name: "Ticket FH-5" }),
      { theme },
    );
  });
}

test("the table", async () => {
  await compare("table", "#/p/flashheart/table", (page) =>
    page.getByRole("table"),
  );
});

test("workstreams", async () => {
  await compare("workstreams", "#/p/flashheart/workstreams", heading);
});

test("the Overview of every project", async () => {
  await compare("overview", "#/all/overview", heading);
});

test("a ticket's own page", async () => {
  await compare("ticket", "#/ticket/FH-5", heading);
});
