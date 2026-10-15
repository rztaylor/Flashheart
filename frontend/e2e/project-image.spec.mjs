import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  expectConnected,
  expectNoAxeViolations,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// A project's own image on its badge (FH-151): chosen from the page
// header's badge, cropped and resized by the server, shown on the rail and
// in the header with the key kept in the name and beside the title, and
// cleared again. Tests run in order over one demo board.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;
let originalProjectFile;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  originalProjectFile = await readFile(
    join(sandbox.root, "flashheart", "project.yaml"),
    "utf8",
  );
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

const main = () => page.getByRole("main");
const rail = () => page.getByRole("navigation", { name: "Projects" });
const railBadge = (name) =>
  rail().getByRole("button", { name: new RegExp(`^${name} \\(`) });
const headerBadge = (key) =>
  main().getByRole("button", { name: `Project image: ${key}` });
const badgeImage = (locator) => locator.locator("img[data-badge-image], img");
const toast = (text) => page.getByRole("status").filter({ hasText: text });
const projectFile = () =>
  readFile(join(sandbox.root, "flashheart", "project.yaml"), "utf8");

// The stored square must fill the header badge without text padding.
async function expectImageFillsBadge(badge) {
  const image = badgeImage(badge);
  await expect(image).toBeVisible();
  const frame = await image.locator("..").boundingBox();
  const bounds = await image.boundingBox();
  expect(bounds.width).toBe(frame.width);
  expect(bounds.height).toBe(frame.height);
  expect(bounds.x).toBe(frame.x);
  expect(bounds.y).toBe(frame.y);
  expect(bounds.width).toBe(bounds.height);
}

// picture draws a width×height picture in the page's own canvas, a
// coloured ground with a white disc off centre, so a crop is visible, and
// returns it as the browser would encode it (PNG or JPEG).
async function picture(type, colour, width = 480, height = 300) {
  const dataURL = await page.evaluate(
    ({ type, colour, width, height }) => {
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const context = canvas.getContext("2d");
      context.fillStyle = colour;
      context.fillRect(0, 0, width, height);
      context.fillStyle = "#ffffff";
      context.beginPath();
      context.arc(width / 2, height / 2, Math.min(width, height) / 3, 0, 7);
      context.fill();
      context.fillStyle = "#111111";
      context.fillRect(0, 0, 40, height);
      return canvas.toDataURL(type);
    },
    { type, colour, width, height },
  );
  return Buffer.from(dataURL.split(",")[1], "base64");
}

// openMenu opens the header badge's menu and returns its panel.
async function openMenu(key) {
  const badge = headerBadge(key);
  if ((await badge.getAttribute("aria-expanded")) !== "true")
    await badge.click();
  return page.locator(`[id="${await badge.getAttribute("aria-controls")}"]`);
}

// choose picks a file through the menu's Choose or Replace button.
async function choose(panel, file) {
  const chooser = page.waitForEvent("filechooser");
  await panel
    .getByRole("button", { name: /Choose image|Replace image/ })
    .click();
  await (await chooser).setFiles(file);
}

// pngSize reads a PNG file's width and height from its header.
function pngSize(data) {
  return [data.readUInt32BE(16), data.readUInt32BE(20)];
}

test("the header badge is a button named with the key; a PNG becomes the badge", async () => {
  await open(page, "#/p/flashheart/board");
  const badge = headerBadge("FH");
  await expect(badge).toHaveAttribute("title", "Project image: FH");
  await expect(badge).toHaveText("FH");
  await expect(railBadge("Flashheart")).toHaveText("FH");
  await expect(railBadge("Flashheart")).toHaveAttribute(
    "aria-label",
    /^Flashheart \(FH, flashheart\): /,
  );

  const panel = await openMenu("FH");
  await expect(panel).toContainText("Choose a JPEG or PNG");
  await expect(panel.getByRole("button", { name: "Remove image" })).toHaveCount(
    0,
  );
  await shot(page, "project-image-menu-1440-light", { keepFocus: true });
  await expectNoAxeViolations(page, "project image menu");
  await choose(panel, {
    name: "logo.png",
    mimeType: "image/png",
    buffer: await picture("image/png", "#be185d"),
  });
  await expect(toast("Project image updated.")).toBeVisible();
  await expect(badge).toHaveAttribute("aria-expanded", "false");

  // The header and the rail show the image; the key stays in the name,
  // the tooltip and beside the title.
  await expect(badgeImage(badge)).toHaveAttribute(
    "src",
    /^\/api\/projects\/flashheart\/image\?v=[0-9a-f]{8}$/,
  );
  await expectImageFillsBadge(badge);
  await expect(badge).toHaveText("");
  await expect(main().locator("[data-project-key]")).toHaveText("FH");
  await expect(main().getByRole("heading", { level: 1 })).toHaveText(
    "Flashheart",
  );
  await expect(badgeImage(railBadge("Flashheart"))).toBeVisible();
  await expect(railBadge("Flashheart")).toHaveText("");
  await expect(railBadge("Flashheart")).toHaveAttribute(
    "title",
    /^Flashheart \(FH, flashheart\): /,
  );
  await expect(rail().getByRole("img")).toHaveCount(0);

  // On disk: a 128×128 PNG in the project folder, named by project.yaml,
  // with every other line kept.
  const stored = await readFile(join(sandbox.root, "flashheart", "image.png"));
  expect(stored.subarray(0, 8)).toEqual(
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
  );
  expect(pngSize(stored)).toEqual([128, 128]);
  expect(await projectFile()).toBe(`${originalProjectFile}image: image.png\n`);
  expect(await readdir(join(sandbox.root, "flashheart"))).not.toContainEqual(
    expect.stringMatching(/\.tmp/),
  );
});

