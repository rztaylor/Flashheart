import { readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  launch,
  makeSandbox,
  screenshotDir,
  stopIfRunning,
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
  await page.mouse.move(target.x + target.width / 2, target.y + 200, {
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

test("starting a blocked ticket asks for a reason and records it", async () => {
  await open("#/p/flashheart/board");
  await card("Workstreams as transit lines").focus();
  await page.keyboard.press("Shift+ArrowRight");
  const dialog = page.getByRole("dialog", {
    name: "Start FH-12 while it is blocked?",
  });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("Comes after FH-10");
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

test("stations reorder along their line with Shift and an arrow", async () => {
  await open("#/p/flashheart/workstreams");
  const line = page.getByRole("article", { name: "Board editing" });
  await line
    .getByRole("button", { name: /^Drag cards between columns,/ })
    .focus();
  await page.keyboard.press("Shift+ArrowLeft");
  const stations = line
    .getByRole("list", { name: "Board editing stations" })
    .getByRole("listitem");
  await expect(stations.first()).toContainText("Drag cards between columns");
  await expect(
    line.getByRole("button", { name: /^Drag cards between columns,/ }),
  ).toBeFocused();
  await expect
    .poll(() =>
      readFile(
        join(sandbox.root, "flashheart", "workstreams", "board-editing.md"),
        "utf8",
      ),
    )
    .toContain("tickets:\n  - FH-15\n  - FH-14\n");
});

test("preferences are saved through the backend and survive a reload", async () => {
  await open("#/p/flashheart/board");
  await page
    .getByRole("combobox", { name: "Colour by" })
    .selectOption({ label: "Priority" });
  await page.getByRole("combobox", { name: "Type" }).selectOption("bug");
  await expect
    .poll(() =>
      readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
    )
    .toContain("colour_by: priority");
  await page.reload();
  await expect(page.getByRole("combobox", { name: "Colour by" })).toHaveValue(
    "priority",
  );
  await expect(page.getByRole("combobox", { name: "Type" })).toHaveValue("bug");
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
