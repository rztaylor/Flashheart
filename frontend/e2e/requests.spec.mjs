import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { callTool, promptHook, seedClaimedRun } from "./agent-runs.mjs";
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

// Change requests in review (REV-6, FH-126), end to end: a person asks for
// changes on a ticket in review from its full page, a live session holding
// it gets them with its next prompt through the real hook, marks one
// addressed through the real `flashheart mcp`, and the person moves the
// ticket back (warned of the open one) and resolves the addressed one.
// One server over a writable demo board; tests run in order.
test.describe.configure({ mode: "serial" });

const SESSION = "aa11bb22-3c4d-4e5f-8a9b-0c1d2e3f4a5b";

let sandbox;
let server;
let context;
let page;
let claimed;
const ids = {};

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  // A live session holds FH-8, which is in review (its claim keeps it
  // there).
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
const properties = () => main().getByRole("region", { name: "Properties" });
const requests = () => main().getByRole("region", { name: /^Change requests/ });
const dialog = () =>
  page.getByRole("dialog", { name: "Request changes on FH-8" });
const activity = () => main().getByRole("region", { name: "Activity" });
// dismiss closes the toasts showing, so a screenshot shows the page.
const dismiss = async () => {
  for (const button of await page
    .getByRole("status")
    .getByRole("button", { name: "Dismiss" })
    .all())
    await button.click();
};

test("Request changes sends comments back and moves the ticket to In progress", async () => {
  await open(page, "#/ticket/FH-8");
  await expect(
    properties().getByRole("combobox", { name: "Move to" }),
  ).toHaveValue("review");
  // A comment written during the review, to send back with a new one.
  const box = activity().getByRole("textbox", { name: "Add a comment" });
  await box.fill("The header wraps on narrow screens.");
  await activity()
    .getByRole("button", { name: "Comment", exact: true })
    .click();
  await expect(
    activity().getByText("The header wraps on narrow screens."),
  ).toBeVisible();

  // The keyboard opens the dialog and sends it.
  await properties().getByRole("button", { name: "Request changes" }).focus();
  await page.keyboard.press("Enter");
  await expect(dialog()).toBeVisible();
  await dialog()
    .getByRole("checkbox", { name: /The header wraps/ })
    .check();
  await dialog()
    .getByRole("textbox", { name: "New change request" })
    .fill("Rename the button to Save.");
  await shot(page, "requests-dialog-1440-light", { keepFocus: true });
  await expectNoAxeViolations(page, "request changes dialog, light");
  await dialog()
    .getByRole("textbox", { name: "New change request" })
    .press("Control+Enter");
  await expect(dialog()).toBeHidden();
  await expect(
    page.getByRole("status").filter({
      hasText: "Requested 2 changes on FH-8; sent to claude:aa11bb22.",
    }),
  ).toBeVisible();
  await expect(
    properties().getByRole("combobox", { name: "Move to" }),
  ).toHaveValue("in-progress");
  await expect(requests()).toContainText("2 changes requested");
  // The branch is kept.
  await expect(properties()).toContainText("feature/locks");
});

test("the session holding the ticket gets them with its next prompt and marks one addressed", async () => {
  // The hook's output is JSON; its context is the text the model reads.
  const output = JSON.parse(promptHook(sandbox.root, SESSION, claimed.cwd));
  const prompted = output.hookSpecificOutput?.additionalContext ?? "";
  expect(prompted).toContain(
    "Changes requested by the user on the board (their own words):",
  );
  for (const [key, text] of [
    ["header", "The header wraps on narrow screens."],
    ["rename", "Rename the button to Save."],
  ]) {
    const match = new RegExp(
      `- FH-8 \\([^,]+, id ([0-9a-f]{8})\\): "${text.replace(".", "\\.")}"`,
    ).exec(prompted);
    expect(match, prompted).not.toBeNull();
    ids[key] = match?.[1];
  }
  const out = await callTool(sandbox.root, claimed.cwd, "update_ticket", {
    ticket: "FH-8",
    addressed: [ids.rename],
    append_notes: "Renamed the button to Save.",
    run: claimed.run,
  });
  expect(out).toContain("changed=notes,addressed");

  await expect(requests()).toContainText("1 open · 1 addressed");
  const rows = requests().getByRole("listitem");
  await expect(rows.nth(0)).toContainText("Change requested");
  await expect(rows.nth(1)).toContainText("Addressed");
  await expect(
    activity().getByText("addressed a change request", { exact: false }),
  ).toBeVisible();
  await dismiss();
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-8", { theme });
    await requests().scrollIntoViewIfNeeded();
    await shot(page, `requests-list-1440-${theme}`);
    await expectNoAxeViolations(page, `change requests, ${theme}`);
  }
  await open(page, "#/ticket/FH-8", { width: 390, height: 844 });
  await requests().scrollIntoViewIfNeeded();
  await shot(page, "requests-list-390-light");
  await expectNoAxeViolations(page, "change requests, 390");
});

test("moving back to review warns of the open request, naming it", async () => {
  await open(page, "#/ticket/FH-8");
  await properties()
    .getByRole("combobox", { name: "Move to" })
    .selectOption("review");
  const toast = page
    .getByRole("status")
    .filter({ hasText: "Moved FH-8 to Ready to review." });
  await expect(toast).toContainText("1 change request is still open:");
  await expect(toast).toContainText(ids.header);
  await expect(toast).toContainText("The header wraps on narrow screens.");
  await shot(page, "requests-review-warning-1440-light");
  await expect(
    properties().getByRole("combobox", { name: "Move to" }),
  ).toHaveValue("review");
});

test("the person resolves the addressed request; history records it", async () => {
  await dismiss();
  await open(page, "#/ticket/FH-8");
  await requests().scrollIntoViewIfNeeded();
  await shot(page, "requests-addressed-1440-light");
  const resolve = requests().getByRole("button", {
    name: "Resolve “Rename the button to Save.”",
  });
  await resolve.focus();
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("status").filter({
      hasText: "Resolved the change request on FH-8.",
    }),
  ).toBeVisible();
  await expect(requests()).toContainText("1 change requested");
  await expect(requests().getByRole("listitem").nth(1)).toContainText(
    "Resolved",
  );
  // Focus follows the request to its new place, on its Reopen.
  await expect(
    requests().getByRole("button", {
      name: "Reopen “Rename the button to Save.”",
    }),
  ).toBeFocused();
  await expect(
    activity().getByText("resolved a change request", { exact: false }),
  ).toBeVisible();
});

test("the card counts the open request", async () => {
  await dismiss();
  await open(page, "#/p/flashheart/board");
  const fh8 = card(page, "Store read side on os.Root");
  await expect(fh8).toHaveAccessibleName(/1 change requested/);
  await fh8.scrollIntoViewIfNeeded();
  await shot(page, "requests-card-1440-light");
  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/flashheart/board", { theme });
    await expectNoAxeViolations(page, `board with a change request, ${theme}`);
  }
});
