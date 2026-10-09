import { execFileSync } from "node:child_process";
import {
  mkdir,
  readdir,
  readFile,
  realpath,
  writeFile,
} from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  filterButton,
  filterMenu,
  launch,
  makeSandbox,
  screenshotDir,
  stopIfRunning,
  viewOption,
  waitForManualURL,
} from "./support.mjs";

// Editing (board-editing): one server over a writable demo board. Tests run
// in order and each leaves the board as the next expects.
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

const ticketFile = (folder) =>
  join(sandbox.root, "flashheart", "tickets", folder, `${folder}.md`);
const readTicket = (folder) => readFile(ticketFile(folder), "utf8");

async function open(hash) {
  await page.evaluate((target) => {
    window.location.hash = target;
  }, hash);
  await expect(
    page.getByRole("button", { name: "Backend connected. Check connection" }),
  ).toBeVisible();
}

function column(name) {
  return page.getByRole("region", { name });
}

function card(title) {
  return page.getByRole("button", { name: new RegExp(`^${title},`) });
}

async function expectNoAxeViolations(label) {
  const results = await new AxeBuilder({ page }).analyze();
  expect(
    results.violations,
    `${label}: ${results.violations.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`,
  ).toEqual([]);
}

test("Shift and an arrow move a ticket to the next column, with Undo", async () => {
  await open("#/p/flashheart/board");
  await card("Release notes template").focus();
  await page.keyboard.press("Shift+ArrowRight");
  await expect(
    column("Up next").getByRole("button", { name: /^Release notes template,/ }),
  ).toBeVisible();
  await expect(card("Release notes template")).toBeFocused();
  const toast = page
    .getByRole("status")
    .filter({ hasText: "Moved FH-32 to Up next." });
  await expect(toast).toBeVisible();
  await expect
    .poll(() => readTicket("FH-32-release-notes"))
    .toContain("status: up-next");

  await toast.getByRole("button", { name: "Undo" }).click();
  await expect(
    column("Backlog").getByRole("button", { name: /^Release notes template,/ }),
  ).toBeVisible();
  await expect
    .poll(() => readTicket("FH-32-release-notes"))
    .toContain("status: backlog");
});

test("dragging a card to another column moves it", async () => {
  await open("#/p/flashheart/board");
  await card("First-run user guide").scrollIntoViewIfNeeded();
  const source = await card("First-run user guide").boundingBox();
  const target = await column("In progress").boundingBox();
  await page.mouse.move(source.x + source.width / 2, source.y + 20);
  await page.mouse.down();
  await page.mouse.move(source.x + source.width / 2 + 20, source.y + 30, {
    steps: 4,
  });
  // Columns scroll together, so the target is level with the source card.
  await page.mouse.move(target.x + target.width / 2, source.y + 20, {
    steps: 12,
  });
  await page.mouse.up();
  await expect(
    column("In progress").getByRole("button", {
      name: /^First-run user guide,/,
    }),
  ).toBeVisible();
  await expect
    .poll(() => readTicket("FH-33-user-guide"))
    .toContain("status: in-progress");
});

// ticketById reads a ticket file by id, whatever its folder's slug.
async function ticketById(id) {
  const dir = join(sandbox.root, "flashheart", "tickets");
  const folder = (await readdir(dir)).find((name) => name.startsWith(`${id}-`));
  return readFile(join(dir, folder, `${folder}.md`), "utf8");
}

function order(name) {
  return column(name)
    .locator("[data-ticket]")
    .evaluateAll((nodes) => nodes.map((node) => node.dataset.ticket));
}

async function workstreamFiles() {
  const dir = join(sandbox.root, "flashheart", "workstreams");
  const names = (await readdir(dir)).sort();
  return Promise.all(names.map((name) => readFile(join(dir, name), "utf8")));
}

// boardTop scrolls the board back to its first cards, which earlier tests
// may have scrolled away.
async function boardTop() {
  await page.locator(".board-grid").evaluate((element) => {
    element.scrollTop = 0;
  });
}

// drag moves a card with the pointer to a point, in small steps so the
// drag sensor and drop indicator follow.
async function drag(source, x, y) {
  await source.scrollIntoViewIfNeeded();
  const box = await source.boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + 20);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2 + 10, box.y + 30, { steps: 4 });
  await page.mouse.move(x, y, { steps: 12 });
  await page.mouse.up();
}

