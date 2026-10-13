import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { seedRuns } from "./agent-runs.mjs";
import { writeDemoBoard } from "./demo-board.mjs";
import {
  expectNoAxeViolations,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// The All projects Overview's list of projects (FH-153): every project with
// its ticket, column, blocked, needs-repair and needs-you counts, in the
// rail's order, each row leading to the project's own Overview. The demo
// board's alpha project has a ticket that needs repair and, from the
// seeded runs, a session waiting on a permission.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  await seedRuns(sandbox.home, sandbox.root);
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext();
  page = await context.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto(url);
  await page.waitForURL((current) => current.pathname === "/");
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

const list = () => page.getByRole("region", { name: "Projects" });
const rows = () => list().getByRole("link");
const row = (name) =>
  list().getByRole("link", { name: new RegExp(`^${name}`) });
// labels are each element's accessible name, up to the counts.
const names = (locator) =>
  locator.evaluateAll((elements) =>
    elements.map((element) => element.getAttribute("aria-label").split(":")[0]),
  );
// fits says the element shows all its content with no sideways scroll.
const fits = (locator) =>
  locator.evaluate((element) => element.scrollWidth <= element.clientWidth);

test("lists every project in the rail's order, with its counts", async () => {
  await open(page, "#/all/overview");
  await expect(rows().first()).toBeVisible();
  const rail = page.getByRole("navigation", { name: "Projects" });
  const badges = await names(rail.getByRole("button"));
  // The rail starts with All projects.
  expect(await names(rows())).toEqual(badges.slice(1));
  expect(await rows().count()).toBeGreaterThanOrEqual(4);

  const flashheart = row("Flashheart");
  await expect(flashheart).toHaveAttribute(
    "aria-label",
    /^Flashheart \(flashheart\): \d+ tickets \(\d+ in backlog, \d+ up next, \d+ in progress, \d+ ready to review, \d+ done\), \d+ blocked, 0 need repair, 0 agents need you$/,
  );
  await expect(flashheart).toContainText("FH");
  // The column words head the counts once, not in every row.
  const head = list().locator("[data-columns]");
  await expect(head).toBeVisible();
  for (const word of ["Tickets", "In progress", "Blocked", "Needs you"])
    await expect(head).toContainText(word);
  await expect(flashheart.getByText("In progress")).toBeHidden();
  // Its counts are the badge's: the same tickets and in progress.
  const badge = await rail
    .getByRole("button", { name: /^Flashheart/ })
    .getAttribute("aria-label");
  const [, tickets, progress] = badge.match(
    /: (\d+) tickets, (\d+) in progress/,
  );
  await expect(flashheart).toHaveAttribute(
    "aria-label",
    new RegExp(`: ${tickets} tickets \\(.* ${progress} in progress,`),
  );
});

test("Needs you and needs repair stand out on their project's row", async () => {
  await open(page, "#/all/overview");
  const alpha = row("Alpha");
  await expect(alpha).toHaveAttribute(
    "aria-label",
    /, 1 needs repair, 1 agent needs you$/,
  );
  await expect(alpha.locator("[data-mark=needs-you]")).toHaveText("1");
  await expect(alpha.locator("[data-mark=repair]")).toHaveText("1");
  await expect(row("Flashheart").locator("[data-mark]")).toHaveCount(0);
});

test("choosing a row switches to that project's Overview, which has no list", async () => {
  await open(page, "#/all/overview");
  await row("Alpha").click();
  await expect(page).toHaveURL(/#\/p\/alpha\/overview$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(/Alpha/);
  await expect(page.locator("[data-pipeline]")).toBeVisible();
  await expect(list()).toHaveCount(0);
});

test("fits a phone and the panel, and is accessible in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    for (const width of [1440, 390]) {
      const height = width === 390 ? 844 : 900;
      await open(page, "#/all/overview", { width, height, theme });
      await expect(rows().first()).toBeVisible();
      expect(await fits(list()), `${width} ${theme}`).toBe(true);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
        `${width} ${theme} page`,
      ).toBe(true);
      await expectNoAxeViolations(page, `projects list ${width} ${theme}`);
      await page.mouse.move(0, 0);
      await list().scrollIntoViewIfNeeded();
      await shot(page, `overview-projects-${width}-${theme}`);
    }
  }
  // One project's Overview, with no list.
  await open(page, "#/p/alpha/overview", { theme: "light" });
  await expect(page.locator("[data-pipeline]")).toBeVisible();
  await shot(page, "overview-project-alpha-1440-light");

  // With a ticket open beside it at 1280 the rows stack and still fit.
  await open(page, "#/all/overview", { width: 1280, height: 900 });
  await page
    .getByRole("region", { name: "In progress" })
    .getByRole("button")
    .filter({ hasNotText: /^In progress/ })
    .first()
    .click();
  await expect(page.getByRole("complementary")).toBeVisible();
  expect(await fits(list()), "1280 with the panel").toBe(true);
  await list().scrollIntoViewIfNeeded();
  await shot(page, "overview-projects-1280-panel-light");
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});
