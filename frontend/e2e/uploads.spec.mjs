import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { syntheticScreenshot, writeDemoBoard } from "./demo-board.mjs";
import {
  expectNoAxeViolations,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// Pasting and dropping files into a ticket (FH-18), under the app's real
// CSP: one server over a writable demo board. Tests run in order and each
// leaves the board as the next expects. Every request the page makes and
// every CSP violation is recorded and checked at the end.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;
let origin;
const requests = [];
const consoleErrors = [];

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  origin = new URL(url).origin;
  context = await browser.newContext();
  await context.addInitScript(() => {
    window.cspViolations = [];
    document.addEventListener("securitypolicyviolation", (event) =>
      window.cspViolations.push(
        `${event.violatedDirective} ${event.blockedURI}`,
      ),
    );
  });
  page = await context.newPage();
  page.on("request", (request) => requests.push(request.url()));
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });
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

const ticketFolder = async (id) => {
  const tickets = join(sandbox.root, "flashheart", "tickets");
  const folder = (await readdir(tickets)).find((name) =>
    name.startsWith(`${id}-`),
  );
  return join(tickets, folder);
};
const readTicket = async (id) => {
  const folder = await ticketFolder(id);
  return readFile(join(folder, `${folder.split("/").pop()}.md`), "utf8");
};
const storedFiles = async (id) =>
  readdir(join(await ticketFolder(id), "files")).catch(() => []);

const main = (id) => page.getByRole("main", { name: `Ticket ${id}` });
const textbox = (id, name) => main(id).getByRole("textbox", { name });
// section is one of a ticket page's sections ("description", "files").
const section = (id, name) => main(id).locator(`[data-section="${name}"]`);

// openDescription opens a ticket's Description in the editor, with the
// cursor on a new line at its end.
async function openDescription(id) {
  await main(id).getByRole("button", { name: "Edit description" }).click();
  const editor = textbox(id, "Description");
  await expect(editor).toBeFocused();
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  return editor;
}

// give pastes or drops files on target, made in the page as the browser
// makes them: PNGs drawn on a canvas, other files from their text or as
// zero bytes of a size. events is "paste", "drop", or drag events
// ("dragenter,dragover") to show a drop target without dropping.
async function give(target, events, specs) {
  await target.evaluate(
    async (element, { events, specs }) => {
      const make = async (spec) => {
        if (spec.png) {
          const canvas = document.createElement("canvas");
          canvas.width = 480;
          canvas.height = 270;
          const context = canvas.getContext("2d");
          context.fillStyle = spec.png;
          context.fillRect(0, 0, 480, 270);
          context.fillStyle = "#ffffff";
          context.font = "bold 32px sans-serif";
          context.fillText(spec.label ?? spec.name, 28, 148);
          const blob = await new Promise((done) =>
            canvas.toBlob(done, "image/png"),
          );
          return new File([blob], spec.name, { type: "image/png" });
        }
        if (spec.size)
          return new File([new Uint8Array(spec.size)], spec.name, {
            type: spec.type ?? "",
          });
        return new File([spec.text], spec.name, { type: spec.type ?? "" });
      };
      const data = new DataTransfer();
      for (const spec of specs) data.items.add(await make(spec));
      // A drag happens over the target's last line, as a pointer's would.
      const box = element.getBoundingClientRect();
      const init = {
        bubbles: true,
        cancelable: true,
        composed: true,
        clientX: box.left + 8,
        clientY: box.bottom - 8,
      };
      for (const type of events.split(",")) {
        element.dispatchEvent(
          type === "paste"
            ? new ClipboardEvent(type, { ...init, clipboardData: data })
            : new DragEvent(type, { ...init, dataTransfer: data }),
        );
      }
    },
    { events, specs },
  );
}

// holdUploads keeps the ticket's uploads waiting until release is called;
// later ones then go straight through.
async function holdUploads(id) {
  let release;
  const held = new Promise((done) => {
    release = done;
  });
  await page.route(`**/api/tickets/${id}/files?*`, async (route) => {
    await held;
    await route.continue();
  });
  return release;
}

const loaded = (image) =>
  image.evaluate((element) => element.complete && element.naturalWidth > 0);

const screenshotRef = /!\[screenshot\]\(files\/\d{8}T\d{4}-screenshot\.png\)/;

