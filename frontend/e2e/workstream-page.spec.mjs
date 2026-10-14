import { readdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { callTool } from "./agent-runs.mjs";
import { writeDemoBoard } from "./demo-board.mjs";
import {
  expectNoAxeViolations,
  filterButton,
  filterMenu,
  launch,
  makeSandbox,
  open,
  settingsMenu,
  shot,
  stopIfRunning,
  viewOption,
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
  // Enter right as the list updates is not the menu's (CodeMirror's
  // interaction delay), so wait for the filtered choice, as editor.spec does.
  await expect(menu.getByRole("option", { selected: true })).toContainText(
    "Task list",
  );
  await page.keyboard.press("Enter");
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

// ticketFiles are the project's ticket files, live and archived, by path.
async function ticketFiles(project) {
  const out = new Map();
  for (const dir of ["tickets", join(".archive", "tickets")]) {
    const base = join(sandbox.root, project, dir);
    const folders = await readdir(base).catch(() => []);
    for (const folder of folders) {
      const path = join(base, folder, `${folder}.md`);
      const text = await readFile(path, "utf8").catch(() => undefined);
      if (text !== undefined) out.set(path, text);
    }
  }
  return out;
}

test("renames the workstream from its name: references follow and the page moves (FH-167, FH-170)", async () => {
  const dir = join(sandbox.root, "flashheart", "workstreams");
  const config = join(sandbox.root, ".flashheart", "config.yaml");
  // A saved Board filter that hides the line: the rename keeps it
  // (FH-170; the expanded choice is in the store's and api's tests).
  // Written before a reload, so the page loads it as its preferences.
  await writeFile(
    config,
    [
      "version: 2",
      "ui:",
      "  theme: system",
      "  density: normal",
      "  colour_by: type",
      "  hidden_columns: []",
      "  editor: formatted",
      "  scopes:",
      "    flashheart:",
      "      view: board",
      "      workstream: {include: [], exclude: [board-editing]}",
      "",
    ].join("\n"),
  );
  await page.reload();
  await open(page, "#/p/flashheart/workstreams");
  await page
    .getByRole("article", { name: "Board editing" })
    .getByRole("link", { name: "Board editing" })
    .click();
  await expect(page).toHaveURL(/#\/workstream\/flashheart\/board-editing$/);
  const before = await ticketFiles("flashheart");
  const members = [...before].filter(([, text]) =>
    /^workstream: board-editing$/m.test(text),
  );
  expect(members.length).toBeGreaterThan(0);

  const page1 = page.getByRole("main", { name: "Workstream board-editing" });
  await page1.getByRole("button", { name: "Edit name" }).click();
  const name = page1.getByRole("textbox", { name: "Name" });
  await expect(name).toBeFocused();
  await expect(name).toHaveValue("Board editing");
  await shot(page, "workstream-name-1440-light", { keepFocus: true });

  // A name another workstream has (FH-163) is named at the field without
  // being sent; one whose slug another has is the server's refusal. Each
  // keeps the field open with what was typed, and writes nothing.
  const files = await readdir(dir);
  const patches = [];
  const onPatch = (request) => {
    if (request.method() === "PATCH") patches.push(request.url());
  };
  page.on("request", onPatch);
  await name.fill("board  CORE");
  await page.keyboard.press("ControlOrMeta+Enter");
  const alert = page1.getByRole("alert");
  await expect(alert).toHaveText(
    "Flashheart already has a workstream called Board core. Choose another name.",
  );
  expect(patches).toEqual([]);
  page.off("request", onPatch);
  await name.fill("Board core!");
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(alert).toContainText("already has the short name board-core");
  await expect(alert).toContainText("choose another name");
  await expect(name).toHaveValue("Board core!");
  await expect(name).toBeVisible();
  expect(await readdir(dir)).toEqual(files);
  await expectNoAxeViolations(page, "workstream name taken");
  await shot(page, "workstream-name-taken-1440-light", { keepFocus: true });

  // A free name: the slug follows it, with every reference.
  await name.fill("Board editing in place");
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(page).toHaveURL(
    /#\/workstream\/flashheart\/board-editing-in-place$/,
  );
  const page2 = page.getByRole("main", {
    name: "Workstream board-editing-in-place",
  });
  await expect(
    page2.getByRole("heading", { level: 1, name: "Board editing in place" }),
  ).toBeVisible();
  await expect(
    page.getByText("Renamed Board editing to Board editing in place."),
  ).toBeVisible();
  await expect(page).toHaveTitle("Board editing in place · Flashheart");
  await expect(page2.getByText(/could not be loaded/)).toHaveCount(0);
  const renamed = await readFile(
    join(dir, "board-editing-in-place.md"),
    "utf8",
  );
  expect(renamed).toMatch(/^slug: board-editing-in-place$/m);
  expect(renamed).toContain("\n# Board editing in place\n");
  expect((await readdir(dir)).includes("board-editing.md")).toBe(false);
  expect(await readFile(join(dir, "agent-runs.md"), "utf8")).toMatch(
    /^depends-on-workstreams: \[board-editing-in-place\]$/m,
  );
  const after = await ticketFiles("flashheart");
  for (const [path] of members)
    expect(after.get(path)).toMatch(/^workstream: board-editing-in-place$/m);
  for (const text of after.values())
    expect(text).not.toMatch(/\bboard-editing\b(?!-in-place)/);
  // The saved preferences name the new slug, and the page's own copy
  // does too: its next save does not put the old one back.
  const hidesNew = /exclude: \[board-editing-in-place\]/;
  await expect.poll(() => readFile(config, "utf8")).toMatch(hidesNew);
  await page2.getByRole("button", { name: "Edit goal" }).click();
  await page2
    .getByRole("group", { name: "Show the text as" })
    .locator("label", { hasText: "Markdown" })
    .click();
  await expect
    .poll(() => readFile(config, "utf8"))
    .toMatch(/^ {2}editor: markdown$/m);
  expect(await readFile(config, "utf8")).toMatch(hidesNew);
  await page2
    .getByRole("group", { name: "Show the text as" })
    .locator("label", { hasText: "Formatted" })
    .click();
  await page.keyboard.press("Escape");

  // Back leaves the page for where it came from, not the old address;
  // Forward comes back to the renamed one.
  await page.goBack();
  await expect(page).toHaveURL(/#\/p\/flashheart\/workstreams$/);
  await expect(
    page.getByRole("article", { name: "Board editing in place" }),
  ).toBeVisible();
  await page.goForward();
  await expect(
    page2.getByRole("heading", { level: 1, name: "Board editing in place" }),
  ).toBeVisible();

  // The name edit in both themes and on a phone.
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
  ])
    for (const theme of ["light", "dark"]) {
      await open(page, "#/workstream/flashheart/board-editing-in-place", {
        width,
        height,
        theme,
      });
      await page2.getByRole("button", { name: "Edit name" }).click();
      await expect(page2.getByRole("textbox", { name: "Name" })).toBeFocused();
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth,
      );
      expect(overflow).toBe(false);
      await expectNoAxeViolations(page, `workstream name ${width} ${theme}`);
      await shot(page, `workstream-name-${width}-${theme}`, {
        keepFocus: true,
      });
      if (width === 390 && theme === "dark") {
        await page2.getByRole("textbox", { name: "Name" }).fill("Board core!");
        await page2.getByRole("button", { name: "Save", exact: true }).click();
        await expect(page2.getByRole("alert")).toContainText(
          "choose another name",
        );
        await shot(page, "workstream-name-taken-390-dark", { keepFocus: true });
      }
      await page.keyboard.press("Escape");
    }
  await open(page, "#/workstream/flashheart/board-editing-in-place");
});

// renameByAgent renames a workstream of the demo's flashheart project
// through the real `flashheart mcp`, as an agent's update_workstream does.
const renameByAgent = (workstream, to) =>
  callTool(sandbox.root, sandbox.root, "update_workstream", {
    project: "flashheart",
    workstream,
    rename_to: to,
  });

test("an agent's rename while the board is open keeps the saved filter on the new slug (FH-173)", async () => {
  const config = join(sandbox.root, ".flashheart", "config.yaml");
  const read = () => readFile(config, "utf8");
  // A saved Board filter that hides Foundation, loaded by the open board.
  await writeFile(
    config,
    [
      "version: 2",
      "ui:",
      "  theme: system",
      "  density: normal",
      "  colour_by: type",
      "  hidden_columns: []",
      "  editor: formatted",
      "  scopes:",
      "    flashheart:",
      "      view: board",
      "      workstream: {include: [], exclude: [foundation]}",
      "",
    ].join("\n"),
  );
  await page.reload();
  await open(page, "#/p/flashheart/board");
  await expect(filterButton(page, "Workstream")).toHaveAccessibleName(
    /^Workstream: 1/,
  );

  // A change in the tab just before the rename, then the agent's rename.
  await viewOption(page, "Colour by", "Priority");
  await renameByAgent("foundation", "groundwork");
  const hidesNew = /exclude: \[groundwork\]/;
  await expect.poll(read).toMatch(hidesNew);
  await expect.poll(read).toContain("colour_by: priority");

  // The tab's next saves keep the new slug: a theme, then a filter of
  // this project, whose open filters followed the rename.
  const theme = (await settingsMenu(page)).getByRole("group", {
    name: "Theme",
  });
  await theme.getByText("Dark", { exact: true }).click();
  await expect.poll(read).toContain("theme: dark");
  let saved = await read();
  expect(saved).toMatch(hidesNew);
  expect(saved).toContain("colour_by: priority");
  await theme.getByText("System", { exact: true }).click();
  await expect.poll(read).toContain("theme: system");
  await page.keyboard.press("Escape");
  const types = await filterMenu(page, "Type");
  await types.getByRole("button", { name: "bug", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect.poll(read).toMatch(/type:\n\s+include: \[bug\]/);
  saved = await read();
  expect(saved).toMatch(hidesNew);
  expect(saved).not.toMatch(/\bfoundation\b/);
  await page.getByRole("button", { name: "Clear filters" }).click();
  await viewOption(page, "Colour by", "Type");
  await expect.poll(read).toContain("colour_by: type");
});

test("a workstream page follows a rename made elsewhere, and says when its workstream is gone (FH-174)", async () => {
  await open(page, "#/workstream/flashheart/groundwork");
  const before = page.getByRole("main", { name: "Workstream groundwork" });
  await expect(
    before.getByRole("heading", { level: 1, name: "Foundation" }),
  ).toBeVisible();

  // An agent renames it: the page follows to the new address.
  await renameByAgent("groundwork", "bedrock");
  await expect(page).toHaveURL(/#\/workstream\/flashheart\/bedrock$/);
  const after = page.getByRole("main", { name: "Workstream bedrock" });
  await expect(
    after.getByRole("heading", { level: 1, name: "Foundation" }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "Foundation was renamed elsewhere: its short name is now bedrock.",
    ),
  ).toBeVisible();
  await expect(page.getByText(/could not be loaded/)).toHaveCount(0);
  await shot(page, "workstream-followed-1440-light");
  await page.getByRole("button", { name: "Dismiss" }).click();

  // Removed while open: the page says so in words, with the way back.
  await rm(join(sandbox.root, "flashheart", "workstreams", "bedrock.md"));
  await expect(
    after.getByRole("heading", {
      level: 1,
      name: "Foundation is no longer here",
    }),
  ).toBeVisible();
  await expect(
    after.getByText(
      "Flashheart has no workstream bedrock now: it was renamed or removed.",
    ),
  ).toBeVisible();
  await expect(page.getByText(/could not be loaded/)).toHaveCount(0);
  for (const [width, height] of [
    [1440, 900],
    [390, 844],
  ])
    for (const theme of ["light", "dark"]) {
      await open(page, "#/workstream/flashheart/bedrock", {
        width,
        height,
        theme,
      });
      await expect(
        after.getByRole("heading", { name: "Foundation is no longer here" }),
      ).toBeVisible();
      const overflow = await page.evaluate(
        () => document.documentElement.scrollWidth > window.innerWidth,
      );
      expect(overflow).toBe(false);
      await expectNoAxeViolations(page, `workstream gone ${width} ${theme}`);
      await shot(page, `workstream-gone-${width}-${theme}`);
    }
  await open(page, "#/workstream/flashheart/bedrock");
  await after
    .getByRole("link", { name: "Show the Flashheart workstreams" })
    .click();
  await expect(page).toHaveURL(/#\/p\/flashheart\/workstreams$/);
});

test("made no request beyond the board", async () => {
  const outside = requests.filter(
    (url) => !url.startsWith(`${origin}/`) && !url.startsWith("data:"),
  );
  expect(outside).toEqual([]);
});
