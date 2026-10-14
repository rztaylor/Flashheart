import { readFile, writeFile } from "node:fs/promises";
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

// The formatted section editor (FH-16, D61) on a ticket's full page, under
// the app's real CSP: one server over a writable demo board. Tests run in
// order and each leaves the board as the next expects. Every request the
// page makes and every CSP violation is recorded and checked at the end.
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
  // A CSP violation anywhere, in the document or a shadow root, is noted.
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

const ticketFile = async (id) => {
  const { readdir } = await import("node:fs/promises");
  const tickets = join(sandbox.root, "flashheart", "tickets");
  const folder = (await readdir(tickets)).find((name) =>
    name.startsWith(`${id}-`),
  );
  return join(tickets, folder, `${folder}.md`);
};
const readTicket = async (id) => readFile(await ticketFile(id), "utf8");
const withoutUpdated = (text) => text.replace(/^updated: .*\n/m, "");

const main = (id) => page.getByRole("main", { name: `Ticket ${id}` });
const pencil = (id, name) =>
  main(id).getByRole("button", { name: `Edit ${name}` });
const save = (id) =>
  main(id).getByRole("button", { name: "Save", exact: true });
const textbox = (id, name) => main(id).getByRole("textbox", { name });

// showAs picks the editor's view by its segment, as a pointer does (the
// radio itself is visually hidden).
async function showAs(id, name) {
  await main(id)
    .getByRole("group", { name: "Show the text as" })
    .locator("label", { hasText: name })
    .click();
  await expect(main(id).getByRole("radio", { name })).toBeChecked();
}

// openSection opens a section's editor and waits for it to take focus.
async function openSection(id, name, title) {
  await pencil(id, name).click();
  const editor = textbox(id, title);
  await expect(editor).toBeFocused();
  return editor;
}

const typed = [
  "Run state is **derived** from hook events.",
  "",
  "### Rules",
  "",
  "- [ ] A quiet run",
  "- [ ] A working run",
  "",
  "See FH-19 and [the spec](https://example.com/spec).",
].join("\n");

test("shows markdown formatted as it is typed, its syntax only on the line being edited", async () => {
  await open(page, "#/ticket/FH-20");
  const before = await readTicket("FH-20");
  const editor = await openSection("FH-20", "description", "Description");
  await page.keyboard.press("ControlOrMeta+a");
  // Typed as a person types it: Enter continues the task list, and Enter
  // on an empty item ends it.
  const keys = page.keyboard;
  await keys.type("Run state is **derived** from hook events.");
  await keys.press("Enter");
  await keys.press("Enter");
  await keys.type("### Rules");
  await keys.press("Enter");
  await keys.press("Enter");
  await keys.type("- [ ] A quiet run");
  await keys.press("Enter");
  await keys.type("A working run");
  await keys.press("Enter");
  await keys.press("Enter");
  await keys.press("Enter");
  await keys.type("See FH-19 and [the spec](https://example.com/spec).");

  const lines = editor.locator(".cm-line");
  // The cursor is on the last line, which shows its markdown...
  await expect(lines.last()).toHaveText(
    "See FH-19 and [the spec](https://example.com/spec).",
  );
  // ...and every other line is drawn formatted.
  await expect(lines.first()).toHaveText(
    "Run state is derived from hook events.",
  );
  await expect(lines.first().locator(".fh-strong")).toHaveText("derived");
  await expect(
    editor.getByRole("heading", { level: 3, name: "Rules" }),
  ).toBeVisible();
  await expect(
    editor.getByRole("checkbox", { name: "A quiet run" }),
  ).not.toBeChecked();
  await expect(
    editor.getByRole("checkbox", { name: "A working run" }),
  ).not.toBeChecked();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `editor ${theme}`);
    await shot(page, `editor-formatted-1440-${theme}`, { keepFocus: true });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });

  // Moving to the first line shows its syntax and hides the last line's.
  await page.keyboard.press("ControlOrMeta+Home");
  await expect(lines.first()).toHaveText(
    "Run state is **derived** from hook events.",
  );
  await expect(lines.last()).toHaveText("See FH-19 and the spec.");
  await expect(lines.last().locator(".fh-ticket")).toHaveText("FH-19");
  await shot(page, "editor-line-being-edited-1440-light", { keepFocus: true });

  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(pencil("FH-20", "description")).toBeFocused();
  // The section is saved as typed; nothing else in the file changed.
  const after = await readTicket("FH-20");
  expect(withoutUpdated(after)).toBe(
    withoutUpdated(before).replace(
      /## Description\n\n[^\n]*\n/,
      `## Description\n\n${typed}\n`,
    ),
  );
  // The view renders what the editor showed.
  await expect(
    main("FH-20").getByRole("heading", { level: 3, name: "Rules" }),
  ).toBeVisible();

  // Opening and saving it untouched writes nothing.
  await openSection("FH-20", "description", "Description");
  await save("FH-20").click();
  await expect(pencil("FH-20", "description")).toBeFocused();
  expect(await readTicket("FH-20")).toBe(after);
});