test("a pasted screenshot uploads, shows its placeholder, then shows as an image in the editor and the view", async () => {
  await open(page, "#/ticket/FH-20");
  const before = await readTicket("FH-20");
  const editor = await openDescription("FH-20");
  const release = await holdUploads("FH-20");
  await give(editor, "paste", [
    { name: "image.png", png: "#b0004e", label: "Pasted screenshot" },
  ]);
  const placeholder = editor.locator(".fh-upload");
  await expect(placeholder).toHaveText("Uploading screenshot.png…");
  await expect(section("FH-20", "description").getByRole("status")).toHaveText(
    "Uploading 1 file…",
  );
  await shot(page, "upload-placeholder-1440-light", { keepFocus: true });
  // Nothing is saved while it uploads: the upload never writes the ticket.
  expect(await readTicket("FH-20")).toBe(before);

  release();
  await expect(placeholder).toHaveCount(0);
  await expect(section("FH-20", "description").getByRole("status")).toHaveText(
    "Attached screenshot.png.",
  );
  await expect(editor).toContainText(screenshotRef);
  // Off the line being edited, the reference shows as the image.
  await page.keyboard.press("ArrowUp");
  const image = editor.getByRole("img", { name: "screenshot" });
  await expect(image).toBeVisible();
  await expect.poll(() => loaded(image)).toBe(true);
  await shot(page, "upload-pasted-image-1440-light", { keepFocus: true });
  expect(await readTicket("FH-20")).toBe(before);
  expect(
    (await storedFiles("FH-20")).some((name) =>
      name.endsWith("-screenshot.png"),
    ),
  ).toBe(true);

  // The reference is saved with the section, as any edit is.
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(editor).toHaveCount(0);
  expect(await readTicket("FH-20")).toMatch(screenshotRef);
  await page.reload();
  await open(page, "#/ticket/FH-20");
  const shown = main("FH-20").getByRole("img", { name: "screenshot" });
  await expect(shown).toBeVisible();
  await expect.poll(() => loaded(shown)).toBe(true);
});

test("files dropped together attach in order: images as images, others as links", async () => {
  await open(page, "#/ticket/FH-20");
  const editor = await openDescription("FH-20");
  await give(editor, "drop", [
    { name: "spec.pdf", text: "%PDF-1.4\n%%EOF\n", type: "application/pdf" },
    { name: "before.png", png: "#3a3a3a", label: "Before" },
    { name: "after.png", png: "#2f7d32", label: "After" },
  ]);
  await expect(section("FH-20", "description").getByRole("status")).toHaveText(
    "Attached 3 files.",
  );
  await expect(editor.locator(".fh-upload")).toHaveCount(0);
  await page.keyboard.press("ControlOrMeta+Home");
  await expect(editor.getByRole("img", { name: "before" })).toBeVisible();
  await expect(editor.getByRole("img", { name: "after" })).toBeVisible();
  await editor.getByRole("img", { name: "before" }).scrollIntoViewIfNeeded();
  await shot(page, "upload-dropped-files-1440-light", { keepFocus: true });
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(editor).toHaveCount(0);
  const text = await readTicket("FH-20");
  expect(text).toMatch(
    /\[spec\.pdf\]\(files\/\d{8}T\d{4}-spec\.pdf\)\n!\[before\]\(files\/\d{8}T\d{4}-before\.png\)\n!\[after\]\(files\/\d{8}T\d{4}-after\.png\)/,
  );
  const link = main("FH-20").getByRole("link", { name: "spec.pdf" });
  await expect(link).toHaveAttribute(
    "href",
    /^\/api\/projects\/flashheart\/tickets\/FH-20\/files\/\d{8}T\d{4}-spec\.pdf$/,
  );
});

