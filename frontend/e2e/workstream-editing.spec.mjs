import { readdir, readFile } from "node:fs/promises";
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

// Workstreams made and filled on the board (FH-17): New workstream in the
// Workstreams view writes board-format's template and opens its page; New
// ticket opens the ticket it created ready to edit; the panel's workstream
// picker moves a ticket into a workstream and out again, changing the
// ticket and the workstream together. One server over a writable demo
// board; tests run in order and each leaves the board as the next expects.
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

const workstreamFile = (slug) =>
  join(sandbox.root, "flashheart", "workstreams", `${slug}.md`);
const readWorkstream = (slug) => readFile(workstreamFile(slug), "utf8");
async function readTicket(id) {
  const tickets = join(sandbox.root, "flashheart", "tickets");
  const folder = (await readdir(tickets)).find((name) =>
    name.startsWith(`${id}-`),
  );
  return readFile(join(tickets, folder, `${folder}.md`), "utf8");
}
// members is a workstream file's tickets list, in flow or block style.
function members(text) {
  const flow = text.match(/^tickets: \[(.*)\]$/m);
  if (flow)
    return flow[1]
      .split(",")
      .map((id) => id.trim())
      .filter(Boolean);
  const block = text.match(/^tickets:\n((?: {2}- .*\n)+)/m)?.[1] ?? "";
  return [...block.matchAll(/ {2}- (.*)\n/g)].map((match) => match[1]);
}

const newWorkstream = () =>
  page.getByRole("button", { name: "New workstream", exact: true });
const dialog = () => page.getByRole("dialog", { name: "New workstream" });
// routeCard is workstream-editing's route card; a second of the same name
// comes later.
const routeCard = () =>
  page.locator('article[aria-labelledby="ws-flashheart-workstream-editing"]');

let created = "";

test("New workstream writes the template and opens its page", async () => {
  await open(page, "#/p/flashheart/workstreams");
  await newWorkstream().click();
  const title = dialog().getByRole("textbox", { name: "Title" });
  await expect(title).toBeFocused();
  await expect(
    dialog().getByRole("button", { name: "Create workstream" }),
  ).toBeDisabled();
  await title.fill("Workstream editing");
  await dialog()
    .getByRole("textbox", { name: "Goal" })
    .fill("Make and fill workstreams on the board.");
  await dialog()
    .getByRole("combobox", { name: "Priority" })
    .selectOption("high");
  await dialog()
    .getByRole("group", { name: "Depends on workstreams" })
    .getByRole("checkbox", { name: "Board core" })
    .check();
  await expectNoAxeViolations(page, "new workstream dialog");
  await shot(page, "new-workstream-1440-light", { keepFocus: true });
  // Enter in the title submits, as in New ticket.
  await title.press("Enter");

  await expect(page).toHaveURL(
    /#\/workstream\/flashheart\/workstream-editing$/,
  );
  await expect(page.getByRole("dialog")).toHaveCount(0);
  const main = page.getByRole("main", {
    name: "Workstream workstream-editing",
  });
  await expect(
    main.getByRole("heading", { level: 1, name: "Workstream editing" }),
  ).toBeVisible();
  await expect(main.getByRole("region", { name: "Goal" })).toContainText(
    "Make and fill workstreams on the board.",
  );
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "Created workstream Workstream editing." }),
  ).toBeVisible();

  const text = await readWorkstream("workstream-editing");
  expect(text).toMatch(
    /^---\nslug: workstream-editing\nstatus: active\npriority: high\ncreated: \d{4}-\d{2}-\d{2}\ntickets: \[\]\ndepends-on-workstreams: \[board-core\]\ntags: \[\]\n---\n\n# Workstream editing\n\n## Goal\n\nMake and fill workstreams on the board.\n\n## Scope\n\n## Success Criteria\n/,
  );

  // Back in the view, its route card waits for tickets.
  await open(page, "#/p/flashheart/workstreams");
  await expect(routeCard()).toContainText("No tickets on this line yet.");
});

test("the same title takes a fresh slug", async () => {
  await newWorkstream().click();
  await dialog()
    .getByRole("textbox", { name: "Title" })
    .fill("Workstream editing");
  await dialog().getByRole("button", { name: "Create workstream" }).click();
  await expect(page).toHaveURL(
    /#\/workstream\/flashheart\/workstream-editing-2$/,
  );
  expect(await readWorkstream("workstream-editing-2")).toContain(
    "priority: medium\n",
  );
});