test("ticks a task from its checkbox or with Alt+Enter, changing one character each", async () => {
  await open(page, "#/ticket/FH-20");
  const before = await readTicket("FH-20");
  const editor = await openSection("FH-20", "description", "Description");
  await shot(page, "editor-task-checkboxes-1440-light", { keepFocus: true });
  await editor.getByRole("checkbox", { name: "A quiet run" }).click();
  await expect(
    editor.getByRole("checkbox", { name: "A quiet run" }),
  ).toBeChecked();
  // With the cursor on a task's line, Alt+Enter ticks it.
  await editor.getByText("A working run").click();
  await page.keyboard.press("Alt+Enter");
  await expect(
    editor.locator(".cm-line", { hasText: "A working run" }),
  ).toHaveText("- [x] A working run");
  await save("FH-20").click();
  await expect(pencil("FH-20", "description")).toBeFocused();
  expect(withoutUpdated(await readTicket("FH-20"))).toBe(
    withoutUpdated(before)
      .replace("- [ ] A quiet run", "- [x] A quiet run")
      .replace("- [ ] A working run", "- [x] A working run"),
  );
});

test("the slash menu inserts blocks by keyboard, and Escape closes only the menu", async () => {
  await open(page, "#/ticket/FH-20");
  const editor = await openSection("FH-20", "description", "Description");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Enter");
  await page.keyboard.type("/");
  const menu = main("FH-20").getByRole("listbox", { name: "Blocks" });
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("option").first()).toContainText("Heading");
  await expectNoAxeViolations(page, "slash menu");
  await shot(page, "editor-slash-menu-1440-light", { keepFocus: true });
  await page.keyboard.type("task");
  await expect(menu.getByRole("option", { selected: true })).toContainText(
    "Task list",
  );
  // CodeMirror ignores Enter for 75 ms after the list last updated (its
  // interaction delay), and the list can still update after the choice
  // reads "Task list"; give it longer than that so Enter is the menu's
  // (FH-172).
  await page.waitForTimeout(200);
  await page.keyboard.press("Enter");
  await expect(menu).toBeHidden();
  await page.keyboard.type("Follow up on quiet runs");
  await expect(editor.locator(".cm-line").last()).toHaveText(
    "- [ ] Follow up on quiet runs",
  );

  // Escape closes the menu and keeps the edit open.
  await page.keyboard.press("Enter");
  await page.keyboard.type("/");
  await expect(menu).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
  await expect(editor).toBeFocused();
  await page.keyboard.press("Backspace");
  await page.keyboard.press("Escape");
  await expect(editor).toHaveCount(0);
});

