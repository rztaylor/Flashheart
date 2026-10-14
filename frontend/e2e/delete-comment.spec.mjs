import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { promptHook, seedClaimedRun } from "./agent-runs.mjs";
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

// Deleting a comment posted by mistake (FH-140), end to end: from the
// ticket's full page and the card panel's Comments tab, after a
// confirmation; a comment sent to a live session and deleted before it
// took it never reaches it through the real prompt hook; one the session
// has read, or an open change request, stays and says why. One server
// over a writable demo board, a live session holding FH-8 (in review);
// tests run in order.
test.describe.configure({ mode: "serial" });

const SESSION = "aa11bb22-3c4d-4e5f-8a9b-0c1d2e3f4a5b";

let sandbox;
let server;
let context;
let page;
let claimed;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  claimed = await seedClaimedRun(sandbox.home, sandbox.root, "FH-8");
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

const main = () => page.getByRole("main", { name: "Ticket FH-8" });
const activity = () => main().getByRole("region", { name: "Activity" });
const box = () => activity().getByRole("textbox", { name: "Add a comment" });
const tell = () =>
  activity().getByRole("checkbox", { name: "Tell a session now" });
const comment = (text, within = activity()) =>
  within.getByRole("listitem").filter({ hasText: text });
const deleteOf = (text, within) =>
  comment(text, within).getByRole("button", { name: "Delete comment" });
const confirmation = () =>
  page.getByRole("dialog", { name: "Delete this comment?" });
const activityFile = async () => {
  const tickets = join(sandbox.root, "flashheart", "tickets");
  const folder = (await readdir(tickets)).find((name) =>
    name.startsWith("FH-8-"),
  );
  return readFile(join(tickets, folder, "activity.md"), "utf8");
};
const dismiss = async () => {
  for (const button of await page
    .getByRole("status")
    .getByRole("button", { name: "Dismiss" })
    .all())
    await button.click();
};
const post = async (text, { send = false } = {}) => {
  await box().fill(text);
  if (send) await tell().check();
  await box().press("Control+Enter");
  // The box empties once the post's answer, and its toast, are back
  // (FH-156).
  await expect(box()).toHaveValue("");
  await expect(comment(text)).toBeVisible();
  await dismiss();
};

