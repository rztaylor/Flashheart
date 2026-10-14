import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { seedRuns } from "./agent-runs.mjs";
import { writeDemoBoard } from "./demo-board.mjs";
import {
  card,
  column,
  expectNoAxeViolations,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// The slim project rail (FH-150): key badges only, about 4rem at every
// width from 48rem, with what needs the user on the badge and the rest in
// its name and tooltip. The demo board's alpha project has a ticket that
// needs repair and, from the seeded runs, a session waiting on a
// permission.
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

const rail = () => page.getByRole("navigation", { name: "Projects" });
const badge = (name) =>
  rail().getByRole("button", { name: new RegExp(`^${name}`) });
const railWidth = async () => (await rail().boundingBox()).width;

test("the rail is slim at every width, with a ticket open too", async () => {
  for (const width of [1280, 1440, 1920]) {
    await open(page, "#/p/flashheart/board", { width, height: 900 });
    const slim = await railWidth();
    expect(slim, `${width}`).toBeGreaterThanOrEqual(60);
    expect(slim, `${width}`).toBeLessThanOrEqual(64);
    await card(page, "axe-core checks in both themes").click();
    await expect(
      page.getByRole("complementary", { name: "Ticket FH-31" }),
    ).toBeVisible();
    expect(await railWidth(), `${width} with the panel`).toBe(slim);
    await page.keyboard.press("Escape");
  }
  // At 1440 every column fits beside it, Done included.
  await open(page, "#/p/flashheart/board", { width: 1440, height: 900 });
  const board = await page.locator(".board-grid").boundingBox();
  const done = await column(page, "Done").boundingBox();
  expect(done.x + done.width).toBeLessThanOrEqual(board.x + board.width + 1);
});

test("badges: All projects, then each project; the current one is filled", async () => {
  await open(page, "#/p/flashheart/board");
  const names = await rail()
    .getByRole("button")
    .evaluateAll((buttons) =>
      buttons.map((button) => button.getAttribute("aria-label")),
    );
  expect(names[0]).toMatch(/^All projects \(\d+ projects\): /);
  expect(names.slice(1).map((name) => name.split(/[ :]/)[0])).toEqual(
    expect.arrayContaining(["Flashheart", "Alpha", "NG+", "beta"]),
  );
  await expect(badge("Flashheart")).toHaveAttribute("aria-current", "page");
  // No names, counts or route bars as text: only the keys.
  await expect(rail()).not.toContainText("in progress");
  await expect(rail().getByRole("img")).toHaveCount(0);
  await expect(badge("Flashheart")).toHaveText("FH");
  // Its tooltip is its name.
  const name = await badge("Flashheart").getAttribute("aria-label");
  await expect(badge("Flashheart")).toHaveAttribute("title", name);
  expect(name).toMatch(
    /^Flashheart \(flashheart\): \d+ tickets, \d+ in progress, \d+ blocked/,
  );

  await badge("Alpha").click();
  await expect(page).toHaveURL(/#\/p\/alpha\//);
  await expect(badge("Alpha")).toHaveAttribute("aria-current", "page");
  await badge("All projects").click();
  await expect(page).toHaveURL(/#\/all\//);
});

test("a badge carries its Needs you count and a repair mark", async () => {
  await open(page, "#/p/flashheart/board");
  const alpha = badge("Alpha");
  await expect(alpha).toHaveAttribute(
    "aria-label",
    /, 1 needs repair, 1 agent needs you$/,
  );
  await expect(alpha.locator("[data-mark=needs-you]")).toHaveText("1");
  await expect(alpha.locator("[data-mark=repair]")).toHaveCount(1);
  // Flashheart has neither.
  await expect(badge("Flashheart").locator("[data-mark]")).toHaveCount(0);
});

test("a phone still picks the project from the control row", async () => {
  await open(page, "#/p/flashheart/board", { width: 390, height: 844 });
  await expect(rail()).toBeHidden();
  await expect(page.getByRole("combobox", { name: "Project" })).toBeVisible();
});

test("the slim rail is accessible in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    for (const [hash, name] of [
      ["#/p/flashheart/board", "board"],
      ["#/all/overview", "overview"],
    ]) {
      await open(page, hash, { theme });
      await expect(badge("Alpha")).toBeVisible();
      await expectNoAxeViolations(page, `slim rail ${name} ${theme}`);
      // The resting rail, with no badge under the pointer.
      await page.mouse.move(720, 600);
      await shot(page, `rail-${name}-1440-${theme}`);
    }
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});
