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

// Comments on a ticket's full page and Activity's filter (FH-125): one
// server over a writable demo board. Tests run in order and each leaves the
// board as the next expects.
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
const activityFile = () =>
  join(sandbox.root, "flashheart", "tickets", folder, "activity.md");
const readActivity = () => readFile(activityFile(), "utf8").catch(() => "");

const main = () => page.getByRole("main", { name: "Ticket FH-20" });
const activity = () => main().getByRole("region", { name: "Activity" });
const box = () => activity().getByRole("textbox", { name: "Add a comment" });
const entries = () => activity().getByRole("listitem");
// show picks Activity's filter by its segment, as a pointer does (the
// radio itself is visually hidden).
const show = async (name) => {
  await activity()
    .getByRole("group", { name: "Show in Activity" })
    .locator("label", { hasText: name })
    .click();
  await expect(activity().getByRole("radio", { name })).toBeChecked();
};

test("a comment quoting a criterion posts with Ctrl+Enter and stays after a reload", async () => {
  await open(page, "#/ticket/FH-20");
  // A property edit gives the ticket some history first.
  await main().getByRole("button", { name: "Edit priority" }).click();
  await main().getByRole("combobox", { name: "Priority" }).selectOption("low");
  await main().getByRole("button", { name: "Save", exact: true }).click();
  await expect(
    activity().getByText("edited the priority", { exact: false }),
  ).toBeVisible();

  await box().scrollIntoViewIfNeeded();
  // An empty comment cannot be posted.
  await expect(
    activity().getByRole("button", { name: "Comment", exact: true }),
  ).toBeDisabled();
  await activity()
    .getByRole("combobox", { name: "Quote" })
    .selectOption({ label: "2. Behaviour implemented" });
  await expect(
    activity().getByRole("figure", { name: "Quoting Acceptance criteria 2" }),
  ).toContainText("Behaviour implemented");
  await box().fill("Cover the quiet state too.");
  await expectNoAxeViolations(page, "comment box with a quote");
  await shot(page, "comments-box-quote-1440-light", { keepFocus: true });
  await page.keyboard.press("Control+Enter");
  await expect(box()).toHaveValue("");
  await expect(entries().last()).toContainText("Cover the quiet state too.");
  await expect(entries().last()).toContainText("Behaviour implemented");
  const file = await readActivity();
  expect(file).toContain("quote: Acceptance criteria 2");
  expect(file).toContain("> Behaviour implemented");
  expect(file).toContain("Cover the quiet state too.");

  await box().fill("Second thought: keep the old wording.");
  await activity()
    .getByRole("button", { name: "Comment", exact: true })
    .click();
  await expect(entries().last()).toContainText("Second thought");

  await page.reload();
  await page.waitForURL(/#\/ticket\/FH-20$/);
  await expect(
    entries().filter({ hasText: "Cover the quiet state too." }),
  ).toHaveCount(1);
  await expect(activity().getByText("edited the priority")).toBeVisible();
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-20", { theme });
    await activity().scrollIntoViewIfNeeded();
    await expectNoAxeViolations(page, `activity with comments ${theme}`);
    await shot(page, `comments-activity-1440-${theme}`);
  }
});

test("a comment edits in place and shows as edited", async () => {
  await open(page, "#/ticket/FH-20");
  const second = entries().filter({ hasText: "Second thought" });
  await second.hover();
  await second.getByRole("button", { name: "Edit comment" }).click();
  const editor = main().getByRole("textbox", { name: "Comment", exact: true });
  await expect(editor).toBeFocused();
  await editor.fill("Second thought: keep the new wording.");
  await page.keyboard.press("Control+Enter");
  await expect(editor).toHaveCount(0);
  const edited = entries().filter({ hasText: "keep the new wording" });
  await expect(edited).toContainText("edited");
  await expect(
    edited.getByRole("button", { name: "Edit comment" }),
  ).toBeFocused();
  await expect.poll(readActivity).toMatch(/^edited: /m);
  await edited.scrollIntoViewIfNeeded();
  await shot(page, "comments-edited-1440-light");
});