test("a refusal shows in words and keeps the dialog", async () => {
  await open(page, "#/p/flashheart/workstreams");
  await page.route("**/api/projects/flashheart/workstreams", (route) =>
    route.request().method() === "POST"
      ? route.fulfill({
          status: 404,
          contentType: "application/json",
          body: JSON.stringify({
            error: {
              code: "not_found",
              message:
                'workstream "board-core" is not in flashheart; its workstreams: foundation',
            },
          }),
        })
      : route.fallback(),
  );
  await newWorkstream().click();
  await dialog().getByRole("textbox", { name: "Title" }).fill("Refused");
  await dialog().getByRole("button", { name: "Create workstream" }).click();
  await expect(dialog().getByRole("alert")).toHaveText(
    'workstream "board-core" is not in flashheart; its workstreams: foundation',
  );
  await expect(dialog().getByRole("textbox", { name: "Title" })).toHaveValue(
    "Refused",
  );
  await page.unroute("**/api/projects/flashheart/workstreams");
  // Escape closes it and focus returns to New workstream.
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(newWorkstream()).toBeFocused();
});

test("the dialog in dark and at 390 px passes axe", async () => {
  for (const [width, height, theme] of [
    [1440, 900, "dark"],
    [390, 844, "light"],
    [390, 844, "dark"],
  ]) {
    await open(page, "#/p/flashheart/workstreams", { width, height, theme });
    await newWorkstream().click();
    await expect(
      dialog().getByRole("textbox", { name: "Title" }),
    ).toBeFocused();
    await expectNoAxeViolations(page, `new workstream ${width} ${theme}`);
    // Nothing in the dialog is wider than the screen.
    const box = await dialog().boundingBox();
    expect(box && box.x >= 0 && box.x + box.width <= width).toBe(true);
    await shot(page, `new-workstream-${width}-${theme}`, { keepFocus: true });
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  }
});

test("New ticket opens the ticket it created ready to edit", async () => {
  await open(page, "#/p/flashheart/board");
  await page.getByRole("button", { name: "New ticket", exact: true }).click();
  const newTicket = page.getByRole("dialog", { name: "New ticket" });
  await newTicket
    .getByRole("textbox", { name: "Title" })
    .fill("Workstream picker on the panel");
  await newTicket.getByRole("button", { name: "Create ticket" }).click();
  const panel = page.getByRole("complementary", { name: /^Ticket FH-\d+$/ });
  await expect(panel).toBeVisible();
  created = (await panel.getAttribute("aria-label")).replace("Ticket ", "");
  await expect(panel.getByRole("tab", { name: "Edit" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  const title = panel.getByRole("textbox", { name: "Title" });
  await expect(title).toBeFocused();
  await expect(title).toHaveValue("Workstream picker on the panel");
  await expectNoAxeViolations(page, "new ticket panel");
  await shot(page, "new-ticket-panel-1440-light", { keepFocus: true });
});

test("the panel's workstream picker adds the ticket and takes it out", async () => {
  await open(page, `#/p/flashheart/board?t=${created}`);
  const panel = page.getByRole("complementary", { name: `Ticket ${created}` });
  await panel.getByRole("tab", { name: "Edit" }).click();
  const picker = panel.getByRole("combobox", { name: "Workstream" });
  // By slug: the second workstream of the same name is listed too.
  await picker.selectOption("workstream-editing");
  await shot(page, "workstream-picker-1440-light", { keepFocus: true });
  await panel.getByRole("button", { name: "Save changes" }).click();
  await expect
    .poll(async () => members(await readWorkstream("workstream-editing")))
    .toEqual([created]);
  expect(await readTicket(created)).toMatch(
    /^workstream: workstream-editing$/m,
  );
  await expect(panel.locator("header")).toContainText("Workstream editing");

  // Its station is on the line.
  await open(page, "#/p/flashheart/workstreams");
  await expect(routeCard()).toContainText("Workstream picker on the panel");

  // None takes it out of the list and clears the field, together.
  await open(page, `#/p/flashheart/workstreams?t=${created}`);
  const again = page.getByRole("complementary", { name: `Ticket ${created}` });
  await again.getByRole("tab", { name: "Edit" }).click();
  await again
    .getByRole("combobox", { name: "Workstream" })
    .selectOption({ label: "None" });
  await again.getByRole("button", { name: "Save changes" }).click();
  await expect
    .poll(async () => members(await readWorkstream("workstream-editing")))
    .toEqual([]);
  expect(await readTicket(created)).toMatch(/^workstream: *$/m);
  await expect(routeCard()).toContainText("No tickets on this line yet.");
  for (const theme of ["light", "dark"]) {
    await open(page, `#/p/flashheart/workstreams?t=${created}`, { theme });
    await page
      .getByRole("complementary", { name: `Ticket ${created}` })
      .getByRole("tab", { name: "Edit" })
      .click();
    await expectNoAxeViolations(page, `workstream picker ${theme}`);
  }
});
