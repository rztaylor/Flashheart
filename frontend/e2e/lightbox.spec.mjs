import { join } from "node:path";
import { expect, test } from "@playwright/test";

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

// The lightbox (FH-146): near the whole viewport, zoom from 25% to 800% by
// keys, buttons and the wheel, drag and arrow keys to pan, Fit, and focus
// back on the thumbnail. Opened from the ticket page's Evidence and the
// card panel's Attachments over the demo board's 1200 by 700 screenshot.
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

const lightbox = (name) => page.getByRole("dialog", { name });
const level = (dialog) => dialog.getByRole("status");
const percent = async (dialog) =>
  Number.parseInt(await level(dialog).innerText(), 10);
const image = (dialog) => dialog.getByRole("img");
// offset is the image's pan from the stage's centre, in CSS pixels.
const offset = async (dialog) => {
  const transform = await image(dialog).evaluate(
    (element) => element.style.transform,
  );
  const match = /translate\((-?[\d.]+)px, (-?[\d.]+)px\)$/.exec(transform);
  return { x: Number(match?.[1]), y: Number(match?.[2]) };
};
const stageBox = async (dialog) =>
  dialog.locator("[data-lightbox-stage]").boundingBox();

// openEvidence opens the ticket page's Evidence screenshot.
async function openEvidence(options) {
  await open(page, "#/ticket/FH-8", options);
  const thumbnail = page
    .getByRole("main", { name: "Ticket FH-8" })
    .getByRole("region", { name: /^Evidence/ })
    .getByRole("button", { name: "Open Store tests passing" });
  await thumbnail.click();
  const dialog = lightbox("Store tests passing");
  await expect(image(dialog)).toBeVisible();
  await expect(level(dialog)).toHaveText(/^\d+%$/);
  return { dialog, thumbnail };
}

test("Evidence opens large, zooms by keys to 800% and 25%, and fits again", async () => {
  const { dialog, thumbnail } = await openEvidence();
  // Nearly the whole viewport: a 1rem margin each side.
  const box = await dialog.boundingBox();
  expect(box.width).toBeGreaterThanOrEqual(1440 - 34);
  expect(box.height).toBeGreaterThanOrEqual(900 - 34);
  // The 1200 by 700 screenshot fits at (or near) its natural size, far
  // larger than the old 60vh limit.
  const fitted = await image(dialog).boundingBox();
  expect(fitted.height).toBeGreaterThan(900 * 0.6);
  const fit = await level(dialog).innerText();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `lightbox fit ${theme}`);
    await shot(page, `lightbox-fit-1440-${theme}`, { keepFocus: true });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });

  // Cmd or Ctrl with plus and minus are the lightbox's, not the browser's.
  await page.evaluate(() => {
    window.lastKeyPrevented = null;
    window.addEventListener("keydown", (event) => {
      window.lastKeyPrevented = event.defaultPrevented;
    });
  });
  await page.keyboard.press("ControlOrMeta+=");
  expect(await page.evaluate(() => window.lastKeyPrevented)).toBe(true);
  expect(await percent(dialog)).toBeGreaterThan(Number.parseInt(fit, 10));
  const zoomIn = dialog.getByRole("button", { name: "Zoom in" });
  for (let step = 0; step < 12; step++) {
    await page.keyboard.press("ControlOrMeta+=");
  }
  await expect(level(dialog)).toHaveText("800%");
  await expect(zoomIn).toHaveAttribute("aria-disabled", "true");
  const zoomed = await image(dialog).boundingBox();
  expect(Math.round(zoomed.width)).toBe(1200 * 8);
  await shot(page, "lightbox-800-1440-light", { keepFocus: true });

  await page.keyboard.press("0");
  await expect(level(dialog)).toHaveText(fit);
  for (let step = 0; step < 12; step++) {
    await page.keyboard.press("ControlOrMeta+-");
  }
  expect(await page.evaluate(() => window.lastKeyPrevented)).toBe(true);
  await expect(level(dialog)).toHaveText("25%");
  await expect(
    dialog.getByRole("button", { name: "Zoom out" }),
  ).toHaveAttribute("aria-disabled", "true");
  await shot(page, "lightbox-25-1440-light", { keepFocus: true });
  await page.keyboard.press("ControlOrMeta+0");
  await expect(level(dialog)).toHaveText(fit);

  // The controls by keyboard, with the focus ring showing.
  await zoomIn.focus();
  await page.keyboard.press("Enter");
  await page.keyboard.press("Enter");
  await expect(zoomIn).toBeFocused();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `lightbox zoomed ${theme}`);
    await shot(page, `lightbox-controls-focused-1440-${theme}`, {
      keepFocus: true,
    });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await dialog.getByRole("button", { name: "Fit" }).click();
  await expect(level(dialog)).toHaveText(fit);

  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(thumbnail).toBeFocused();
});