test("an edit over a newer version opens the conflict dialog", async () => {
  await open(page, "#/ticket/FH-20");
  const second = entries().filter({ hasText: "keep the new wording" });
  await second.hover();
  await second.getByRole("button", { name: "Edit comment" }).click();
  await main()
    .getByRole("textbox", { name: "Comment", exact: true })
    .fill("Mine: keep both wordings.");
  // The comment changes on disk during the edit (another tab, a sync).
  const disk = await readActivity();
  await writeFile(
    activityFile(),
    disk.replace(
      "Second thought: keep the new wording.",
      "Theirs: keep the old wording after all.",
    ),
  );
  await main().getByRole("button", { name: "Save", exact: true }).click();
  const conflict = page.getByRole("dialog", {
    name: "FH-20 changed while you were editing",
  });
  await expect(conflict).toBeVisible();
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).toContainText("Theirs: keep the old wording after all.");
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).not.toContainText("Cover the quiet state too.");
  await expectNoAxeViolations(page, "comment conflict");
  await shot(page, "comments-conflict-1440-light", { keepFocus: true });
  await conflict.getByRole("button", { name: "Overwrite with mine" }).click();
  await expect.poll(readActivity).toContain("Mine: keep both wordings.");
  // The file shows the overwrite before its answer is back; the editor
  // closes on the answer, which must not land in the next test.
  await expect(
    main().getByRole("textbox", { name: "Comment", exact: true }),
  ).toHaveCount(0);
});

test("a failed post keeps the text with the error beside it", async () => {
  await open(page, "#/ticket/FH-20");
  await page.route("**/api/tickets/FH-20/comments", (route) =>
    route.fulfill({
      status: 500,
      contentType: "application/json",
      body: JSON.stringify({
        error: { code: "write_failed", message: "The disk is full." },
      }),
    }),
  );
  await box().fill("This will not post.");
  await activity()
    .getByRole("button", { name: "Comment", exact: true })
    .click();
  await expect(activity().getByRole("alert")).toContainText(
    "The disk is full.",
  );
  await expect(box()).toHaveValue("This will not post.");
  await activity()
    .getByText("posts", { exact: false })
    .scrollIntoViewIfNeeded();
  await shot(page, "comments-post-error-1440-light", { keepFocus: true });
  await page.unroute("**/api/tickets/FH-20/comments");

  // The draft is unsaved work: leaving asks first.
  await main()
    .getByRole("link", { name: /Show on the .* board/ })
    .click();
  const ask = page.getByRole("dialog", {
    name: "Unsaved changes to the comment",
  });
  await expect(ask).toBeVisible();
  await ask.getByRole("button", { name: "Discard changes" }).click();
  await expect(page).toHaveURL(/#\/p\/flashheart\/board/);
});

test("Activity filters by All, Comments and History, and the outline counts what it shows", async () => {
  await open(page, "#/ticket/FH-20");
  const outline = main()
    .getByRole("navigation", { name: "Sections" })
    .getByRole("button", { name: /^Activity/ });
  const all = await entries().count();
  await show("Comments");
  await expect(entries()).toHaveCount(2);
  await expect(outline).toHaveAccessibleName("Activity 2");
  await activity().scrollIntoViewIfNeeded();
  await shot(page, "comments-filter-comments-1440-light");
  await show("History");
  await expect(entries().first()).toContainText("edited the priority");
  await expect(entries().filter({ hasText: "commented" })).toHaveCount(0);
  await shot(page, "comments-filter-history-1440-light");
  await show("All");
  await expect(entries()).toHaveCount(all);
});

test("comments fit a phone and pass axe in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-20", { width: 390, height: 844, theme });
    await activity()
      .getByRole("combobox", { name: "Quote" })
      .selectOption({ label: "2. Behaviour implemented" });
    await box().fill("On a phone.");
    await activity()
      .getByText("posts", { exact: false })
      .scrollIntoViewIfNeeded();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
    expect(overflow).toBe(false);
    await expectNoAxeViolations(page, `comments 390 ${theme}`);
    await shot(page, `comments-390-${theme}`, { keepFocus: true });
    await box().fill("");
    await activity()
      .getByRole("combobox", { name: "Quote" })
      .selectOption({ label: "No quote" });
  }
});
