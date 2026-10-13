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

// Editing a ticket in place on its full page (FH-66): one server over a
// writable demo board. Tests run in order and each leaves the board as the
// next expects.
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

const folder = "FH-20-run-state";
const ticketFile = () =>
  join(sandbox.root, "flashheart", "tickets", folder, `${folder}.md`);
const readTicket = () => readFile(ticketFile(), "utf8");
// withoutUpdated drops the updated: stamp every save writes.
const withoutUpdated = (text) => text.replace(/^updated: .*\n/m, "");

const main = () => page.getByRole("main", { name: "Ticket FH-20" });
const pencil = (name) => main().getByRole("button", { name: `Edit ${name}` });
const save = () => main().getByRole("button", { name: "Save", exact: true });

test("the title, a property and Description save in place, each alone", async () => {
  await open(page, "#/ticket/FH-20");
  const before = await readTicket();

  // A pencil shows on hover and on keyboard focus.
  await main().getByRole("heading", { level: 2, name: "Description" }).hover();
  await expect(pencil("description")).toHaveCSS("opacity", "1");
  await pencil("title").focus();
  await shot(page, "in-place-pencil-1440-light", { keepFocus: true });

  await page.keyboard.press("Enter");
  const title = main().getByRole("textbox", { name: "Title" });
  await expect(title).toBeFocused();
  await title.fill("Derive run state from hook events");
  await page.keyboard.press("Control+Enter");
  await expect(
    main().getByRole("heading", {
      level: 1,
      name: "Derive run state from hook events",
    }),
  ).toBeVisible();
  await expect(pencil("title")).toBeFocused();
  let saved = await readTicket();
  expect(withoutUpdated(saved)).toBe(
    withoutUpdated(before).replace(
      /^# .*$/m,
      "# Derive run state from hook events",
    ),
  );

  await pencil("priority").click();
  const priority = main().getByRole("combobox", { name: "Priority" });
  await expect(priority).toBeFocused();
  await shot(page, "in-place-property-select-1440-light", { keepFocus: true });
  await priority.selectOption("low");
  await save().click();
  await expect(pencil("priority")).toBeFocused();
  const afterPriority = await readTicket();
  expect(withoutUpdated(afterPriority)).toBe(
    withoutUpdated(saved).replace(/^priority: .*$/m, "priority: low"),
  );
  saved = afterPriority;

  await pencil("description").click();
  const description = main().getByRole("textbox", { name: "Description" });
  await description.fill("Run state comes from hook events, oldest first.");
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(page, `section open ${theme}`);
    await shot(page, `in-place-section-1440-${theme}`, { keepFocus: true });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await save().click();
  await expect(pencil("description")).toBeFocused();
  await expect(
    main().getByText("Run state comes from hook events, oldest first."),
  ).toBeVisible();
  const afterDescription = await readTicket();
  const section = /## Description\n\n[^\n]*\n/;
  expect(withoutUpdated(afterDescription)).toBe(
    withoutUpdated(saved).replace(
      section,
      "## Description\n\nRun state comes from hook events, oldest first.\n",
    ),
  );

  // An untouched save writes nothing.
  await pencil("description").click();
  await save().click();
  await expect(pencil("description")).toBeFocused();
  expect(await readTicket()).toBe(afterDescription);
});

test("Escape cancels an edit and returns focus", async () => {
  await open(page, "#/ticket/FH-20");
  const before = await readTicket();
  await pencil("branch").click();
  await main().getByRole("textbox", { name: "Branch" }).fill("feature/nope");
  await page.keyboard.press("Escape");
  await expect(main().getByRole("textbox", { name: "Branch" })).toHaveCount(0);
  await expect(pencil("branch")).toBeFocused();
  expect(await readTicket()).toBe(before);
});