test("the selection toolbar formats by pointer and by keyboard", async () => {
  await open(page, "#/ticket/FH-20");
  const editor = await openSection("FH-20", "description", "Description");
  // Select "events" on the first line.
  await page.keyboard.press("ControlOrMeta+Home");
  await page.keyboard.press("End");
  await page.keyboard.press("ArrowLeft");
  for (let n = 0; n < "events".length; n++)
    await page.keyboard.press("Shift+ArrowLeft");
  const toolbar = main("FH-20").getByRole("toolbar", { name: "Format" });
  await expect(toolbar).toBeVisible();
  await expectNoAxeViolations(page, "selection toolbar");
  await shot(page, "editor-selection-toolbar-1440-light", { keepFocus: true });
  await toolbar.getByRole("button", { name: "Italic" }).click();
  await expect(editor).toBeFocused();
  await expect(editor.locator(".cm-line").first()).toContainText("*events*");

  // Alt+F10 reaches it; arrows move along; Escape returns to the text.
  await page.keyboard.press("Alt+F10");
  await expect(toolbar.getByRole("button", { name: "Bold" })).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("ArrowRight");
  await expect(toolbar.getByRole("button", { name: "Code" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(editor).toBeFocused();
  // Tab from a selection reaches the toolbar too, at the button last
  // used; Enter applies it.
  await page.keyboard.press("Tab");
  await expect(toolbar.getByRole("button", { name: "Code" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(editor).toBeFocused();
  await expect(editor.locator(".cm-line").first()).toContainText("*`events`*");
  await page.keyboard.press("Escape");
});

test("Markdown shows the text as written, and the choice is remembered", async () => {
  await open(page, "#/ticket/FH-20");
  const editor = await openSection("FH-20", "description", "Description");
  await showAs("FH-20", "Markdown");
  await expect(editor.locator(".cm-line").first()).toHaveText(
    "Run state is **derived** from hook events.",
  );
  await expect(editor.getByRole("checkbox")).toHaveCount(0);
  await shot(page, "editor-markdown-1440-light", { keepFocus: true });
  const config = join(sandbox.root, ".flashheart", "config.yaml");
  await expect
    .poll(() => readFile(config, "utf8").catch(() => ""))
    .toMatch(/^ {2}editor: markdown$/m);
  await page.keyboard.press("Escape");

  await page.reload();
  await open(page, "#/ticket/FH-20");
  await openSection("FH-20", "description", "Description");
  await expect(
    main("FH-20").getByRole("radio", { name: "Markdown" }),
  ).toBeChecked();
  await showAs("FH-20", "Formatted");
  await expect(
    textbox("FH-20", "Description").getByRole("checkbox").first(),
  ).toBeVisible();
  await expect
    .poll(() => readFile(config, "utf8"))
    .toMatch(/^ {2}editor: formatted$/m);
  await page.keyboard.press("Escape");
});

test("an agent's checkpoint during an edit saves without a conflict; the same section conflicts", async () => {
  await open(page, "#/ticket/FH-10");
  // The Handoff is shown but has no editor.
  await expect(
    main("FH-10").getByRole("heading", { level: 2, name: "Handoff" }),
  ).toBeVisible();
  await expect(
    main("FH-10").getByRole("button", { name: /^Edit handoff/i }),
  ).toHaveCount(0);
  await expect(
    main("FH-10").getByRole("button", { name: /^Edit notes/i }),
  ).toHaveCount(0);

  const editor = await openSection("FH-10", "description", "Description");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Mine.");
  // An agent checkpoints: the Handoff and Notes change on disk.
  const disk = await readTicket("FH-10");
  const checkpointed = disk
    .replace("- Wrote the failing test", "- Wrote the failing test and the fix")
    .replace("None.", "- 2026-10-13 · claude:demo: Checkpointed.");
  await writeFile(await ticketFile("FH-10"), checkpointed);
  await expect(main("FH-10")).toContainText(
    "Wrote the failing test and the fix",
  );
  await expect(editor).toContainText("Mine.");
  await save("FH-10").click();
  await expect(pencil("FH-10", "description")).toBeFocused();
  await expect(
    page.getByRole("dialog", { name: /changed while you were editing/ }),
  ).toHaveCount(0);
  const merged = await readTicket("FH-10");
  expect(merged).toContain("Wrote the failing test and the fix");
  expect(merged).toContain("Checkpointed.");
  expect(merged).toMatch(/## Description\n\n[^\n]* Mine\.\n/);

  // A checkpoint the page has not heard of yet when Save is pressed: the
  // save meets it (409), sees the section unchanged there and saves again
  // on it, without asking. The checkpoint is written as the first save
  // leaves the page, so the page cannot have heard of it (holding back
  // /api/changes cannot promise that: a long poll already waiting still
  // answers, FH-145).
  await openSection("FH-10", "description", "Description");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Later.");
  const section = "**/api/tickets/FH-10/section";
  const checkpointFile = await ticketFile("FH-10");
  let written = false;
  await page.route(section, async (route) => {
    if (!written) {
      written = true;
      await writeFile(
        checkpointFile,
        merged.replace("Checkpointed.", "Checkpointed again."),
      );
    }
    await route.continue();
  });
  const statuses = [];
  const onSection = (response) => {
    if (response.url().endsWith("/api/tickets/FH-10/section"))
      statuses.push(response.status());
  };
  page.on("response", onSection);
  await save("FH-10").click();
  await expect(pencil("FH-10", "description")).toBeFocused();
  expect(statuses).toEqual([409, 200]);
  page.off("response", onSection);
  await page.unroute(section);
  await expect(
    page.getByRole("dialog", { name: /changed while you were editing/ }),
  ).toHaveCount(0);
  const raced = await readTicket("FH-10");
  expect(raced).toContain("Checkpointed again.");
  expect(raced).toMatch(/## Description\n\n[^\n]* Mine\. Later\.\n/);

  // A change to the section itself is a conflict, shown, not overwritten.
  await openSection("FH-10", "description", "Description");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.type(" Again.");
  await writeFile(
    await ticketFile("FH-10"),
    raced.replace(/(## Description\n\n[^\n]*)\n/, "$1 Theirs.\n"),
  );
  await save("FH-10").click();
  const conflict = page.getByRole("dialog", {
    name: "FH-10 changed while you were editing",
  });
  await expect(conflict).toBeVisible();
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).toContainText("Theirs.");
  await expectNoAxeViolations(page, "editor conflict");
  await conflict
    .getByRole("button", { name: "Discard mine and reload" })
    .click();
  await expect(main("FH-10")).toContainText("Theirs.");
  expect(await readTicket("FH-10")).toContain("Theirs.");
  expect(await readTicket("FH-10")).not.toContain("Again.");
});

test("shows the ticket's own images and never fetches a remote one", async () => {
  await open(page, "#/ticket/FH-8");
  const before = await readTicket("FH-8");
  // Escape straight after typing cancels the edit.
  await openSection("FH-8", "description", "Description");
  await page.keyboard.type("x");
  await page.keyboard.press("Escape");
  await expect(textbox("FH-8", "Description")).toHaveCount(0);
  expect(await readTicket("FH-8")).toBe(before);

  await openSection("FH-8", "description", "Description");
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.insertText(
    [
      "![Store tests passing](files/20261004T1412-store-tests.png)",
      "",
      "![remote](https://example.com/remote.png)",
      "",
      "End.",
    ].join("\n"),
  );
  const editor = textbox("FH-8", "Description");
  const image = editor.getByRole("img", { name: "Store tests passing" });
  await expect(image).toBeVisible();
  await expect(image).toHaveAttribute(
    "src",
    /^\/api\/projects\/flashheart\/tickets\/FH-8\/files\//,
  );
  await expect(editor).toContainText("[image: remote]");
  await shot(page, "editor-image-1440-light", { keepFocus: true });
  await page.keyboard.press("Escape");
  await expect(editor).toHaveCount(0);
});

// A web page's section as Chrome copies it (FH-142), with what must not
// survive: a javascript: link, a remote image, a script, a style sheet
// and a frame, each of which would show itself if it ran or loaded.
const webPage = [
  '<meta charset="utf-8"><h1 style="font-size:2em">Install the CLI</h1>',
  '<p>Flashheart runs <strong>locally</strong>; see the <a href="https://example.com/docs/setup">setup guide</a> or <a href="javascript:window.pastedLinkRan = true">this trick</a>.</p>',
  "<h2>Steps</h2><ol><li>Download the binary<ul><li>macOS: <code>flashheart-darwin-arm64</code></li><li>Linux: <code>flashheart-linux-amd64</code></li></ul></li><li>Run <code>flashheart setup</code></li></ol>",
  '<pre><code class="language-sh">flashheart setup --yes\nflashheart serve</code></pre>',
  "<blockquote><p><em>Note:</em> nothing leaves your machine.</p></blockquote>",
  '<p><img src="https://images.example.invalid/diagram.png" alt="Architecture diagram" onerror="window.pastedImageLoaded = true" onload="window.pastedImageLoaded = true"></p>',
  "<table><tr><th>Flag</th><th>Meaning</th></tr><tr><td><code>--yes</code></td><td>Skip the question</td></tr></table>",
  '<script>window.pastedScriptRan = true</script><link rel="stylesheet" href="https://styles.example.invalid/site.css"><iframe src="https://frames.example.invalid/"></iframe>',
].join("");
const webPageText = "Install the CLI\n\nFlashheart runs locally.";
const webPageMarkdown = [
  "### Install the CLI",
  "",
  "Flashheart runs **locally**; see the [setup guide](https://example.com/docs/setup) or this trick.",
  "",
  "### Steps",
  "",
  "1. Download the binary",
  "   - macOS: `flashheart-darwin-arm64`",
  "   - Linux: `flashheart-linux-amd64`",
  "2. Run `flashheart setup`",
  "",
  "```sh",
  "flashheart setup --yes",
  "flashheart serve",
  "```",
  "",
  "> *Note:* nothing leaves your machine.",
  "",
  "[Architecture diagram](https://images.example.invalid/diagram.png)",
  "",
  "| Flag | Meaning |",
  "| --- | --- |",
  "| `--yes` | Skip the question |",
].join("\n");

// pasteInto pastes a clipboard of HTML and its plain text into the
// focused editor as the browser would; plainKey first presses Ctrl (Cmd
// on a Mac) with Shift and V as a key event alone, which the editor
// notes (a real press would paste the machine's own clipboard).
async function pasteInto(editor, { html, text, plainKey = false }) {
  const mac = process.platform === "darwin";
  await editor.evaluate(
    (content, { html, text, plainKey, mac }) => {
      if (plainKey)
        content.dispatchEvent(
          new KeyboardEvent("keydown", {
            key: "V",
            code: "KeyV",
            ctrlKey: !mac,
            metaKey: mac,
            shiftKey: true,
            bubbles: true,
            cancelable: true,
          }),
        );
      const data = new DataTransfer();
      data.setData("text/html", html);
      data.setData("text/plain", text);
      content.dispatchEvent(
        new ClipboardEvent("paste", {
          clipboardData: data,
          bubbles: true,
          cancelable: true,
        }),
      );
    },
    { html, text, plainKey, mac },
  );
}

test("pastes a web page as markdown, fetching and running nothing; Shift pastes plain text", async () => {
  await open(page, "#/ticket/FH-20");
  const editor = await openSection("FH-20", "description", "Description");
  await page.keyboard.press("ControlOrMeta+a");
  await pasteInto(editor, { html: webPage, text: webPageText });

  // Drawn formatted: headings, a list, code, a quote, the image as a link.
  await expect(
    editor.getByRole("heading", { level: 3, name: "Install the CLI" }),
  ).toBeVisible();
  await expect(editor.locator(".fh-strong").first()).toHaveText("locally");
  await expect(editor).toContainText("Architecture diagram");
  await expect(editor.locator('img[src*="example.invalid"]')).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `editor web paste ${theme}`);
    await shot(page, `editor-web-paste-1440-${theme}`, { keepFocus: true });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await showAs("FH-20", "Markdown");
  await expect(editor.locator(".cm-line").first()).toHaveText(
    "### Install the CLI",
  );
  await shot(page, "editor-web-paste-markdown-1440-light", { keepFocus: true });
  await showAs("FH-20", "Formatted");

  // Nothing in the pasted page loaded or ran.
  expect(
    await page.evaluate(() => [
      window.pastedImageLoaded,
      window.pastedScriptRan,
      window.pastedLinkRan,
    ]),
  ).toEqual([undefined, undefined, undefined]);
  expect(requests.filter((url) => url.includes("example.invalid"))).toEqual([]);

  await editor.focus();
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(pencil("FH-20", "description")).toBeFocused();
  expect(await readTicket("FH-20")).toContain(
    `## Description\n\n${webPageMarkdown}\n`,
  );

  // With Ctrl or Cmd and Shift and V, the same copy pastes as plain text.
  const again = await openSection("FH-20", "description", "Description");
  await page.keyboard.press("ControlOrMeta+a");
  await pasteInto(again, { html: webPage, text: webPageText, plainKey: true });
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(pencil("FH-20", "description")).toBeFocused();
  expect(await readTicket("FH-20")).toContain(
    `## Description\n\n${webPageText}\n`,
  );
});

test("fits a phone and passes axe in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-20", { width: 390, height: 844, theme });
    await openSection("FH-20", "description", "Description");
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
    expect(overflow).toBe(false);
    await expectNoAxeViolations(page, `editor 390 ${theme}`);
    await shot(page, `editor-390-${theme}`, { keepFocus: true });
    await page.keyboard.press("Escape");
  }
});

test("made no request beyond the board and broke no CSP rule", async () => {
  const outside = requests.filter(
    (url) => !url.startsWith(`${origin}/`) && !url.startsWith("data:"),
  );
  expect(outside).toEqual([]);
  expect(await page.evaluate(() => window.cspViolations)).toEqual([]);
  expect(
    consoleErrors.filter((text) => /Content Security Policy/i.test(text)),
  ).toEqual([]);
});