test("the wheel zooms around the pointer, dragging and the arrows pan", async () => {
  const { dialog, thumbnail } = await openEvidence();
  const stage = await stageBox(dialog);
  const centre = {
    x: stage.x + stage.width / 2,
    y: stage.y + stage.height / 2,
  };
  const fit = await percent(dialog);

  // Up zooms in around the pointer: the image point under it stays put.
  const pointer = { x: centre.x + 200, y: centre.y + 100 };
  await page.mouse.move(pointer.x, pointer.y);
  const under = async () => {
    const box = await image(dialog).boundingBox();
    return {
      x: (pointer.x - box.x) / box.width,
      y: (pointer.y - box.y) / box.height,
    };
  };
  const before = await under();
  await page.mouse.wheel(0, -300);
  await expect.poll(() => percent(dialog)).toBeGreaterThan(fit * 1.5);
  const after = await under();
  expect(after.x).toBeCloseTo(before.x, 2);
  expect(after.y).toBeCloseTo(before.y, 2);

  // A trackpad pinch arrives as a wheel event with ctrlKey.
  const pinched = await percent(dialog);
  await page.keyboard.down("Control");
  await page.mouse.wheel(0, 20);
  await page.keyboard.up("Control");
  await expect.poll(() => percent(dialog)).toBeLessThan(pinched);

  // Drag pans by as far as the pointer moved.
  await page.keyboard.press("0");
  for (let step = 0; step < 8 && (await percent(dialog)) < 300; step++) {
    await page.keyboard.press("=");
  }
  await expect(level(dialog)).toHaveText("300%");
  const start = await offset(dialog);
  await page.mouse.move(centre.x, centre.y);
  await page.mouse.down();
  await page.mouse.move(centre.x - 100, centre.y - 60, { steps: 5 });
  await page.mouse.move(centre.x - 150, centre.y - 90, { steps: 5 });
  await page.mouse.up();
  const dragged = await offset(dialog);
  expect(dragged.x - start.x).toBeCloseTo(-150, 0);
  expect(dragged.y - start.y).toBeCloseTo(-90, 0);

  // Zoomed in, the arrows pan (Right looks right, so the image moves left).
  await page.keyboard.press("ArrowRight");
  expect((await offset(dialog)).x).toBeCloseTo(dragged.x - 80, 0);
  await page.keyboard.press("Shift+ArrowUp");
  expect((await offset(dialog)).y).toBeCloseTo(dragged.y + 320, 0);
  await shot(page, "lightbox-zoomed-panned-1440-light", { keepFocus: true });

  // Panning stops at the image's edge.
  for (let step = 0; step < 20; step++)
    await page.keyboard.press("Shift+ArrowLeft");
  const edge = await image(dialog).boundingBox();
  const shown = await stageBox(dialog);
  // The stage's 1px border.
  expect(Math.abs(edge.x - (shown.x + 1))).toBeLessThan(1);

  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(thumbnail).toBeFocused();
});

test("the card panel's Attachments open the same lightbox", async () => {
  await open(page, "#/p/flashheart/board?t=FH-8");
  const panel = page.getByRole("complementary", { name: "Ticket FH-8" });
  await panel.getByRole("tab", { name: /Attachments/ }).click();
  const thumbnail = panel.getByRole("button", {
    name: "Open Store tests passing (synthetic)",
  });
  await thumbnail.click();
  const dialog = lightbox("Store tests passing (synthetic)");
  await expect(image(dialog)).toBeVisible();
  await expect(level(dialog)).toHaveText(/^\d+%$/);
  const fit = await percent(dialog);
  const stage = await stageBox(dialog);
  await page.mouse.move(stage.x + stage.width / 2, stage.y + stage.height / 2);
  await page.mouse.wheel(0, -200);
  await expect.poll(() => percent(dialog)).toBeGreaterThan(fit);
  await dialog.getByRole("button", { name: "Zoom in" }).click();
  await page.keyboard.press("ControlOrMeta+0");
  await expect(level(dialog)).toHaveText(`${fit}%`);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(thumbnail).toBeFocused();
  // Escape closed only the lightbox, not the panel under it.
  await expect(panel).toBeVisible();
});

test("fits a phone in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    const { dialog, thumbnail } = await openEvidence({
      width: 390,
      height: 844,
      theme,
    });
    const box = await dialog.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(15);
    expect(box.x + box.width).toBeLessThanOrEqual(390 - 15);
    expect(box.height).toBeGreaterThanOrEqual(844 - 34);
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
    expect(overflow).toBe(false);
    for (const name of ["Zoom out", "Zoom in", "Fit", "Close"]) {
      await expect(dialog.getByRole("button", { name })).toBeInViewport();
    }
    await expectNoAxeViolations(page, `lightbox 390 ${theme}`);
    await shot(page, `lightbox-fit-390-${theme}`, { keepFocus: true });
    await dialog.getByRole("button", { name: "Zoom in" }).click();
    await dialog.getByRole("button", { name: "Zoom in" }).click();
    await shot(page, `lightbox-zoomed-390-${theme}`, { keepFocus: true });
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await expect(thumbnail).toBeFocused();
  }
});