test("tags, depends on, Add a section and the criteria edit in place", async () => {
  await open(page, "#/ticket/FH-20");
  await pencil("tags").click();
  const tags = main().getByRole("group", { name: "Tags" });
  await tags.getByRole("textbox", { name: "Add a tag" }).fill("runs");
  await page.keyboard.press("Enter");
  await tags.getByRole("textbox", { name: "Add a tag" }).fill("hooks");
  await page.keyboard.press("Enter");
  await shot(page, "in-place-tags-1440-light", { keepFocus: true });
  await save().click();
  await expect(pencil("tags")).toBeFocused();
  expect(await readTicket()).toMatch(/^tags: \[runs, hooks\]$/m);

  await pencil("depends on").click();
  const picker = main().getByRole("group", { name: "Depends on" });
  const add = picker.getByRole("combobox", { name: "Add a ticket" });
  await add.fill("fh-18");
  await page.keyboard.press("Enter");
  await add.fill("FH-");
  await expectNoAxeViolations(page, "depends on picker");
  await shot(page, "in-place-depends-on-1440-light", { keepFocus: true });
  await add.fill("");
  await save().click();
  await expect.poll(readTicket).toMatch(/^depends-on: \[FH-19, FH-18\]$/m);

  // An empty template section offers Add in its place.
  const addContext = main().getByRole("button", { name: "Add context" });
  await addContext.scrollIntoViewIfNeeded();
  await shot(page, "in-place-add-section-1440-light");
  await addContext.click();
  await main()
    .getByRole("textbox", { name: "Context" })
    .fill("Folded by internal/runs.");
  await save().click();
  await expect
    .poll(readTicket)
    .toContain("## Context\n\nFolded by internal/runs.\n\n## Notes\n");
  await expect(pencil("context")).toBeVisible();

  await pencil("acceptance criteria").click();
  const criteria = main().getByRole("group", { name: "Acceptance criteria" });
  await criteria.getByRole("button", { name: "Add a criterion" }).click();
  await page.keyboard.type("Stale runs read as quiet");
  await criteria.getByRole("button", { name: "Move criterion 4 up" }).click();
  await criteria
    .getByRole("textbox", { name: "Criterion 1" })
    .fill("Failing test written first, red");
  await expectNoAxeViolations(page, "criteria editor");
  await shot(page, "in-place-criteria-1440-light", { keepFocus: true });
  await save().click();
  await expect
    .poll(readTicket)
    .toContain(
      [
        "## Acceptance Criteria",
        "",
        "- [x] Failing test written first, red",
        "- [ ] Behaviour implemented",
        "- [ ] Stale runs read as quiet",
        "- [ ] Docs and changelog updated",
        "",
        "## Context",
      ].join("\n"),
    );
  // Ticking stays one click.
  await main().getByRole("checkbox", { name: "Behaviour implemented" }).click();
  await expect.poll(readTicket).toContain("- [x] Behaviour implemented");
});

test("unsaved changes are kept or discarded before another item or another page", async () => {
  await open(page, "#/ticket/FH-20");
  await pencil("title").click();
  await main().getByRole("textbox", { name: "Title" }).fill("Half a title");
  await pencil("type").click();
  const ask = page.getByRole("dialog", {
    name: "Unsaved changes to the title",
  });
  await expect(ask).toBeVisible();
  await expectNoAxeViolations(page, "unsaved changes");
  await shot(page, "in-place-unsaved-1440-light", { keepFocus: true });
  await ask.getByRole("button", { name: "Keep editing" }).click();
  await expect(main().getByRole("textbox", { name: "Title" })).toBeFocused();

  // Leaving the page asks too; discarding goes on to the board.
  await main()
    .getByRole("link", { name: /Show on the .* board/ })
    .click();
  await expect(ask).toBeVisible();
  await expect(page).toHaveURL(/#\/ticket\/FH-20$/);
  await ask.getByRole("button", { name: "Discard changes" }).click();
  await expect(page).toHaveURL(/#\/p\/flashheart\/board/);
  expect(await readTicket()).not.toContain("Half a title");
});

test("a save over a newer version opens the conflict dialog; a failed save keeps the edit", async () => {
  await open(page, "#/ticket/FH-20");
  await pencil("description").click();
  await main()
    .getByRole("textbox", { name: "Description" })
    .fill("Mine: run state from events.");
  // Another writer changes the description on disk during the edit.
  const disk = await readTicket();
  await writeFile(
    ticketFile(),
    disk.replace(
      /## Description\n\n[^\n]*\n/,
      "## Description\n\nTheirs: written by an agent.\n",
    ),
  );
  await save().click();
  const conflict = page.getByRole("dialog", {
    name: "FH-20 changed while you were editing",
  });
  await expect(conflict).toBeVisible();
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).toContainText("Theirs: written by an agent.");
  await expectNoAxeViolations(page, "in-place conflict");
  await shot(page, "in-place-conflict-1440-light", { keepFocus: true });
  await conflict.getByRole("button", { name: "Overwrite with mine" }).click();
  await expect.poll(readTicket).toContain("Mine: run state from events.");

  // A save that fails keeps the edit open with the reason beside it.
  await page.route("**/api/tickets/FH-20/section", (route) =>
    route.fulfill({
      status: 500,
      contentType: "application/json",
      body: JSON.stringify({
        error: { code: "write_failed", message: "The disk is full." },
      }),
    }),
  );
  await pencil("description").click();
  await main()
    .getByRole("textbox", { name: "Description" })
    .fill("This will not save.");
  await save().click();
  await expect(main().getByRole("alert")).toContainText("The disk is full.");
  await expect(
    main().getByRole("textbox", { name: "Description" }),
  ).toHaveValue("This will not save.");
  await shot(page, "in-place-error-1440-light", { keepFocus: true });
  await page.unroute("**/api/tickets/FH-20/section");
  await page.keyboard.press("Escape");
});

test("in-place editing fits a phone and passes axe in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-20", { width: 390, height: 844, theme });
    await pencil("description").click();
    await expect(
      main().getByRole("textbox", { name: "Description" }),
    ).toBeVisible();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
    expect(overflow).toBe(false);
    await expectNoAxeViolations(page, `in-place 390 ${theme}`);
    await shot(page, `in-place-390-${theme}`, { keepFocus: true });
    await page.keyboard.press("Escape");
  }
});
