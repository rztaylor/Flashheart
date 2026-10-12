import { readdir, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import {
  promptHook,
  seedBranchRun,
  seedClaimedRun,
  startSubagent,
} from "./agent-runs.mjs";
import { writeDemoBoard } from "./demo-board.mjs";
import {
  card,
  column,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// Keyboard focus across live reloads (FH-84, ui-layout.md "Keeping focus"):
// a focused card or row that moves keeps focus; one that disappears hands it
// to its neighbour, the next item in its list or else the previous one, or
// else the list's heading. The board changes underneath the page through its
// files, as an agent's MCP call or a text editor would change it.
test.describe.configure({ mode: "serial" });

let sandbox;
let branchRun;
let server;
let context;
let page;

const BRANCH_SESSION = "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9";

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  // FH-25 has two sessions: one on its branch with a Plan subagent, and a
  // later one that claims it and runs three subagents, so it is listed
  // first on the Runs tab.
  branchRun = await seedBranchRun(
    sandbox.home,
    sandbox.root,
    "flashheart",
    "feature/focus-ring-dark",
  );
  startSubagent(sandbox.root, BRANCH_SESSION, branchRun, "e1f2a3b4", "Plan");
  await seedClaimedRun(sandbox.home, sandbox.root, "FH-25");
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext();
  page = await context.newPage();
  await page.goto(url);
  await page.waitForURL((current) => current.pathname === "/");
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

const tickets = () => join(sandbox.root, "flashheart", "tickets");

// folderOf is a ticket's folder by id, whatever its slug.
async function folderOf(id) {
  const name = (await readdir(tickets())).find((item) =>
    item.startsWith(`${id}-`),
  );
  if (!name) throw new Error(`no folder for ${id}`);
  return name;
}

// rewrite changes a field in a ticket's file, as another writer would.
async function rewrite(id, field, value) {
  const folder = await folderOf(id);
  const file = join(tickets(), folder, `${folder}.md`);
  const data = await readFile(file, "utf8");
  await writeFile(
    file,
    data.replace(new RegExp(`^${field}: .*$`, "m"), `${field}: ${value}`),
  );
}
const setStatus = (id, status) => rewrite(id, "status", status);
const remove = async (id) =>
  rm(join(tickets(), await folderOf(id)), { recursive: true });

// focusedTicket is the ticket id on the focused element, or the focused
// element's tag when it carries none.
const focusedTicket = () =>
  page.evaluate(() => {
    const active = document.activeElement;
    return active?.getAttribute("data-ticket") ?? active?.tagName.toLowerCase();
  });

// focusedRow is the Overview row holding focus, or the focused element's
// tag when it is in none.
const focusedRow = () =>
  page.evaluate(
    () =>
      document.activeElement?.closest("[data-row]")?.getAttribute("data-row") ??
      document.activeElement?.tagName.toLowerCase(),
  );

// shown lists a column's card ids from top to bottom.
const shown = (name) =>
  column(page, name)
    .locator("[data-ticket]")
    .evaluateAll((nodes) => nodes.map((node) => node.dataset.ticket));

test("a focused card that moves column on a live reload keeps focus", async () => {
  await open(page, "#/p/flashheart/board");
  await card(page, "Sortable table view").focus();
  await expect(card(page, "Sortable table view")).toBeFocused();
  await setStatus("FH-13", "in-progress");
  await expect(
    column(page, "In progress").getByRole("button", {
      name: /^Sortable table view,/,
    }),
  ).toBeVisible();
  await expect.poll(focusedTicket).toBe("FH-13");
});

test("a focused card that disappears hands focus to the next card, else the previous", async () => {
  await open(page, "#/p/flashheart/board");
  const before = await shown("Up next");
  expect(before.length).toBeGreaterThanOrEqual(3);
  const [first, second] = before;
  await column(page, "Up next").locator(`[data-ticket="${first}"]`).focus();
  await remove(first);
  await expect(column(page, "Up next").locator("[data-ticket]")).toHaveCount(
    before.length - 1,
  );
  await expect.poll(focusedTicket).toBe(second);

  // The last card in the column has no next: the previous one takes focus.
  const now = await shown("Up next");
  const last = now.at(-1);
  await column(page, "Up next").locator(`[data-ticket="${last}"]`).focus();
  await remove(last);
  await expect(column(page, "Up next").locator("[data-ticket]")).toHaveCount(
    now.length - 1,
  );
  await expect.poll(focusedTicket).toBe(now.at(-2));

  // The column's last card goes: its heading takes focus.
  const [only, ...rest] = await shown("Up next");
  expect(rest).toEqual([]);
  await column(page, "Up next").locator(`[data-ticket="${only}"]`).focus();
  await remove(only);
  await expect(column(page, "Up next").getByText("No tickets")).toBeVisible();
  await expect(
    column(page, "Up next").getByRole("heading", { name: /Up next/ }),
  ).toBeFocused();
  await shot(page, "focus-column-heading-1440-light", { keepFocus: true });
});

test("a focused Overview row follows its ticket to another section, and hands focus on when it leaves", async () => {
  await open(page, "#/p/flashheart/overview");
  const review = page.getByRole("region", { name: "Ready for your review" });
  await review
    .locator('[data-row="FH-8"]')
    .getByRole("button", { name: "Store read side on os.Root" })
    .focus();
  // Back in progress, the row moves to another section and keeps focus.
  await setStatus("FH-8", "in-progress");
  await expect(review.locator('[data-row="FH-8"]')).toHaveCount(0);
  await expect(page.locator('[data-row="FH-8"]')).not.toHaveCount(0);
  await expect.poll(focusedRow).toBe("FH-8");
  await expect(
    page.locator('[data-row="FH-8"]').getByRole("button", {
      name: "Store read side on os.Root",
    }),
  ).toBeFocused();

  // Done is on no section: focus moves to the row's neighbour.
  const neighbours = await page.evaluate(() => {
    const section = document.activeElement?.closest("section[data-section]");
    return [...(section?.querySelectorAll("[data-row]") ?? [])].map((node) =>
      node.getAttribute("data-row"),
    );
  });
  const at = neighbours.indexOf("FH-8");
  const expected = neighbours[at + 1] ?? neighbours[at - 1];
  expect(expected).toBeTruthy();
  await setStatus("FH-8", "done");
  await expect(page.locator('[data-row="FH-8"]')).toHaveCount(0);
  await expect.poll(focusedRow).toBe(expected);
});

test("a focused Runs-tab row keeps focus when its session moves down the list", async () => {
  await open(page, "#/p/flashheart/board?t=FH-25");
  const panel = page.getByRole("complementary", { name: "Ticket FH-25" });
  await panel.getByRole("tab", { name: /^Runs/ }).click();
  const sessions = panel.locator("article");
  await expect(sessions).toHaveCount(2);
  const toggle = panel.getByRole("button", { name: "general-purpose" });
  await expect(sessions.first().getByRole("button")).toContainText([
    "general-purpose",
  ]);
  await toggle.focus();
  // The branch session acts: it is now the most recent, listed first, and
  // the claimed session with its subagents moves down.
  promptHook(sandbox.root, BRANCH_SESSION, branchRun);
  await expect(sessions.last().getByRole("button")).toContainText([
    "general-purpose",
  ]);
  await expect(toggle).toBeFocused();
});

test("a focused Runs-tab row whose session unlinks hands focus to the next session", async () => {
  await open(page, "#/p/flashheart/board?t=FH-25");
  const panel = page.getByRole("complementary", { name: "Ticket FH-25" });
  await panel.getByRole("tab", { name: /^Runs/ }).click();
  const sessions = panel.locator("article");
  await expect(sessions).toHaveCount(2);
  const plan = panel.getByRole("button", { name: "Plan", exact: true });
  await expect(sessions.first().getByRole("button")).toHaveText(["Plan"]);
  await plan.focus();
  // The ticket moves to another branch: the branch session is no longer
  // linked, and its row goes with it.
  await rewrite("FH-25", "branch", "feature/focus-ring-elsewhere");
  await expect(sessions).toHaveCount(1);
  // The next session's first subagent row takes focus.
  await expect(sessions.first().getByRole("button").first()).toBeFocused();
});

test("focus outside the board stays put while a card disappears", async () => {
  await open(page, "#/p/flashheart/board");
  const search = page.getByRole("searchbox", { name: "Search tickets" });
  await search.focus();
  const [first] = await shown("Review");
  await setStatus(first, "backlog");
  await expect(
    column(page, "Review").locator(`[data-ticket="${first}"]`),
  ).toHaveCount(0);
  await expect(search).toBeFocused();
});
