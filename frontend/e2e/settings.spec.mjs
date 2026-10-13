import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  expectConnected,
  expectNoAxeViolations,
  launch,
  makeSandbox,
  open,
  settingsMenu,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// The band's Settings gear (FH-149): the theme choice, the board root and
// the version line, in the same place on every view and the ticket page,
// replacing the rail's footer.
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

const band = () => page.getByRole("banner");
const gear = () => band().getByRole("button", { name: "Settings" });
const menu = () => settingsMenu(page);
const config = () =>
  readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8");

test("the gear sits between the backend status and Quit on every screen", async () => {
  for (const hash of [
    "#/all/overview",
    "#/all/workstreams",
    "#/p/flashheart/table",
    "#/p/flashheart/board",
    "#/ticket/FH-36",
  ]) {
    await open(page, hash);
    await expect(gear(), hash).toBeVisible();
    const order = await band()
      .getByRole("button")
      .evaluateAll((buttons) =>
        buttons.map(
          (button) =>
            button.getAttribute("aria-label") ?? button.textContent?.trim(),
        ),
      );
    const settings = order.indexOf("Settings");
    expect(order[settings - 1], hash).toBe(
      "Backend connected. Check connection",
    );
    expect(order[settings + 1], hash).toBe("Quit");
  }
});

test("the rail has no footer: theme, root and version are in the menu", async () => {
  await open(page, "#/p/flashheart/board");
  await expect(page.getByRole("group", { name: "Theme" })).toHaveCount(0);
  // The sandbox's root ends in /demo (the path may be resolved).
  const rootPath = /\/demo$/;
  await expect(page.getByText(rootPath)).toHaveCount(0);
  await gear().click();
  const panel = await menu();
  await expect(panel.getByRole("group", { name: "Theme" })).toBeVisible();
  // The root is cut from its start; its tooltip has the whole path.
  const path = panel.getByText(rootPath);
  await expect(path).toBeAttached();
  const whole = await path.textContent();
  await expect(path.locator("xpath=..")).toHaveAttribute("title", whole);
  await expect(panel.getByText(/· agent protocol \d+$/)).toBeVisible();
  await page.keyboard.press("Escape");
});

test("a theme chosen in the menu applies at once and survives a reload", async () => {
  await open(page, "#/ticket/FH-36");
  await gear().click();
  const theme = (await menu()).getByRole("group", { name: "Theme" });
  await theme.getByText("Dark", { exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  // It stays open to show the change.
  await expect(gear()).toHaveAttribute("aria-expanded", "true");
  await expect.poll(config).toContain("theme: dark");
  await page.reload();
  await expectConnected(page);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await gear().click();
  await expect(
    (await menu()).getByRole("radio", { name: "Dark" }),
  ).toBeChecked();
  await (await menu()).getByText("System", { exact: true }).click();
  await expect.poll(config).toContain("theme: system");
  await page.keyboard.press("Escape");
});

test("the menu opens and closes from the keyboard, back on the gear", async () => {
  await open(page, "#/p/flashheart/board");
  await gear().focus();
  await page.keyboard.press("Enter");
  await expect(gear()).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Tab");
  await expect(
    (await menu()).getByRole("radio", { name: "System" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(gear()).toHaveAttribute("aria-expanded", "false");
  await expect(gear()).toBeFocused();
  await page.keyboard.press("Space");
  await expect(gear()).toHaveAttribute("aria-expanded", "true");
  await page.keyboard.press("Escape");
  await expect(gear()).toBeFocused();
});

test("the open menu is accessible in both themes, wide and narrow", async () => {
  // The wide screens first, so no phone layout has scrolled the board.
  for (const [hash, width, height] of [
    ["#/p/flashheart/board", 1440, 900],
    ["#/ticket/FH-36", 1440, 900],
    ["#/p/flashheart/board", 390, 844],
  ]) {
    for (const theme of ["light", "dark"]) {
      await open(page, hash, { width, height, theme });
      await expect(gear()).toBeVisible();
      const board = page.locator(".board-grid");
      const scrolled = async () =>
        (await board.count()) === 0
          ? 0
          : board.evaluate((element) => element.scrollLeft);
      const before = await scrolled();
      await gear().click();
      const panel = await menu();
      await expect(panel).toBeVisible();
      // Opening the menu leaves the board where it was.
      expect(await scrolled()).toBe(before);
      // On a phone the menu stays inside the screen.
      const box = await panel.boundingBox();
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(width);
      await expectNoAxeViolations(page, `settings ${hash} ${width} ${theme}`);
      const where = hash.startsWith("#/ticket") ? "ticket" : "board";
      await shot(page, `settings-${where}-${width}-${theme}`, {
        keepFocus: true,
      });
      await page.keyboard.press("Escape");
      expect(await scrolled()).toBe(before);
    }
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});

test("the menu opens over a ticket panel covering a phone's screen", async () => {
  await open(page, "#/p/flashheart/board?t=FH-36", {
    width: 390,
    height: 844,
  });
  await expect(
    page.getByRole("complementary", { name: "Ticket FH-36" }),
  ).toBeVisible();
  await gear().click();
  const radio = (await menu()).getByText("Dark", { exact: true });
  // The theme's segment is what a click there reaches, not the panel.
  await expect(radio).toBeVisible();
  const box = await radio.boundingBox();
  const hit = await page.evaluate(
    ({ x, y }) => document.elementFromPoint(x, y)?.textContent,
    { x: box.x + box.width / 2, y: box.y + box.height / 2 },
  );
  expect(hit).toBe("Dark");
  await page.keyboard.press("Escape");
});