test("dragging a card within a column sets its place, and it stays", async () => {
  await open("#/p/flashheart/board");
  await boardTop();
  const lines = await workstreamFiles();
  const [first, second, third] = await order("Backlog");
  const target = await column("Backlog")
    .locator(`[data-ticket="${first}"]`)
    .boundingBox();
  await drag(
    column("Backlog").locator(`[data-ticket="${third}"]`),
    target.x + target.width / 2,
    target.y + 6,
  );
  await expect
    .poll(async () => (await order("Backlog")).slice(0, 3))
    .toEqual([third, first, second]);
  await expect.poll(() => ticketById(third)).toMatch(/\nrank: \S+\n/);
  const toast = page
    .getByRole("status")
    .filter({ hasText: `Moved ${third} in Backlog.` });
  await expect(toast).toBeVisible();

  // The order is the saved one: it survives a reload and a trip elsewhere.
  await page.reload();
  await open("#/p/flashheart/table");
  await open("#/p/flashheart/board");
  await boardTop();
  await expect
    .poll(async () => (await order("Backlog")).slice(0, 3))
    .toEqual([third, first, second]);

  // Undo puts it back where it was.
  await drag(
    column("Backlog").locator(`[data-ticket="${second}"]`),
    target.x + target.width / 2,
    target.y + 6,
  );
  await expect.poll(async () => (await order("Backlog"))[0]).toBe(second);
  await page
    .getByRole("status")
    .filter({ hasText: `Moved ${second} in Backlog.` })
    .getByRole("button", { name: "Undo" })
    .click();
  await expect
    .poll(async () => (await order("Backlog")).slice(0, 3))
    .toEqual([third, first, second]);
  // Board order never touches a workstream's own order.
  expect(await workstreamFiles()).toEqual(lines);
});

test("a card dropped in another column lands where it was dropped", async () => {
  await open("#/p/flashheart/board");
  await boardTop();
  const [moving] = await order("Backlog");
  const [top, next] = await order("Up next");
  const below = await column("Up next")
    .locator(`[data-ticket="${next}"]`)
    .boundingBox();
  await drag(
    column("Backlog").locator(`[data-ticket="${moving}"]`),
    below.x + below.width / 2,
    below.y + 6,
  );
  await expect
    .poll(async () => (await order("Up next")).slice(0, 3))
    .toEqual([top, moving, next]);
  await expect.poll(() => ticketById(moving)).toContain("status: up-next");
});

test("a drop lands at the pointer after the board scrolls mid-drag", async () => {
  await open("#/p/flashheart/board");
  await boardTop();
  const backlog = column("Backlog");
  const ids = await order("Backlog");
  const moving = ids[0];
  const source = await backlog
    .locator(`[data-ticket="${moving}"]`)
    .boundingBox();
  await page.mouse.move(source.x + source.width / 2, source.y + 20);
  await page.mouse.down();
  await page.mouse.move(source.x + source.width / 2 + 10, source.y + 30, {
    steps: 4,
  });
  // Hold the card at the board's bottom edge until it scrolls, then aim
  // above a card that is now in view.
  const edge = await page.locator(".board-grid").boundingBox();
  await page.mouse.move(source.x + source.width / 2, edge.y + edge.height - 6, {
    steps: 10,
  });
  await expect
    .poll(() =>
      page.locator(".board-grid").evaluate((element) => element.scrollTop),
    )
    .toBeGreaterThan(300);
  await page.mouse.move(source.x + source.width / 2, edge.y + edge.height / 2, {
    steps: 4,
  });
  const grid = await page.locator(".board-grid").boundingBox();
  const target = await backlog.locator("[data-ticket]").evaluateAll(
    (nodes, middle) =>
      nodes
        .map((node) => ({
          id: node.dataset.ticket,
          box: node.getBoundingClientRect(),
        }))
        .find(({ box }) => box.top > middle)?.id,
    grid.y + grid.height / 2,
  );
  const box = await backlog.locator(`[data-ticket="${target}"]`).boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + 6, { steps: 8 });
  await page.mouse.up();
  await expect
    .poll(async () => {
      const now = await order("Backlog");
      return now[now.indexOf(target) - 1];
    })
    .toBe(moving);
});

