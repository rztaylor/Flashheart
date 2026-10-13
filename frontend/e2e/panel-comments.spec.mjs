import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  card,
  expectNoAxeViolations,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// Comments in the board's card panel (FH-128): the Comments tab shows the
// ticket's comments and the full page's comment box. One server over a
// writable demo board; tests run in order and each leaves the board as the
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
const activityFile = () =>
  join(sandbox.root, "flashheart", "tickets", folder, "activity.md");
const readActivity = () => readFile(activityFile(), "utf8").catch(() => "");

const boardWithPanel = "#/p/flashheart/board?t=FH-20";
const panel = () => page.getByRole("complementary", { name: "Ticket FH-20" });
const commentsTab = () => panel().getByRole("tab", { name: /^Comments/ });
const comments = () => panel().getByRole("tabpanel", { name: /^Comments/ });
const box = () => comments().getByRole("textbox", { name: "Add a comment" });
const entries = () => comments().getByRole("listitem");
const post = () =>
  comments().getByRole("button", { name: "Comment", exact: true });

// showComments opens the panel on the ticket and chooses its Comments tab.
async function showComments(options) {
  await open(page, boardWithPanel, options);
  await commentsTab().click();
  await expect(commentsTab()).toHaveAttribute("aria-selected", "true");
}

test("a comment posted from the panel shows on the ticket's full page", async () => {
  await showComments();
  await expect(comments().getByText("No comments yet.")).toBeVisible();
  await expect(post()).toBeDisabled();
  await comments()
    .getByRole("combobox", { name: "Quote" })
    .selectOption({ label: "2. Behaviour implemented" });
  await expect(
    comments().getByRole("figure", { name: "Quoting Acceptance criteria 2" }),
  ).toContainText("Behaviour implemented");
  await box().fill("Cover the quiet state too.");
  await expectNoAxeViolations(page, "panel comment box with a quote");
  await shot(page, "panel-comments-box-1440-light", { keepFocus: true });
  await page.keyboard.press("Control+Enter");
  await expect(box()).toHaveValue("");
  await expect(entries().last()).toContainText("Cover the quiet state too.");
  await expect(entries().last()).toContainText("Behaviour implemented");
  await expect(commentsTab()).toHaveAccessibleName("Comments 1");
  const file = await readActivity();
  expect(file).toContain("quote: Acceptance criteria 2");
  expect(file).toContain("Cover the quiet state too.");

  await box().fill("Second thought: keep the old wording.");
  await post().click();
  await expect(entries().last()).toContainText("Second thought");

  await open(page, "#/ticket/FH-20");
  const activity = page
    .getByRole("main", { name: "Ticket FH-20" })
    .getByRole("region", { name: "Activity" });
  await expect(
    activity
      .getByRole("listitem")
      .filter({ hasText: "Cover the quiet state too." }),
  ).toHaveCount(1);
  await expect(
    activity.getByRole("listitem").filter({ hasText: "Second thought" }),
  ).toHaveCount(1);
});

test("the panel's comments pass axe in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    await showComments({ theme });
    await expect(entries()).toHaveCount(2);
    await expectNoAxeViolations(page, `panel comments ${theme}`);
    await shot(page, `panel-comments-1440-${theme}`);
  }
});

test("a comment edits in place in the panel and shows as edited", async () => {
  await showComments();
  const second = entries().filter({ hasText: "Second thought" });
  await second.hover();
  await second.getByRole("button", { name: "Edit comment" }).click();
  const editor = comments().getByRole("textbox", {
    name: "Comment",
    exact: true,
  });
  await expect(editor).toBeFocused();
  await editor.fill("Second thought: keep the new wording.");
  await shot(page, "panel-comments-editing-1440-light", { keepFocus: true });
  await page.keyboard.press("Control+Enter");
  await expect(editor).toHaveCount(0);
  const edited = entries().filter({ hasText: "keep the new wording" });
  await expect(edited).toContainText("edited");
  await expect(
    edited.getByRole("button", { name: "Edit comment" }),
  ).toBeFocused();
  await expect.poll(readActivity).toMatch(/^edited: /m);
});