test("an SVG named .png and a file over 20 MB are refused in words and leave nothing behind", async () => {
  await open(page, "#/ticket/FH-20");
  const before = await storedFiles("FH-20");
  const editor = await openDescription("FH-20");
  await give(editor, "drop", [
    {
      name: "drawing.png",
      type: "image/png",
      text: '<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>',
    },
    { name: "huge.png", type: "image/png", size: 21 * 1024 * 1024 },
  ]);
  const alert = section("FH-20", "description").getByRole("alert");
  await expect(alert).toContainText(
    "drawing.png was not attached because it is an SVG image.",
  );
  await expect(alert).toContainText(
    "huge.png is larger than 20 MB, the limit for an attachment.",
  );
  await expect(editor.locator(".fh-upload")).toHaveCount(0);
  await expect(section("FH-20", "description").getByRole("status")).toHaveText(
    "",
  );
  await alert.scrollIntoViewIfNeeded();
  await shot(page, "upload-refused-1440-light", { keepFocus: true });
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `editor with a refusal ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  expect(await storedFiles("FH-20")).toEqual(before);
  await section("FH-20", "description")
    .getByRole("button", { name: "Dismiss" })
    .click();
  await expect(alert).toHaveCount(0);
  // Dismissing takes the button away, so focus goes back to the text.
  await expect(editor).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(editor).toHaveCount(0);
});

test("files dropped on the panel's Attachments tab are attached without touching the ticket", async () => {
  await open(page, "#/p/flashheart/board?t=FH-10");
  const panel = page.getByRole("complementary", { name: "Ticket FH-10" });
  const tab = panel.getByRole("tab", { name: /^Attachments/ });
  await tab.click();
  await expect(tab).toHaveAttribute("aria-selected", "true");
  const before = await readTicket("FH-10");
  const slot = panel.locator("[data-drop-slot]");
  await expect(slot).toContainText("Drop files here to attach them");
  await give(slot, "dragenter,dragover", [{ name: "build.log", text: "ok\n" }]);
  await expect(slot.locator("..")).toHaveAttribute("data-dropping", "true");
  await shot(page, "upload-attachments-drop-target-1440-light");
  await give(slot, "drop", [
    { name: "build.log", text: "ok\nall tests passed\n", type: "text/plain" },
    { name: "board.png", png: "#b0004e", label: "Board" },
  ]);
  await expect(slot.locator("..")).not.toHaveAttribute("data-dropping");
  await expect(panel.getByRole("tabpanel").getByRole("status")).toHaveText(
    "Attached 2 files.",
  );
  await expect(tab).toHaveText("Attachments 2");
  await expect(panel.getByRole("link", { name: /build\.log/ })).toBeVisible();
  await expect(
    panel.getByRole("button", { name: "Open board.png" }),
  ).toBeVisible();
  await shot(page, "upload-attachments-dropped-1440-light");
  expect(await readTicket("FH-10")).toBe(before);
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `Attachments tab ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});

test("Attach files works from the keyboard on the page's Files and in the editor", async () => {
  await open(page, "#/ticket/FH-10");
  const files = section("FH-10", "files");
  const button = files.getByRole("button", { name: "Attach files" });
  await button.focus();
  const chooser = page.waitForEvent("filechooser");
  await page.keyboard.press("Enter");
  await (await chooser).setFiles({
    name: "Report.png",
    mimeType: "image/png",
    buffer: syntheticScreenshot(480, 280),
  });
  await expect(files.getByRole("status")).toHaveText("Attached Report.png.");
  await expect(
    files.getByRole("button", { name: "Open Report.png" }),
  ).toBeVisible();
  await expect(button).toBeFocused();
  await shot(page, "upload-files-attach-button-1440-light", {
    keepFocus: true,
  });

  // In the editor, Attach files is the next stop after the text, and the
  // reference goes where the cursor was.
  const editor = await openDescription("FH-10");
  await page.keyboard.type("Log: ");
  await page.keyboard.press("Tab");
  const attach = section("FH-10", "description").getByRole("button", {
    name: "Attach files",
  });
  await expect(attach).toBeFocused();
  const picker = page.waitForEvent("filechooser");
  await page.keyboard.press("Enter");
  await (await picker).setFiles({
    name: "run.log",
    mimeType: "text/plain",
    buffer: Buffer.from("step 1 ok\n"),
  });
  // Focus stays on the button, so the line shows formatted: the link's
  // words, its markdown hidden.
  await expect(editor.locator(".fh-upload")).toHaveCount(0);
  await expect(editor).toContainText("Log: run.log");
  await expect(attach).toBeFocused();
  await shot(page, "upload-editor-attach-button-1440-light", {
    keepFocus: true,
  });
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `editor attach row ${theme}`);
    await shot(page, `upload-editor-1440-${theme}`, { keepFocus: true });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(editor).toHaveCount(0);
  expect(await readTicket("FH-10")).toMatch(
    /Log: \[run\.log\]\(files\/\d{8}T\d{4}-run\.log\)/,
  );
});

test("fits a phone and passes axe in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-10", { width: 390, height: 844, theme });
    const files = section("FH-10", "files");
    await files.scrollIntoViewIfNeeded();
    await expect(files.locator("[data-drop-slot]")).toBeVisible();
    const editor = await openDescription("FH-10");
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
    expect(overflow).toBe(false);
    await expectNoAxeViolations(page, `uploads 390 ${theme}`);
    await shot(page, `upload-editor-390-${theme}`, { keepFocus: true });
    await page.keyboard.press("Escape");
    await expect(editor).toHaveCount(0);
    await files.scrollIntoViewIfNeeded();
    await shot(page, `upload-files-390-${theme}`);
  }
});

test("made no request beyond the board and broke no CSP rule", async () => {
  const outside = requests.filter(
    (url) =>
      !url.startsWith(`${origin}/`) &&
      !url.startsWith("data:") &&
      !url.startsWith("blob:"),
  );
  expect(outside).toEqual([]);
  expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
  expect(
    consoleErrors.filter((text) => /Content Security Policy/i.test(text)),
  ).toEqual([]);
});