test("Shift with Up or Down moves a card within its column, with Undo", async () => {
  await open("#/p/flashheart/board");
  const [first, second] = await order("Up next");
  await column("Up next").locator(`[data-ticket="${second}"]`).focus();
  await page.keyboard.press("Shift+ArrowUp");
  await expect
    .poll(async () => (await order("Up next")).slice(0, 2))
    .toEqual([second, first]);
  await expect(
    column("Up next").locator(`[data-ticket="${second}"]`),
  ).toBeFocused();
  await page.keyboard.press("Shift+ArrowDown");
  await expect
    .poll(async () => (await order("Up next")).slice(0, 2))
    .toEqual([first, second]);
  await page
    .getByRole("status")
    .filter({ hasText: `Moved ${second} in Up next.` })
    .getByRole("button", { name: "Undo" })
    .click();
  await expect
    .poll(async () => (await order("Up next")).slice(0, 2))
    .toEqual([second, first]);
});

test("the panel moves a ticket within its column", async () => {
  await open("#/p/flashheart/board");
  const ids = await order("Up next");
  const last = ids.at(-1);
  await open(`#/p/flashheart/board?t=${last}`);
  const panel = page.getByRole("complementary", { name: `Ticket ${last}` });
  await panel.getByRole("button", { name: "Top of its column" }).click();
  await expect.poll(async () => (await order("Up next"))[0]).toBe(last);
  await panel.getByRole("button", { name: "Down one place" }).click();
  await expect.poll(async () => (await order("Up next"))[1]).toBe(last);
  await panel.getByRole("button", { name: "Bottom of its column" }).click();
  await expect.poll(async () => (await order("Up next")).at(-1)).toBe(last);
  await page.keyboard.press("Escape");
});

