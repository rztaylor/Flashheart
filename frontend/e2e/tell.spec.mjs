import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { endSession, promptHook, seedClaimedRun } from "./agent-runs.mjs";
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

// Tell a session now (D47, FH-127), end to end: a live session holds FH-8
// (with its subagents); from the ticket's full page a person sends it a
// comment, the session gets it with its next prompt through the real
// hook, and the page shows it delivered and locked. A comment the session
// never takes, because it ended, says so, and the option is then off.
// One server over a writable demo board; tests run in order.
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

const main = (id = "FH-8") => page.getByRole("main", { name: `Ticket ${id}` });
const activity = (id = "FH-8") =>
  main(id).getByRole("region", { name: "Activity" });
const box = (id) =>
  activity(id).getByRole("textbox", { name: "Add a comment" });
const tell = (id) =>
  activity(id).getByRole("checkbox", { name: "Tell a session now" });
const picker = () => activity().getByRole("combobox", { name: "Session" });
const comment = (text) =>
  activity().getByRole("listitem").filter({ hasText: text });
const dismiss = async () => {
  for (const button of await page
    .getByRole("status")
    .getByRole("button", { name: "Dismiss" })
    .all())
    await button.click();
};

test("a ticket no live session holds offers no session, saying why", async () => {
  await open(page, "#/ticket/FH-1");
  await box("FH-1").scrollIntoViewIfNeeded();
  await expect(tell("FH-1")).toBeDisabled();
  await expect(tell("FH-1")).toHaveAccessibleDescription(
    "No live session holds FH-1 now.",
  );
  await shot(page, "tell-disabled-1440-light");
  await expectNoAxeViolations(page, "tell disabled, light");
});

test("the comment box offers the session holding FH-8, with its subagents", async () => {
  await open(page, "#/ticket/FH-8");
  await box().fill("Use the blue header, not the raspberry one.");
  // By keyboard: Tab from the text to the option, Space to check it.
  await box().focus();
  await page.keyboard.press("Tab");
  await expect(tell()).toBeFocused();
  await page.keyboard.press("Space");
  await expect(tell()).toBeChecked();
  await expect(picker()).toHaveValue(claimed.run);
  const groups = await picker()
    .locator("optgroup")
    .evaluateAll((list) =>
      list.map((group) => [
        group.label,
        ...[...group.querySelectorAll("option")].map((o) => o.textContent),
      ]),
    );
  expect(groups).toEqual([
    [
      "Session claude:aa11bb22",
      expect.stringMatching(/^claude:aa11bb22 · /),
      expect.stringMatching(/^Explore subagent · /),
      expect.stringMatching(/^general-purpose subagent · /),
    ],
  ]);
  await box().scrollIntoViewIfNeeded();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(page, `tell-picker-1440-${theme}`);
    await expectNoAxeViolations(page, `tell picker, ${theme}`);
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await box().press("Control+Enter");
  await expect(
    page.getByRole("status").filter({
      hasText: "Commented on FH-8 and sent it to claude:aa11bb22.",
    }),
  ).toBeVisible();
  await expect(comment("Use the blue header")).toContainText(
    "Waiting for claude:aa11bb22 to take it with its next prompt or tool call.",
  );
  // Until it is delivered it can still be edited.
  await expect(
    comment("Use the blue header").getByRole("button", {
      name: "Edit comment",
    }),
  ).toBeVisible();
  await expect(tell()).not.toBeChecked();
  await dismiss();
  await comment("Use the blue header").scrollIntoViewIfNeeded();
  await shot(page, "tell-waiting-1440-light");
});

test("the session gets it with its next prompt; the page shows it delivered and locked", async () => {
  const output = JSON.parse(promptHook(sandbox.root, SESSION, claimed.cwd));
  const prompted = output.hookSpecificOutput?.additionalContext ?? "";
  expect(prompted).toContain(
    "[Flashheart] Comments from the user on the board (their own words):",
  );
  expect(prompted).toMatch(
    /- FH-8 \([^)]+\): "Use the blue header, not the raspberry one\."/,
  );
  // The page follows without a reload.
  await expect(comment("Use the blue header")).toContainText(
    /Delivered to claude:aa11bb22 at .+\. It can no longer be edited\./,
  );
  await expect(
    comment("Use the blue header").getByRole("button", {
      name: "Edit comment",
    }),
  ).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-8", { theme });
    await comment("Use the blue header").scrollIntoViewIfNeeded();
    await shot(page, `tell-delivered-1440-${theme}`);
    await expectNoAxeViolations(page, `tell delivered, ${theme}`);
  }
  await open(page, "#/ticket/FH-8", { width: 390, height: 844 });
  await comment("Use the blue header").scrollIntoViewIfNeeded();
  await shot(page, "tell-delivered-390-light");
  await expectNoAxeViolations(page, "tell delivered, 390");
  await box().scrollIntoViewIfNeeded();
  await tell().check();
  await shot(page, "tell-picker-390-light");
  await expectNoAxeViolations(page, "tell picker, 390");
  await tell().uncheck();
});

test("a comment the session never took says so, and the option is then off", async () => {
  await open(page, "#/ticket/FH-8");
  await box().fill("Also check the footer.");
  await tell().check();
  await box().press("Control+Enter");
  await expect(comment("Also check the footer.")).toContainText("Waiting for");
  endSession(sandbox.root, SESSION, claimed.cwd);
  await expect(comment("Also check the footer.")).toContainText(
    "Not delivered: claude:aa11bb22 ended before taking it. It stays on the ticket.",
  );
  await expect(tell()).toBeDisabled();
  await expect(tell()).toHaveAccessibleDescription(
    "No live session holds FH-8 now.",
  );
  // Not delivered, so still the person's to edit.
  await expect(
    comment("Also check the footer.").getByRole("button", {
      name: "Edit comment",
    }),
  ).toBeVisible();
  await dismiss();
  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/FH-8", { theme });
    await comment("Also check the footer.").scrollIntoViewIfNeeded();
    await shot(page, `tell-missed-1440-${theme}`);
    await expectNoAxeViolations(page, `tell missed, ${theme}`);
  }
});