test("an edit over a newer version opens the conflict dialog", async () => {
  await showComments();
  const second = entries().filter({ hasText: "keep the new wording" });
  await second.hover();
  await second.getByRole("button", { name: "Edit comment" }).click();
  await comments()
    .getByRole("textbox", { name: "Comment", exact: true })
    .fill("Mine: keep both wordings.");
  const disk = await readActivity();
  await writeFile(
    activityFile(),
    disk.replace(
      "Second thought: keep the new wording.",
      "Theirs: keep the old wording after all.",
    ),
  );
  await comments().getByRole("button", { name: "Save", exact: true }).click();
  const conflict = page.getByRole("dialog", {
    name: "FH-20 changed while you were editing",
  });
  await expect(conflict).toBeVisible();
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).toContainText("Theirs: keep the old wording after all.");
  await expectNoAxeViolations(page, "panel comment conflict");
  await shot(page, "panel-comments-conflict-1440-light", { keepFocus: true });
  await conflict.getByRole("button", { name: "Overwrite with mine" }).click();
  await expect.poll(readActivity).toContain("Mine: keep both wordings.");
  await expect(
    entries().filter({ hasText: "Mine: keep both wordings." }),
  ).toHaveCount(1);
});

test("a comment being written survives other tabs, and closing the panel asks first", async () => {
  await showComments();
  await box().fill("Half a thought");
  await panel().getByRole("tab", { name: "Ticket" }).click();
  await commentsTab().click();
  await expect(box()).toHaveValue("Half a thought");

  await panel().getByRole("button", { name: "Close ticket" }).click();
  const ask = page.getByRole("dialog", {
    name: "Unsaved changes to the comment",
  });
  await expect(ask).toBeVisible();
  await expectNoAxeViolations(page, "panel unsaved comment");
  await shot(page, "panel-comments-unsaved-1440-light", { keepFocus: true });
  await ask.getByRole("button", { name: "Keep editing" }).click();
  await expect(panel()).toBeVisible();
  await expect(box()).toHaveValue("Half a thought");

  // Opening another ticket asks as well; kept, the panel stays on FH-20.
  await card(page, "Claude Code hook adapter").click();
  await expect(ask).toBeVisible();
  await ask.getByRole("button", { name: "Keep editing" }).click();
  await expect(page).toHaveURL(/\?t=FH-20$/);
  await expect(box()).toHaveValue("Half a thought");

  // Escape closes the panel, asking first; discarded, focus returns to the
  // ticket's card.
  await box().focus();
  await page.keyboard.press("Escape");
  await expect(ask).toBeVisible();
  await ask.getByRole("button", { name: "Discard changes" }).click();
  await expect(panel()).toHaveCount(0);
  await expect(page).toHaveURL(/#\/p\/flashheart\/board$/);
  await expect(card(page, "Run state derivation table")).toBeFocused();
});

test("the panel's comments fit a phone and pass axe in both themes", async () => {
  for (const theme of ["light", "dark"]) {
    await showComments({ width: 390, height: 844, theme });
    await comments()
      .getByRole("combobox", { name: "Quote" })
      .selectOption({ label: "2. Behaviour implemented" });
    await box().fill("On a phone.");
    await post().scrollIntoViewIfNeeded();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
    expect(overflow).toBe(false);
    const fits = await comments().evaluate(
      (element) => element.scrollWidth <= element.clientWidth,
    );
    expect(fits).toBe(true);
    await expectNoAxeViolations(page, `panel comments 390 ${theme}`);
    await shot(page, `panel-comments-390-${theme}`, { keepFocus: true });
    await box().fill("");
    await comments()
      .getByRole("combobox", { name: "Quote" })
      .selectOption({ label: "No quote" });
  }
});