test("starting a blocked ticket asks for a reason and records it", async () => {
  await open("#/p/flashheart/board");
  await card("Workstreams as transit lines").focus();
  await page.keyboard.press("Shift+ArrowRight");
  const dialog = page.getByRole("dialog", {
    name: "Start FH-12 while it is blocked?",
  });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Depends on FH-11");
  await expect(
    dialog.getByRole("button", { name: "Start anyway" }),
  ).toBeDisabled();
  await expectNoAxeViolations("blocked move dialog");
  await page.screenshot({
    path: resolve(screenshotDir, "blocked-move-1440-light.png"),
  });
  await dialog.getByRole("textbox").fill("Demo for the review on Friday");
  await dialog.getByRole("button", { name: "Start anyway" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(
    column("In progress").getByRole("button", {
      name: /^Workstreams as transit lines,/,
    }),
  ).toBeVisible();
  await expect
    .poll(() => readTicket("FH-12-workstream-diagram"))
    .toContain("Started while blocked: Demo for the review on Friday");
});

test("moving into review warns about unticked criteria", async () => {
  await open("#/p/flashheart/board?t=FH-33");
  const panel = page.getByRole("complementary", { name: "Ticket FH-33" });
  await panel.getByLabel("Move to").selectOption("review");
  const toast = page
    .getByRole("status")
    .filter({ hasText: "Moved FH-33 to Ready to review." });
  await expect(toast).toContainText("There is no review file yet.");
  await expect(toast).toContainText("2 acceptance criteria are not ticked.");
});

test("criteria tick in the panel and save to the file", async () => {
  await open("#/p/flashheart/board?t=FH-33");
  const panel = page.getByRole("complementary", { name: "Ticket FH-33" });
  await panel.getByRole("checkbox", { name: "Behaviour implemented" }).check();
  await expect(
    panel.getByRole("checkbox", { name: "Behaviour implemented" }),
  ).toBeChecked();
  await expect
    .poll(() => readTicket("FH-33-user-guide"))
    .toContain("- [x] Behaviour implemented");
});

test("review steps tick and screenshots open in a lightbox", async () => {
  await open("#/p/flashheart/board?t=FH-8");
  const panel = page.getByRole("complementary", { name: "Ticket FH-8" });
  await panel.getByRole("tab", { name: "Review" }).click();
  const steps = panel.getByRole("region", { name: /Steps/ });
  await expect(steps).toContainText("0 of 2");
  await steps.getByRole("checkbox", { name: /Run scripts\/check\.sh/ }).check();
  await expect(steps).toContainText("1 of 2");
  const folder = (
    await readdir(join(sandbox.root, "flashheart", "tickets"))
  ).find((name) => name.startsWith("FH-8-"));
  await expect
    .poll(() =>
      readFile(
        join(sandbox.root, "flashheart", "tickets", folder, "review.md"),
        "utf8",
      ),
    )
    .toContain("1. [x] Run `scripts/check.sh`.");

  await panel.getByRole("tab", { name: /Attachments/ }).click();
  await panel
    .getByRole("button", { name: "Open Store tests passing (synthetic)" })
    .click();
  const lightbox = page.getByRole("dialog", {
    name: "Store tests passing (synthetic)",
  });
  await expect(lightbox.getByRole("img")).toBeVisible();
  await expect(lightbox.getByRole("button", { name: "Next" })).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(`lightbox ${theme}`);
    await page.screenshot({
      path: resolve(screenshotDir, `lightbox-1440-${theme}.png`),
    });
  }
  await page.keyboard.press("Escape");
  await expect(lightbox).toHaveCount(0);
  await panel.getByRole("tab", { name: "Review" }).click();
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(`review steps ${theme}`);
    await page.screenshot({
      path: resolve(screenshotDir, `review-steps-1440-${theme}.png`),
    });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});

test("the Edit tab saves fields and shows conflicts side by side", async () => {
  await open("#/p/flashheart/board?t=FH-36");
  const panel = page.getByRole("complementary", { name: "Ticket FH-36" });
  await panel.getByRole("tab", { name: "Edit" }).click();
  await panel
    .getByRole("textbox", { name: "Title" })
    .fill("Split API handlers by resource and verb");
  await panel.getByRole("combobox", { name: "Priority" }).selectOption("high");
  await expectNoAxeViolations("edit tab");
  await page.screenshot({
    path: resolve(screenshotDir, "edit-tab-1440-light.png"),
  });
  await panel.getByRole("button", { name: "Save changes" }).click();
  await expect(
    panel.getByRole("heading", {
      level: 2,
      name: "Split API handlers by resource and verb",
    }),
  ).toBeVisible();
  const saved = await readTicket("FH-36-split-api");
  expect(saved).toContain("# Split API handlers by resource and verb");
  expect(saved).toContain("priority: high");
  // The raw editor shows the version just saved.
  await panel.getByText("Raw file", { exact: true }).click();
  await expect(panel.getByRole("textbox", { name: "Ticket file" })).toHaveValue(
    /priority: high/,
  );
  await panel.getByText("Fields", { exact: true }).click();

  // Another writer changes the file while the form has unsaved edits; the
  // change arrives by live update before the save, and still conflicts.
  await panel
    .getByRole("textbox", { name: "Branch" })
    .fill("feature/split-api");
  await writeFile(
    ticketFile("FH-36-split-api"),
    saved.replace("priority: high", "priority: low"),
  );
  await expect(panel.getByText("low priority")).toBeVisible();
  await panel.getByRole("button", { name: "Save changes" }).click();
  const conflict = page.getByRole("dialog", {
    name: "FH-36 changed while you were editing",
  });
  await expect(conflict).toBeVisible();
  await expect(
    conflict.getByRole("region", { name: "Your version" }),
  ).toContainText("branch: feature/split-api");
  await expect(
    conflict.getByRole("region", { name: "Now on disk" }),
  ).toContainText("priority: low");
  await expectNoAxeViolations("conflict dialog");
  await page.screenshot({
    path: resolve(screenshotDir, "conflict-1440-light.png"),
  });
  await conflict.getByRole("button", { name: "Overwrite with mine" }).click();
  await expect(conflict).toHaveCount(0);
  await expect
    .poll(() => readTicket("FH-36-split-api"))
    .toContain("branch: feature/split-api");
  expect(await readTicket("FH-36-split-api")).toContain("priority: low");
});

test("an edit made in a text editor appears within a second", async () => {
  await open("#/p/flashheart/board");
  const name = ticketFile("FH-28-hook-latency");
  const data = await readFile(name, "utf8");
  await writeFile(
    name,
    data.replace(
      "# Measure hook latency on a warm cache",
      "# Measure hook latency, edited outside",
    ),
  );
  await expect(card("Measure hook latency, edited outside")).toBeVisible({
    timeout: 1_500,
  });
});

test("a screenshot linked in a text editor is copied into the ticket", async () => {
  await open("#/p/flashheart/board");
  // Serve copies only from a checkout of the project's repository (SEC-6),
  // so the project records one.
  const repo = join(await realpath(sandbox.home), "src", "flashheart");
  await mkdir(repo, { recursive: true });
  execFileSync("git", ["init", "-q"], { cwd: repo });
  const project = join(sandbox.root, "flashheart", "project.yaml");
  await writeFile(
    project,
    (await readFile(project, "utf8")).replace(
      "/Users/example/src/flashheart",
      repo,
    ),
  );
  const shot = join(repo, "docs", "latency chart.png");
  await mkdir(dirname(shot), { recursive: true });
  // The first bytes of a PNG are enough: the copy is checked by type, not
  // decoded.
  await writeFile(shot, Buffer.from("89504e470d0a1a0a", "hex"));
  const name = ticketFile("FH-28-hook-latency");
  const data = await readFile(name, "utf8");
  await writeFile(name, `${data}\n![Latency chart](<${shot}>)\n`);
  await expect
    .poll(() => readFile(name, "utf8"), { timeout: 3_000 })
    .toMatch(/!\[Latency chart\]\(files\/[^)]*-latency-chart\.png\)/);
  await open("#/p/flashheart/board?t=FH-28");
  const panel = page.getByRole("complementary", { name: "Ticket FH-28" });
  await expect(panel.getByRole("tab", { name: "Attachments 1" })).toBeVisible();
});