test("a comment is deleted from the full page after confirming, and stays gone", async () => {
  await open(page, "#/ticket/FH-8");
  await post("Posted on the wrong ticket.");
  // By keyboard: focus the control, Enter asks, Keep it is the default.
  await deleteOf("Posted on the wrong ticket.").focus();
  await page.keyboard.press("Enter");
  await expect(confirmation()).toBeVisible();
  await expect(
    confirmation().getByRole("button", { name: "Keep it" }),
  ).toBeFocused();
  await expect(confirmation()).toContainText(
    "The history keeps that you deleted a comment, not what it said.",
  );
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(page, `delete-comment-confirm-1440-${theme}`, {
      keepFocus: true,
    });
    await expectNoAxeViolations(page, `delete confirmation, ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await confirmation().getByRole("button", { name: "Delete comment" }).click();
  await expect(confirmation()).toBeHidden();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "Deleted your comment on FH-8." }),
  ).toBeVisible();
  await expect(comment("Posted on the wrong ticket.")).toHaveCount(0);
  await page.reload();
  await expect(activity()).toBeVisible();
  await expect(comment("Posted on the wrong ticket.")).toHaveCount(0);
  await expect(activity()).toContainText("deleted a comment");
  const file = await activityFile();
  expect(file).not.toContain("Posted on the wrong ticket.");
  expect(file).toMatch(/- deleted: comment [0-9a-f]{8}/);
});

test("a comment sent to the session and deleted before it took it never reaches it", async () => {
  await open(page, "#/ticket/FH-8");
  await post("Ignore the footer, wrong ticket.", { send: true });
  await expect(comment("Ignore the footer")).toContainText("Waiting for");
  await deleteOf("Ignore the footer").click();
  await expect(confirmation()).toContainText(
    "claude:aa11bb22 has not taken it yet and never will.",
  );
  await confirmation().getByRole("button", { name: "Delete comment" }).click();
  await expect(comment("Ignore the footer")).toHaveCount(0);
  const output = JSON.parse(
    promptHook(sandbox.root, SESSION, claimed.cwd) || "{}",
  );
  expect(JSON.stringify(output)).not.toContain("Ignore the footer");
  await dismiss();
});

test("a comment the session has read stays, and says why", async () => {
  await open(page, "#/ticket/FH-8");
  await post("Use the blue header.", { send: true });
  const prompted = JSON.parse(promptHook(sandbox.root, SESSION, claimed.cwd));
  expect(prompted.hookSpecificOutput?.additionalContext ?? "").toContain(
    "Use the blue header.",
  );
  await expect(comment("Use the blue header.")).toContainText("Delivered to");
  const control = deleteOf("Use the blue header.");
  await expect(control).toHaveAttribute("aria-disabled", "true");
  await expect(control).toHaveAccessibleDescription(
    "It can't be deleted: claude:aa11bb22 has read it.",
  );
  // Pressed (Playwright will not click a control marked disabled), it
  // shows the reason instead of asking.
  await control.focus();
  await page.keyboard.press("Enter");
  await expect(confirmation()).toHaveCount(0);
  await expect(
    comment("Use the blue header.").getByText(
      "It can't be deleted: claude:aa11bb22 has read it.",
    ),
  ).toBeVisible();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(page, `delete-comment-delivered-1440-${theme}`, {
      keepFocus: true,
    });
    await expectNoAxeViolations(page, `delete refused, delivered, ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});

test("an open change request stays until it is resolved, and says why", async () => {
  await open(page, "#/ticket/FH-8");
  const properties = main().getByRole("region", { name: "Properties" });
  await properties.getByRole("button", { name: "Request changes" }).click();
  const request = page.getByRole("dialog", {
    name: "Request changes on FH-8",
  });
  await request
    .getByRole("textbox", { name: "New change request" })
    .fill("Rename the button to Save.");
  await request
    .getByRole("textbox", { name: "New change request" })
    .press("Control+Enter");
  await expect(request).toBeHidden();
  await dismiss();
  const control = deleteOf("Rename the button to Save.");
  await expect(control).toHaveAttribute("aria-disabled", "true");
  await control.focus();
  await page.keyboard.press("Enter");
  const why = comment("Rename the button to Save.").getByText(
    "It can't be deleted while the change it asks for is open. Resolve it first.",
  );
  await expect(why).toBeVisible();
  await why.scrollIntoViewIfNeeded();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(page, `delete-comment-request-1440-${theme}`, {
      keepFocus: true,
    });
    await expectNoAxeViolations(page, `delete refused, request, ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});

test("a comment is deleted from the card panel's Comments tab", async () => {
  await open(page, "#/p/flashheart/board?t=FH-8");
  const panel = page.getByRole("complementary", { name: "Ticket FH-8" });
  await panel.getByRole("tab", { name: /^Comments/ }).click();
  const comments = panel.getByRole("tabpanel", { name: /^Comments/ });
  await comments
    .getByRole("textbox", { name: "Add a comment" })
    .fill("A slip in the panel.");
  await comments.getByRole("button", { name: "Comment", exact: true }).click();
  await expect(comment("A slip in the panel.", comments)).toBeVisible();
  await dismiss();
  await deleteOf("A slip in the panel.", comments).click();
  await expect(confirmation()).toBeVisible();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(page, `delete-comment-panel-1440-${theme}`, { keepFocus: true });
    await expectNoAxeViolations(page, `panel delete confirmation, ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await confirmation().getByRole("button", { name: "Delete comment" }).click();
  await expect(comment("A slip in the panel.", comments)).toHaveCount(0);
  await expect(panel).toBeVisible();
});