test("replacing it with a JPEG changes the picture, and a reload keeps it", async () => {
  const badge = headerBadge("FH");
  const before = await badgeImage(badge).getAttribute("src");
  const panel = await openMenu("FH");
  await expect(panel).toContainText("its key is FH");
  await choose(panel, {
    name: "photo.jpg",
    mimeType: "image/jpeg",
    buffer: await picture("image/jpeg", "#0b6e4f", 300, 900),
  });
  await expect(toast("Project image updated.")).toBeVisible();
  await expect(badgeImage(badge)).not.toHaveAttribute("src", before);
  const after = await badgeImage(badge).getAttribute("src");
  expect(after).toMatch(/^\/api\/projects\/flashheart\/image\?v=/);
  await expect(badgeImage(railBadge("Flashheart"))).toHaveAttribute(
    "src",
    after,
  );
  // Still one stored file, still 128×128.
  const names = await readdir(join(sandbox.root, "flashheart"));
  expect(names.filter((name) => name.endsWith(".png"))).toEqual(["image.png"]);
  expect(
    pngSize(await readFile(join(sandbox.root, "flashheart", "image.png"))),
  ).toEqual([128, 128]);

  await page.reload();
  await expectConnected(page);
  await expect(badgeImage(headerBadge("FH"))).toHaveAttribute("src", after);
  await expect(badgeImage(railBadge("Flashheart"))).toHaveAttribute(
    "src",
    after,
  );
});

test("a file that is not an image is refused in words and the badge stays", async () => {
  const badge = headerBadge("FH");
  const before = await badgeImage(badge).getAttribute("src");
  const panel = await openMenu("FH");
  await choose(panel, {
    name: "notes.png",
    mimeType: "image/png",
    buffer: Buffer.from("These are words, not a picture.\n"),
  });
  await expect(panel.getByRole("alert")).toHaveText(
    "The image was not saved because it is not a JPEG or PNG image. Choose a JPEG or PNG image.",
  );
  await expect(badge).toHaveAttribute("aria-expanded", "true");
  await shot(page, "project-image-refused-1440-light", { keepFocus: true });
  await expectNoAxeViolations(page, "project image refused");
  await expect(badgeImage(badge)).toHaveAttribute("src", before);
  expect(await projectFile()).toBe(`${originalProjectFile}image: image.png\n`);

  // An SVG is named for what it is.
  await choose(panel, {
    name: "drawing.png",
    mimeType: "image/png",
    buffer: Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"></svg>'),
  });
  await expect(panel.getByRole("alert")).toHaveText(
    "The image was not saved because it is an SVG image. Choose a JPEG or PNG image.",
  );
  await page.keyboard.press("Escape");
  await expect(badge).toHaveAttribute("aria-expanded", "false");
  await expect(badge).toBeFocused();
});

test("a second project takes an image too; the rail mixes images and keys in both themes and widths", async () => {
  await open(page, "#/p/ngplus/board");
  const panel = await openMenu("NG");
  await choose(panel, {
    name: "ng.png",
    mimeType: "image/png",
    buffer: await picture("image/png", "#1d4ed8", 200, 200),
  });
  await expect(toast("Project image updated.")).toBeVisible();
  await expect(badgeImage(railBadge("NG\\+"))).toBeVisible();
  await expect(railBadge("Alpha")).toHaveText("AL");
  await expect(railBadge("beta")).toHaveText("BE");

  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/flashheart/board", { width: 1440, theme });
    await expectImageFillsBadge(headerBadge("FH"));
    await shot(page, `project-image-rail-1440-${theme}`);
    await expectNoAxeViolations(page, `rail with images ${theme}`);
    await open(page, "#/p/flashheart/board", {
      width: 390,
      height: 844,
      theme,
    });
    await expectImageFillsBadge(headerBadge("FH"));
    await expect(main().locator("[data-project-key]")).toHaveText("FH");
    await shot(page, `project-image-header-390-${theme}`);
    await expectNoAxeViolations(page, `header with image at 390 ${theme}`);
  }
  await open(page, "#/p/flashheart/board", { width: 1440, theme: "light" });
});

test("removing the image shows the key again and removes the file and the field", async () => {
  await open(page, "#/p/flashheart/board");
  const panel = await openMenu("FH");
  await panel.getByRole("button", { name: "Remove image" }).click();
  await expect(toast("Project image removed.")).toBeVisible();
  const badge = headerBadge("FH");
  await expect(badge).toHaveText("FH");
  await expect(badgeImage(badge)).toHaveCount(0);
  await expect(main().locator("[data-project-key]")).toHaveCount(0);
  await expect(railBadge("Flashheart")).toHaveText("FH");
  await expect(badgeImage(railBadge("Flashheart"))).toHaveCount(0);
  expect(await projectFile()).toBe(originalProjectFile);
  expect(await readdir(join(sandbox.root, "flashheart"))).not.toContain(
    "image.png",
  );
  // The image's URL no longer serves anything.
  const status = await page.evaluate(async () => {
    const response = await fetch("/api/projects/flashheart/image");
    return response.status;
  });
  expect([401, 404]).toContain(status);
});