test("New ticket creates the next id and opens it", async () => {
  await open("#/p/flashheart/board");
  await page.getByRole("button", { name: "New ticket", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New ticket" });
  await dialog
    .getByRole("textbox", { name: "Title" })
    .fill("Keyboard help overlay");
  await dialog.getByRole("combobox", { name: "Type" }).selectOption("feature");
  await dialog
    .getByRole("combobox", { name: "Column" })
    .selectOption("up-next");
  await dialog
    .getByRole("textbox", { name: "Acceptance criteria" })
    .fill("Opens with ?\nLists every shortcut");
  await expectNoAxeViolations("new ticket dialog");
  await page.screenshot({
    path: resolve(screenshotDir, "new-ticket-1440-light.png"),
  });
  await dialog.getByRole("button", { name: "Create ticket" }).click();
  const panel = page.getByRole("complementary", { name: "Ticket FH-51" });
  await expect(
    panel.getByRole("heading", { level: 2, name: "Keyboard help overlay" }),
  ).toBeVisible();
  await expect(
    column("Up next").getByRole("button", { name: /^Keyboard help overlay,/ }),
  ).toBeVisible();
  const created = await readTicket("FH-51-keyboard-help-overlay");
  expect(created).toContain("- [ ] Lists every shortcut");
});

test("archiving hides a ticket and Undo brings it back", async () => {
  await open("#/p/flashheart/board?t=FH-51");
  const panel = page.getByRole("complementary", { name: "Ticket FH-51" });
  await panel.getByRole("button", { name: "Archive" }).click();
  await expect(card("Keyboard help overlay")).toHaveCount(0);
  const toast = page.getByRole("status").filter({ hasText: "Archived FH-51." });
  await toast.getByRole("button", { name: "Undo" }).click();
  await expect(card("Keyboard help overlay")).toBeVisible();
});

test("the archive restores tickets and deletes them for good", async () => {
  await open("#/p/alpha/board?t=AL-4");
  await page
    .getByRole("complementary", { name: "Ticket AL-4" })
    .getByRole("button", { name: "Archive" })
    .click();
  await expect(card("Drag and drop")).toHaveCount(0);
  await page.getByRole("link", { name: /^Archive/ }).click();
  await expect(page).toHaveURL(/#\/p\/alpha\/archive/);
  await page
    .getByRole("searchbox", { name: "Search archived tickets" })
    .fill("drag");
  const rows = page.getByRole("table", { name: "Archived tickets" });
  await expect(rows.getByRole("row")).toHaveCount(2);
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(`archive ${theme}`);
    await page.screenshot({
      path: resolve(screenshotDir, `archive-1440-${theme}.png`),
    });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });

  // Restore returns it to the column it left.
  await rows.getByRole("button", { name: "Restore AL-4" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "Restored AL-4 to Up next." }),
  ).toBeVisible();
  await expect(rows).toHaveCount(0);
  await page.getByRole("button", { name: "Back to the board" }).click();
  await expect(
    column("Up next").getByRole("button", { name: /^Drag and drop,/ }),
  ).toBeVisible();

  // Archive again, then delete it permanently: the dialog lists what it
  // touches and waits for the typed id.
  await open("#/p/alpha/board?t=AL-4");
  await page
    .getByRole("complementary", { name: "Ticket AL-4" })
    .getByRole("button", { name: "Archive" })
    .click();
  await open("#/p/alpha/archive");
  await page.getByRole("button", { name: "Delete permanently AL-4" }).click();
  const dialog = page.getByRole("dialog", {
    name: "Delete AL-4 permanently?",
  });
  await expect(dialog).toContainText("AL-4-drag-and-drop.md");
  await expect(dialog).toContainText("AL-5");
  await expect(dialog).toContainText("cannot be undone");
  const confirm = dialog.getByRole("button", { name: "Delete permanently" });
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel("Type AL-4 to confirm").fill("AL-40");
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel("Type AL-4 to confirm").fill("AL-4");
  await expect(confirm).toBeEnabled();
  await expectNoAxeViolations("delete dialog");
  await page.screenshot({
    path: resolve(screenshotDir, "delete-ticket-1440-light.png"),
  });
  await confirm.click();
  await expect(
    page.getByRole("status").filter({ hasText: "Deleted AL-4 permanently." }),
  ).toBeVisible();
  await expect(
    page.getByRole("table", { name: "Archived tickets" }).getByText("AL-4"),
  ).toHaveCount(0);

  // The ticket that depended on it no longer waits.
  await open("#/p/alpha/board");
  await expect(card("Long titles overflow the column")).toBeVisible();
  // The card's button covers its drawing, which sits beside it.
  await expect(
    card("Long titles overflow the column").locator("xpath=.."),
  ).not.toContainText("AL-4");
});

