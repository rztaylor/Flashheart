import { readFile, stat, writeFile } from "node:fs/promises";
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

// A workstream's page (FH-139): opened from its route card, its sections
// edited in place in the formatted editor, saved one section at a time
// under the content-hash precondition. One server over a writable demo
// board; tests run in order and each leaves the board as the next expects.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;
let origin;
const requests = [];

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  origin = new URL(url).origin;
  context = await browser.newContext();
  page = await context.newPage();
  page.on("request", (request) => requests.push(request.url()));
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

const file = () =>
  join(sandbox.root, "flashheart", "workstreams", "board-core.md");
const readWorkstream = () => readFile(file(), "utf8");

const main = () => page.getByRole("main", { name: "Workstream board-core" });
const pencil = (name) => main().getByRole("button", { name: `Edit ${name}` });
const save = () => main().getByRole("button", { name: "Save", exact: true });
const textbox = (name) => main().getByRole("textbox", { name });

async function openSection(name, title) {
  await pencil(name).click();
  const editor = textbox(title);
  await expect(editor).toBeFocused();
  return editor;
}

test("opens from its route card's title and shows its sections", async () => {
  await open(page, "#/p/flashheart/workstreams");
  await page
    .getByRole("article", { name: "Board core" })
    .getByRole("link", { name: "Board core" })
    .click();
  await expect(page).toHaveURL(/#\/workstream\/flashheart\/board-core$/);
  await expect(
    main().getByRole("heading", { level: 1, name: "Board core" }),
  ).toBeVisible();
  await expect(main().getByRole("region", { name: "Goal" })).toContainText(
    "Deliver board core.",
  );
  await expect(main().getByRole("button", { name: "Add scope" })).toBeVisible();
  await expect(page).toHaveTitle("Board core · Flashheart");
  await shot(page, "workstream-page-1440-light");
  await main()
    .getByRole("link", { name: /Show in the .* workstreams/ })
    .click();
  await expect(page).toHaveURL(/#\/p\/flashheart\/workstreams$/);
});

test("saves only the edited section, and an untouched save writes nothing", async () => {
  await open(page, "#/workstream/flashheart/board-core");
  const before = await readWorkstream();
  await openSection("goal", "Goal");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Columns **first**.");
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(pencil("goal")).toBeFocused();
  await expect(page.getByText("Saved Board core.")).toBeVisible();
  const after = await readWorkstream();
  expect(after).toBe(
    before.replace(
      "Deliver board core.\n",
      "Deliver board core. Columns **first**.\n",
    ),
  );
  await expect(main().getByRole("region", { name: "Goal" })).toContainText(
    "Deliver board core. Columns first.",
  );

  // Opened and saved untouched: the file keeps its bytes and its time.
  const { mtimeMs } = await stat(file());
  await openSection("goal", "Goal");
  await save().click();
  await expect(pencil("goal")).toBeFocused();
  expect(await readWorkstream()).toBe(after);
  expect((await stat(file())).mtimeMs).toBe(mtimeMs);

  // An empty template section is added in its place.
  await main().getByRole("button", { name: "Add scope" }).click();
  await expect(textbox("Scope")).toBeFocused();
  await page.keyboard.type("Columns and cards.");
  await save().click();
  await expect(pencil("scope")).toBeFocused();
  await expect(main().getByRole("region", { name: "Scope" })).toContainText(
    "Columns and cards.",
  );
  expect(await readWorkstream()).toBe(
    `${after}\n## Scope\n\nColumns and cards.\n`,
  );
});

test("the editor's slash menu, selection toolbar and Markdown view work here", async () => {
  await open(page, "#/workstream/flashheart/board-core");
  const editor = await openSection("scope", "Scope");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Enter");
  await page.keyboard.type("/");
  const menu = main().getByRole("listbox", { name: "Blocks" });
  await expect(menu).toBeVisible();
  await page.keyboard.type("task");
  // Click the choice: CodeMirror ignores Enter for 75 ms after the list
  // last updated (its interaction delay), and the list can update after
  // the selected choice already reads "Task list", so an Enter could fall
  // through to a new line (FH-172). editor.spec covers choosing by Enter.
  await menu.getByRole("option", { name: /Task list/ }).click();
  await expect(menu).toBeHidden();
  await page.keyboard.type("Drag and drop");
  await expect(editor.locator(".cm-line").last()).toHaveText(
    "- [ ] Drag and drop",
  );
  // Select "cards" on the first line: the toolbar formats it.
  await page.keyboard.press("ControlOrMeta+Home");
  await page.keyboard.press("End");
  await page.keyboard.press("ArrowLeft");
  for (let n = 0; n < "cards".length; n++)
    await page.keyboard.press("Shift+ArrowLeft");
  const toolbar = main().getByRole("toolbar", { name: "Format" });
  await expect(toolbar).toBeVisible();
  await toolbar.getByRole("button", { name: "Bold" }).click();
  await expect(editor.locator(".cm-line").first()).toContainText("**cards**");
  await shot(page, "workstream-page-editor-1440-light", { keepFocus: true });

  // The Formatted / Markdown choice is the one the ticket page remembers.
  await main()
    .getByRole("group", { name: "Show the text as" })
    .locator("label", { hasText: "Markdown" })
    .click();
  await expect(main().getByRole("radio", { name: "Markdown" })).toBeChecked();
  const config = join(sandbox.root, ".flashheart", "config.yaml");
  await expect
    .poll(() => readFile(config, "utf8").catch(() => ""))
    .toMatch(/^ {2}editor: markdown$/m);
  await save().click();
  await expect(pencil("scope")).toBeFocused();
  expect(await readWorkstream()).toMatch(
    /## Scope\n\nColumns and \*\*cards\*\*\.\n\n- \[ \] Drag and drop\n$/,
  );
  await page.reload();
  await open(page, "#/workstream/flashheart/board-core");
  await openSection("scope", "Scope");
  await expect(main().getByRole("radio", { name: "Markdown" })).toBeChecked();
  await main()
    .getByRole("group", { name: "Show the text as" })
    .locator("label", { hasText: "Formatted" })
    .click();
  await expect
    .poll(() => readFile(config, "utf8"))
    .toMatch(/^ {2}editor: formatted$/m);
  await page.keyboard.press("Escape");
  await expect(pencil("scope")).toBeFocused();
});

test("a change elsewhere in the file saves; the same section conflicts", async () => {
  await open(page, "#/workstream/flashheart/board-core");
  await openSection("goal", "Goal");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Mine.");
  // Someone adds a note on disk: the page hears of it while editing.
  const noted = (await readWorkstream()).concat("\n## Notes\n\nTheirs.\n");
  await writeFile(file(), noted);
  await expect(main().getByRole("region", { name: "Notes" })).toContainText(
    "Theirs.",
  );
  await save().click();
  await expect(pencil("goal")).toBeFocused();
  await expect(
    page.getByRole("dialog", { name: /changed while you were editing/ }),
  ).toHaveCount(0);
  const merged = await readWorkstream();
  expect(merged).toBe(noted.replace("first**.\n", "first**. Mine.\n"));

  // A change the page has not heard of yet when Save is pressed: the
  // save meets it (409), sees the Goal unchanged there and saves again.
  // The file is written as the first save leaves the page, so the page
  // cannot have heard of it (holding back /api/changes cannot promise
  // that: a long poll already waiting still answers, FH-164).
  await openSection("goal", "Goal");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Later.");
  const section = "**/workstreams/board-core/section";
  let written = false;
  await page.route(section, async (route) => {
    if (!written) {
      written = true;
      await writeFile(file(), merged.replace("Theirs.", "Theirs again."));
    }
    await route.continue();
  });
  const statuses = [];
  const onSection = (response) => {
    if (response.url().endsWith("/workstreams/board-core/section"))
      statuses.push(response.status());
  };
  page.on("response", onSection);
  await save().click();
  await expect(pencil("goal")).toBeFocused();
  expect(statuses).toEqual([409, 200]);
  page.off("response", onSection);
  await page.unroute(section);
  const raced = await readWorkstream();
  expect(raced).toContain("Theirs again.");
  expect(raced).toContain("Mine. Later.\n");

  // A change to the Goal itself is a conflict, shown, not overwritten.
  await openSection("goal", "Goal");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Again.");
  await writeFile(file(), raced.replace("Mine. Later.", "Mine. Theirs."));
  await save().click();
  const conflict = page.getByRole("dialog", {
    name: "Board core changed while you were editing",
  });
  await expect(conflict).toBeVisible();
  await expect(conflict).toContainText("saved this workstream");
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).toContainText("Mine. Theirs.");
  await expectNoAxeViolations(page, "workstream conflict");
  await shot(page, "workstream-page-conflict-1440-light", { keepFocus: true });
  await conflict
    .getByRole("button", { name: "Discard mine and reload" })
    .click();
  await expect(main().getByRole("region", { name: "Goal" })).toContainText(
    "Mine. Theirs.",
  );
  expect(await readWorkstream()).not.toContain("Again.");
});

test("edits by keyboard alone", async () => {
  await open(page, "#/workstream/flashheart/board-core");
  await pencil("goal").focus();
  await page.keyboard.press("Enter");
  await expect(textbox("Goal")).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(pencil("goal")).toBeFocused();
  // Tab moves from the goal's pencil to the next section's.
  await page.keyboard.press("Tab");
  await expect(pencil("scope")).toBeFocused();
});

test("fits a phone and passes axe in both themes", async () => {
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
  ])
    for (const theme of ["light", "dark"]) {
      await open(page, "#/workstream/flashheart/board-core", {
        width,
        height,
        theme,
      });
      await expect(
        main().getByRole("heading", { level: 1, name: "Board core" }),
      ).toBeVisible();
      await expectNoAxeViolations(page, `workstream page ${width} ${theme}`);
      await openSection("goal", "Goal");
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth,
      );
      expect(overflow).toBe(false);
      await expectNoAxeViolations(page, `workstream editor ${width} ${theme}`);
      await shot(page, `workstream-page-editor-${width}-${theme}`, {
        keepFocus: true,
      });
      await page.keyboard.press("Escape");
    }
});

test("made no request beyond the board", async () => {
  const outside = requests.filter(
    (url) => !url.startsWith(`${origin}/`) && !url.startsWith("data:"),
  );
  expect(outside).toEqual([]);
});