test("projects archive, restore and delete for good", async () => {
  await open("#/all/board");
  await page.getByRole("link", { name: /^Archive/ }).click();
  await expect(page).toHaveURL(/#\/all\/archive/);
  await page.getByRole("button", { name: "Archive beta" }).click();
  const archiveDialog = page.getByRole("dialog", { name: "Archive beta?" });
  await expect(archiveDialog).toContainText("1 ticket");
  await expectNoAxeViolations("archive project dialog");
  await archiveDialog.getByRole("button", { name: "Archive project" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "Archived project beta." }),
  ).toBeVisible();
  const rail = page.getByRole("navigation", { name: "Projects" });
  await expect(rail.getByRole("button", { name: /beta/ })).toHaveCount(0);
  const archived = page.getByRole("table", { name: "Archived projects" });
  await expect(archived).toContainText("beta");
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await expectNoAxeViolations(`projects archive ${theme}`);
    await page.screenshot({
      path: resolve(screenshotDir, `projects-archive-1440-${theme}.png`),
    });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });

  // Restore puts it back on the board.
  await archived.getByRole("button", { name: "Restore beta" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "Restored project beta." }),
  ).toBeVisible();
  await expect(rail.getByRole("button", { name: /beta/ })).toBeVisible();

  // Archive alpha from its own archive; its tickets no longer block beta.
  await open("#/p/alpha/archive");
  await page.getByRole("button", { name: "Archive project…" }).click();
  await page
    .getByRole("dialog", { name: "Archive Alpha?" })
    .getByRole("button", { name: "Archive project" })
    .click();
  await expect(page).toHaveURL(/#\/all\/archive/);
  await open("#/p/beta/board");
  await expect(card("Hello")).not.toHaveAccessibleName(/blocked/);

  // Delete it permanently: the dialog lists beta's dependent ticket and
  // waits for the typed name.
  await open("#/all/archive");
  await page.getByRole("button", { name: "Delete permanently Alpha" }).click();
  const deleteDialog = page.getByRole("dialog", {
    name: "Delete Alpha permanently?",
  });
  await expect(deleteDialog).toContainText("BE-1");
  const confirm = deleteDialog.getByRole("button", {
    name: "Delete permanently",
  });
  await deleteDialog.getByLabel("Type alpha to confirm").fill("Alpha");
  await expect(confirm).toBeDisabled();
  await deleteDialog.getByLabel("Type alpha to confirm").fill("alpha");
  await expect(confirm).toBeEnabled();
  await expectNoAxeViolations("delete project dialog");
  await page.screenshot({
    path: resolve(screenshotDir, "delete-project-1440-light.png"),
  });
  await confirm.click();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "Deleted project Alpha permanently." }),
  ).toBeVisible();
  await expect(page.getByText("No archived projects")).toBeVisible();
});

// remarksIn reads the reviewed remarks with ids in [from, to].
async function remarksIn(from, to) {
  const text = await readFile(
    resolve(import.meta.dirname, "../src/model/remarks.md"),
    "utf8",
  );
  return [...text.matchAll(/^(\d+)\. (.+)$/gm)]
    .filter(([, id]) => Number(id) >= from && Number(id) <= to)
    .map(([, , line]) => line.replaceAll("’", "'"));
}

test("finishing a workstream earns a remark; progress remarks are rare", async () => {
  // A one-ticket workstream around FH-32 (in Backlog).
  const folder = (
    await readdir(join(sandbox.root, "flashheart", "tickets"))
  ).find((name) => name.startsWith("FH-32-"));
  const file = join(
    sandbox.root,
    "flashheart",
    "tickets",
    folder,
    `${folder}.md`,
  );
  const ticket = await readFile(file, "utf8");
  await writeFile(file, ticket.replace(/^workstream:.*$/m, "workstream: solo"));
  await writeFile(
    join(sandbox.root, "flashheart", "workstreams", "solo.md"),
    "---\nslug: solo\nstatus: active\npriority: low\ncreated: 2026-10-06\ntickets:\n  - FH-32\ndepends-on-workstreams: []\ntags: []\n---\n\n# Solo\n",
  );
  // The board loads afresh after the Workstreams view has the new line.
  await open("#/p/flashheart/workstreams");
  await expect(page.getByRole("article", { name: "Solo" })).toBeVisible();
  const toast = page.getByRole("status").filter({ hasText: /^Moved/ });
  const completion = await remarksIn(61, 70);
  const progress = await remarksIn(51, 60);
  const moveToDone = async (id) => {
    await open(`#/p/flashheart/board?t=${id}`);
    await page
      .getByRole("complementary", { name: `Ticket ${id}` })
      .getByLabel("Move to")
      .selectOption("done");
    await expect(toast).toContainText(`Moved ${id} to Done.`);
  };

  await moveToDone("FH-32");
  const aside = toast.locator("[data-aside]");
  await expect(aside).toHaveCount(1);
  expect(completion).toContain(
    (await aside.textContent()).replaceAll("’", "'"),
  );

  // An ordinary move to Done: a progress remark, then none for a while.
  await moveToDone("FH-33");
  await expect(aside).toHaveCount(1);
  expect(progress).toContain((await aside.textContent()).replaceAll("’", "'"));
  await moveToDone("FH-36");
  await expect(aside).toHaveCount(0);

  // A fresh load brings no toast and no celebration.
  await page.reload();
  await open("#/p/flashheart/board");
  await expect(toast).toHaveCount(0);
});

test("independent stations reorder with Shift and an arrow", async () => {
  await open("#/p/ngplus/workstreams");
  const line = page.getByRole("article", { name: "Study UI" });
  await line
    .getByRole("button", { name: /^Flashcards from past papers,/ })
    .focus();
  await page.keyboard.press("Shift+ArrowLeft");
  const stations = line
    .getByRole("list", {
      name: "Study UI stations with no dependencies on the line",
    })
    .getByRole("listitem");
  await expect(stations.first()).toContainText("Flashcards from past papers");
  await expect(
    line.getByRole("button", { name: /^Flashcards from past papers,/ }),
  ).toBeFocused();
  await expect
    .poll(() =>
      readFile(
        join(sandbox.root, "ngplus", "workstreams", "study-ui.md"),
        "utf8",
      ),
    )
    .toContain("tickets:\n  - NG-5\n  - NG-4\n");
});

test("preferences are saved through the backend and survive a reload", async () => {
  await open("#/p/flashheart/board");
  await viewOption(page, "Colour by", "Priority");
  const types = await filterMenu(page, "Type");
  await types.getByRole("button", { name: "bug", exact: true }).click();
  await page.keyboard.press("Escape");
  await expect
    .poll(() =>
      readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
    )
    .toContain("colour_by: priority");
  await page.reload();
  await expect
    .poll(() =>
      readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
    )
    .toMatch(/type:\n\s+include: \[bug\]/);
  await page.getByRole("button", { name: "View options" }).click();
  await expect(
    page
      .getByRole("group", { name: "Colour by" })
      .getByRole("radio", { name: "Priority" }),
  ).toBeChecked();
  await page.keyboard.press("Escape");
  await expect(filterButton(page, "Type")).toHaveAccessibleName(
    "Type: 1 chosen",
  );
  await page.getByRole("button", { name: "Clear filters" }).click();
});

test("the theme choice is saved", async () => {
  await open("#/p/flashheart/board");
  const theme = page.getByRole("group", { name: "Theme" });
  await theme.getByText("Dark", { exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect
    .poll(() =>
      readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
    )
    .toContain("theme: dark");
  await theme.getByText("System", { exact: true }).click();
  await expect
    .poll(() =>
      readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
    )
    .toContain("theme: system");
});

test("the Backlog column can be hidden from View options (FH-41)", async () => {
  const config = () =>
    readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8");
  const backlogBox = page.getByRole("checkbox", { name: "Backlog" });
  await open("#/p/flashheart/board");
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await page.getByRole("button", { name: "View options" }).click();
    await expect(backlogBox).toBeChecked();
    // Needs you is a chip now, not a column to show (FH-44).
    await expect(page.getByRole("checkbox", { name: "Needs you" })).toHaveCount(
      0,
    );
    await page.screenshot({
      path: resolve(screenshotDir, `view-options-backlog-1440-${theme}.png`),
    });
    await page.keyboard.press("Escape");
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await page.getByRole("button", { name: "View options" }).click();
  await backlogBox.uncheck();
  await page.keyboard.press("Escape");
  await expect(column("Backlog")).toHaveCount(0);
  await expect.poll(config).toContain("hidden_columns: [backlog]");

  // Shift with Left from Up next has no column to go to.
  const first = column("Up next").getByRole("button").first();
  const name = await first.getAttribute("aria-label");
  await first.focus();
  await page.keyboard.press("Shift+ArrowLeft");
  await expect(
    column("Up next").getByRole("button", { name, exact: true }),
  ).toBeFocused();
  await expect(
    page.getByRole("status").filter({ hasText: "Moved" }),
  ).toHaveCount(0);

  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await page.locator("h1").click();
    await page.screenshot({
      path: resolve(screenshotDir, `backlog-hidden-1440-${theme}.png`),
    });
  }

  // The narrow Column picker leaves it out too.
  await page.setViewportSize({ width: 390, height: 844 });
  const picker = page.getByRole("combobox", { name: "Column" });
  await expect(picker.locator("option")).toHaveText([
    "Up next",
    "In progress",
    "Ready to review",
    "Done",
  ]);
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await page.screenshot({
      path: resolve(screenshotDir, `backlog-hidden-390-${theme}.png`),
    });
  }
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await page.setViewportSize({ width: 1440, height: 900 });

  // The choice survives a reload, and showing it again saves that too.
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Backend connected. Check connection" }),
  ).toBeVisible();
  await expect(column("Up next")).toBeVisible();
  await expect(column("Backlog")).toHaveCount(0);
  await page.getByRole("button", { name: "View options" }).click();
  await expect(backlogBox).not.toBeChecked();
  await backlogBox.check();
  await page.keyboard.press("Escape");
  await expect(column("Backlog")).toBeVisible();
  await expect.poll(config).toContain("hidden_columns: []");
});

test("switching theme keeps the view, filters, open ticket and scroll", async () => {
  await open("#/p/flashheart/table?t=FH-36");
  const priorities = await filterMenu(page, "Priority");
  await priorities.getByRole("button", { name: "High", exact: true }).click();
  await page.keyboard.press("Escape");
  const scroller = page.locator("main .overflow-auto").first();
  await scroller.evaluate((element) => {
    element.scrollTop = 40;
  });
  const before = await scroller.evaluate((element) => element.scrollTop);
  const url = page.url();
  // Both palettes share one structure: the same headings in the same order.
  const outline = () =>
    page
      .getByRole("heading")
      .evaluateAll((nodes) => nodes.map((node) => node.textContent));
  const lightOutline = await outline();
  const theme = page.getByRole("group", { name: "Theme" });
  await theme.getByText("Dark", { exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  expect(page.url()).toBe(url);
  await expect(filterButton(page, "Priority")).toHaveAccessibleName(
    "Priority: 1 chosen",
  );
  await expect(
    page.getByRole("complementary", { name: "Ticket FH-36" }),
  ).toBeVisible();
  expect(await scroller.evaluate((element) => element.scrollTop)).toBe(before);
  expect(await outline()).toEqual(lightOutline);
  await theme.getByText("System", { exact: true }).click();
  await page.getByRole("button", { name: "Clear filters" }).click();
});

test("the Edit tab and dialogs are accessible in dark", async () => {
  await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
  await open("#/p/flashheart/board?t=FH-36");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  const panel = page.getByRole("complementary", { name: "Ticket FH-36" });
  await panel.getByRole("tab", { name: "Edit" }).click();
  await expectNoAxeViolations("edit tab dark");
  await page.screenshot({
    path: resolve(screenshotDir, "edit-tab-1440-dark.png"),
  });
  await page.getByRole("button", { name: "New ticket", exact: true }).click();
  await expectNoAxeViolations("new ticket dark");
  await page.screenshot({
    path: resolve(screenshotDir, "new-ticket-1440-dark.png"),
  });
  // Escape closes the dialog only; the panel under it stays open.
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(panel).toBeVisible();
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
});
