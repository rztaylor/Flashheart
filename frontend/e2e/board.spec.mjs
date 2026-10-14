import { join, resolve } from "node:path";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  card,
  column,
  expectNoAxeViolations,
  filterButton,
  filterMenu,
  launch,
  makeSandbox,
  open,
  screenshotDir,
  shot,
  stopIfRunning,
  viewOption,
  waitForManualURL,
} from "./support.mjs";

// One server and one authenticated page for the whole file: the board is
// read-only in board-core, and a bootstrap URL can be used only once.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let shared;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext();
  shared = await context.newPage();
  await shared.goto(url);
  await shared.waitForURL((current) => current.pathname === "/");
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

async function colourBy(page, option) {
  await viewOption(page, "Colour by", option);
}

// cardFace is what a card shows: its button covers the drawing, which sits
// beside it in the card (CARD-7).
function cardFace(page, title) {
  return card(page, title).locator("xpath=..");
}

test("sample tickets land in the right columns with blocked reasons and repairs", async () => {
  const page = shared;
  await open(page, "#/p/alpha/board");
  await expect(
    page.locator("header").getByText("Alpha", { exact: true }),
  ).toBeVisible();

  const placement = {
    Backlog: [
      "Long titles overflow the column",
      "Offline mode",
      "Broken frontmatter",
    ],
    "Up next": ["Drag and drop"],
    "In progress": ["Card panel"],
    "Ready to review": ["Board columns"],
    Done: ["Project skeleton"],
  };
  for (const [name, titles] of Object.entries(placement)) {
    for (const title of titles) {
      await expect(
        column(page, name).getByRole("button", {
          name: new RegExp(`^${title},`),
        }),
      ).toBeVisible();
    }
  }
  // A Normal card marks a blocker; its reason is in the mark's tooltip and
  // the card's name, and in words on a Detailed card (FH-147).
  for (const [title, reason] of [
    ["Drag and drop", "Depends on AL-3, which is In progress"],
    ["Long titles overflow the column", "Depends on AL-4, which is Up next"],
    ["Offline mode", "Depends on feat--does-not-exist, which does not exist"],
  ]) {
    await expect(card(page, title)).toHaveAccessibleName(
      new RegExp(`blocked: ${reason}`),
    );
    await expect(
      cardFace(page, title).locator('[data-mark="blocked"]'),
    ).toHaveAttribute("title", `Blocked: ${reason}`);
  }
  await viewOption(page, "Density", "Detailed");
  await expect(cardFace(page, "Drag and drop")).toContainText(
    "Depends on AL-3, which is In progress",
  );
  await viewOption(page, "Density", "Normal");
  await expect(card(page, "Broken frontmatter")).toHaveAccessibleName(
    /needs repair/,
  );
  await expect(cardFace(page, "Broken frontmatter")).toContainText(
    "frontmatter does not parse",
  );
  await expect(cardFace(page, "Card panel")).not.toContainText("Depends on");
});

// Blocked has one symbol, the no-entry sign, wherever it is marked
// (FH-148, ui-layout.md §7); the hollow diamond is gone.
test("blocked is marked with the no-entry sign in every view (FH-148)", async () => {
  const page = shared;
  const noEntry = '[data-icon="blocked"]';
  const save = (locator, name) =>
    locator.screenshot({ path: resolve(screenshotDir, `${name}.png`) });
  const noDiamond = () =>
    expect(page.locator('[data-icon="diamond"]')).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    const wide = { width: 1440, height: 900, theme };
    await open(page, "#/p/alpha/board?t=AL-5", wide);
    const face = cardFace(page, "Long titles overflow the column");
    await expect(face.locator(noEntry)).toBeVisible();
    const panel = page.getByRole("complementary", { name: "Ticket AL-5" });
    await expect(
      panel.getByRole("heading", { name: "Blocked by" }).locator(noEntry),
    ).toBeVisible();
    await expect(panel.locator(noEntry).first()).toBeVisible();
    await noDiamond();
    await expectNoAxeViolations(page, `blocked board and panel ${theme}`);
    await page.locator("h1").click();
    await shot(page, `blocked-board-panel-1440-${theme}`);
    await save(face, `blocked-card-${theme}`);

    await open(page, "#/p/alpha/board", wide);
    await viewOption(page, "Density", "Compact");
    await expect(face.locator(noEntry)).toBeVisible();
    await save(face, `blocked-card-compact-${theme}`);
    await viewOption(page, "Density", "Normal");

    const states = await filterMenu(page, "State");
    await expect(
      states
        .getByRole("button", { name: "Blocked", exact: true })
        .locator(noEntry),
    ).toBeVisible();
    await shot(page, `blocked-state-menu-${theme}`, { keepFocus: true });
    await states.getByRole("button", { name: "Blocked", exact: true }).click();
    await expect(filterButton(page, "State").locator(noEntry)).toBeVisible();
    await page.getByRole("button", { name: "Clear filters" }).click();

    await open(page, "#/ticket/AL-5", wide);
    const blockedBy = page.getByRole("heading", { name: "Blocked by" });
    await expect(blockedBy.locator(noEntry)).toBeVisible();
    await noDiamond();
    await expectNoAxeViolations(page, `blocked full page ${theme}`);
    await shot(page, `blocked-ticket-page-1440-${theme}`);

    await open(page, "#/p/flashheart/workstreams", wide);
    const editing = page.getByRole("article", { name: "Board editing" });
    await expect(editing.getByText("Blocked", { exact: true })).toBeVisible();
    await expect(editing.locator(noEntry).first()).toBeVisible();
    await noDiamond();
    await expectNoAxeViolations(page, `blocked workstreams ${theme}`);
    await shot(page, `blocked-workstreams-1440-${theme}`);

    await open(page, "#/p/alpha/table", wide);
    await expect(
      page.getByRole("table").locator(noEntry).first(),
    ).toBeVisible();
    await noDiamond();
    await shot(page, `blocked-table-1440-${theme}`);

    const narrow = { width: 390, height: 844, theme };
    await open(page, "#/p/alpha/board", narrow);
    await page
      .getByRole("combobox", { name: "Column" })
      .selectOption("backlog");
    await expect(face.locator(noEntry)).toBeVisible();
    await shot(page, `blocked-board-narrow-${theme}`);
    await open(page, "#/p/flashheart/workstreams", narrow);
    await shot(page, `blocked-workstreams-narrow-${theme}`);
  }
});

// busy makes AL-5 a ticket with every marker (FH-147): blocked already, a
// working run with a plan, change requests, attachments and a review. The
// board's response is changed in the browser; the server is the real one.
// busy.needsYou turns the run to waiting on a permission for Bash.
async function routeBusyCard(page, busy = { needsYou: false }) {
  await page.route(
    (url) => url.pathname === "/api/projects/alpha/board",
    async (route) => {
      const response = await route.fetch();
      const json = await response.json();
      for (const card of json.cards) {
        if (card.id !== "AL-5") continue;
        card.live = {
          run: "claude:busy0001",
          short: "claude:busy0001",
          agent: "claude",
          state: busy.needsYou ? "needs-you" : "working",
          done: 2,
          total: 5,
          step: "Write the drop test",
          reason: {
            kind: busy.needsYou ? "permission" : "",
            permission: busy.needsYou ? "Bash" : "",
            question: "",
            answered: false,
            waitingOn: "",
            agentType: "",
          },
          lastActivity: new Date().toISOString(),
        };
        card.changeRequests = { open: 2, addressed: 0 };
        card.attachments = 3;
        card.hasReview = true;
      }
      await route.fulfill({ response, json });
    },
  );
}

// metaLine lists the markers a card's meta line shows: those on its one
// line inside its bounds. The rest wrapped out of sight.
function metaLine(face) {
  return face.locator('[data-row="meta"]').evaluate((row) => {
    const line = row.firstElementChild.getBoundingClientRect();
    const shown = [];
    for (const item of row.querySelectorAll("[data-mark], a")) {
      const box = item.getBoundingClientRect();
      const inside =
        box.top < line.bottom - 1 &&
        box.left >= line.left - 0.5 &&
        box.right <= row.getBoundingClientRect().right + 0.5;
      if (inside) shown.push(item.dataset.mark ?? "id");
    }
    return { shown, height: row.getBoundingClientRect().height };
  });
}

// The three layouts of one set of markers (FH-147, ui-layout.md §2).
test("each density lays the card's markers out its own way (FH-147)", async () => {
  const page = shared;
  const busy = { needsYou: false };
  await routeBusyCard(page, busy);
  try {
    await open(page, "#/p/flashheart/board", {
      width: 1280,
      height: 900,
    });
    await open(page, "#/p/alpha/board", { width: 1280, height: 900 });
    const face = cardFace(page, "Long titles overflow the column");
    await expect(face.locator('[data-mark="live"]')).toHaveText("2/5");
    const column = await page
      .locator('section[data-column="backlog"]')
      .boundingBox();
    // Columns are at least 15rem; at 1280 they are no wider.
    expect(column?.width).toBeLessThanOrEqual(241);

    // Normal: one meta line that keeps the id and the markers that never
    // drop, and drops attachments, then review, then age.
    const never = ["type", "id", "blocked", "requests", "live", "priority"];
    const droppable = ["age", "review", "attachments"];
    const atWidth = async (width) => {
      await face.evaluate((node, w) => {
        node.style.width = w ? `${w}px` : "";
      }, width);
      return metaLine(face);
    };
    const natural = await atWidth(undefined);
    for (const mark of never) expect(natural.shown, mark).toContain(mark);
    expect(natural.height).toBeLessThanOrEqual(17);
    await shot(page, "card-busy-15rem-light");
    await face.screenshot({
      path: resolve(screenshotDir, "card-busy-15rem-card-light.png"),
    });
    // Hovering a marker shows its words; the card's button stays on top,
    // so a click on a marker still opens the card.
    const priority = await face.locator('[data-mark="priority"]').boundingBox();
    await page.mouse.move(
      (priority?.x ?? 0) + (priority?.width ?? 0) / 2,
      (priority?.y ?? 0) + (priority?.height ?? 0) / 2,
    );
    await expect(card(page, "Long titles overflow the column")).toHaveAttribute(
      "title",
      "Medium priority",
    );
    await page.mouse.move(0, 0);
    let kept = 3;
    for (const width of [240, 220, 200, 190, 180, 170, 160]) {
      const { shown, height } = await atWidth(width);
      expect(height, `${width}px`).toBeLessThanOrEqual(17);
      const left = droppable.filter((mark) => shown.includes(mark));
      // What stays is always the start of age, review, attachments, and
      // never grows as the card narrows.
      expect(left, `${width}px`).toEqual(droppable.slice(0, left.length));
      expect(left.length, `${width}px`).toBeLessThanOrEqual(kept);
      kept = left.length;
    }
    expect(kept, "everything droppable dropped by 160px").toBe(0);
    await atWidth(undefined);

    // The name still says every marker, dropped or not.
    await expect(
      card(page, "Long titles overflow the column"),
    ).toHaveAccessibleName(
      /blocked: .*Claude working, plan 2 of 5: Write the drop test.*2 changes requested.*bug.*3 attachments, review written, changed/,
    );

    for (const theme of ["light", "dark"]) {
      for (const density of ["Compact", "Normal", "Detailed"]) {
        await open(page, "#/p/alpha/board", {
          width: 1440,
          height: 900,
          theme,
        });
        await viewOption(page, "Density", density);
        await expect(
          face.locator('[data-mark="priority"]').first(),
        ).toBeVisible();
        await expectNoAxeViolations(page, `${density} cards ${theme}`);
        await shot(page, `card-density-${density.toLowerCase()}-1440-${theme}`);
        await face.screenshot({
          path: resolve(
            screenshotDir,
            `card-busy-${density.toLowerCase()}-${theme}.png`,
          ),
        });
        await open(page, "#/p/flashheart/board", {
          width: 1440,
          height: 900,
          theme,
        });
        await shot(
          page,
          `card-density-${density.toLowerCase()}-flashheart-1440-${theme}`,
        );
        await open(page, "#/p/alpha/board", { width: 390, height: 844, theme });
        await page
          .getByRole("combobox", { name: "Column" })
          .selectOption("backlog");
        await shot(
          page,
          `card-density-${density.toLowerCase()}-narrow-${theme}`,
        );
      }
    }
    // Compact has no other row; Detailed has the words.
    await open(page, "#/p/alpha/board", { width: 1440, height: 900 });
    await viewOption(page, "Density", "Compact");
    await expect(face.locator('[data-row="meta"]')).toHaveCount(0);
    await expect(face.locator('[data-part="title"]')).toHaveText(
      /^AL-5\s*Long titles overflow the column$/,
    );
    await viewOption(page, "Density", "Detailed");
    await expect(face.locator('[data-row="run"]')).toContainText(
      "2/5 · Write the drop test",
    );
    await expect(face).toContainText("Depends on AL-4, which is Up next");

    // A run that needs the user: its reason on a line of its own (Normal,
    // Detailed), the plate beside the title (Compact).
    busy.needsYou = true;
    for (const theme of ["light", "dark"]) {
      await open(page, "#/p/flashheart/board", { width: 1440, height: 900 });
      for (const density of ["Compact", "Normal", "Detailed"]) {
        await open(page, "#/p/alpha/board", {
          width: 1440,
          height: 900,
          theme,
        });
        await viewOption(page, "Density", density);
        if (density === "Compact")
          await expect(face.locator('[data-mark="needs-you"]')).toBeVisible();
        else
          await expect(face.locator('[data-row="needs-you"]')).toHaveText(
            /Needs you\s*Permission for Bash/,
          );
        await face.screenshot({
          path: resolve(
            screenshotDir,
            `card-needs-you-${density.toLowerCase()}-${theme}.png`,
          ),
        });
      }
    }
  } finally {
    await viewOption(page, "Density", "Normal");
    await page.unrouteAll({ behavior: "ignoreErrors" });
  }
});

test("the card panel explains, links and renders without raw HTML", async () => {
  const page = shared;
  await open(page, "#/p/alpha/board");
  await card(page, "Long titles overflow the column").click();
  const panel = page.getByRole("complementary", {
    name: "Ticket AL-5",
  });
  await expect(
    panel.getByRole("heading", {
      level: 2,
      name: "Long titles overflow the column",
    }),
  ).toBeVisible();
  await expect(
    panel.getByRole("heading", { name: "Blocked by" }),
  ).toBeVisible();
  await panel.getByRole("button", { name: "Open" }).click();
  await expect(
    page.getByRole("complementary", { name: "Ticket AL-4" }),
  ).toBeVisible();
  await expect(page).toHaveURL(/t=AL-4/);

  // Handoff shown prominently for the in-progress ticket.
  await card(page, "Card panel").click();
  const cardPanel = page.getByRole("complementary", {
    name: "Ticket AL-3",
  });
  await expect(
    cardPanel.getByRole("heading", { name: "Handoff" }),
  ).toBeVisible();
  await expect(
    cardPanel.getByText("Review tab", { exact: true }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(cardPanel).toHaveCount(0);
  await expect(card(page, "Card panel")).toBeFocused();

  // Review tab with a markdown ticket link and an attachment.
  await card(page, "Board columns").click();
  const review = page.getByRole("complementary", {
    name: "Ticket AL-2",
  });
  await review.getByRole("tab", { name: "Review" }).click();
  await expect(
    review.getByRole("img", { name: "Board at desktop" }),
  ).toBeVisible();
  // A ticket id in markdown opens that ticket's full page in a new tab.
  const own = review
    .getByRole("tabpanel", { name: "Review" })
    .getByRole("link", { name: "AL-2", exact: true });
  await expect(own).toHaveAttribute("href", "#/ticket/AL-2");
  await expect(own).toHaveAttribute("target", "_blank");
  await page.keyboard.press("Escape");

  // External links open in a new tab without opener or referrer; raw HTML never renders.
  await open(page, "#/p/flashheart/board?t=FH-8");
  const storePanel = page.getByRole("complementary", {
    name: "Ticket FH-8",
  });
  await storePanel.getByRole("tab", { name: "Review" }).click();
  await expect(
    storePanel.getByRole("img", { name: "Store tests passing" }).first(),
  ).toBeVisible();
  const external = storePanel.getByRole("link", {
    name: "os.Root documentation",
  });
  await expect(external).toHaveAttribute("href", "https://pkg.go.dev/os#Root");
  await expect(external).toHaveAttribute("target", "_blank");
  await expect(external).toHaveAttribute("rel", "noopener noreferrer");
  expect(await page.locator(".markdown script, .markdown iframe").count()).toBe(
    0,
  );
  expect(context.pages()).toHaveLength(1);

  // A bare ticket id in markdown links to that ticket's full page (KEY-3).
  const [linked] = await Promise.all([
    context.waitForEvent("page"),
    storePanel.getByRole("link", { name: "FH-26", exact: true }).click(),
  ]);
  await expect(linked).toHaveURL(/#\/ticket\/FH-26$/);
  await expect(
    linked.getByRole("main", { name: "Ticket FH-26" }),
  ).toBeVisible();
  await linked.close();
});

test("a ticket id opens the ticket's full page in a new tab (CARD-7)", async () => {
  const page = shared;
  await open(page, "#/p/alpha/board");
  // The id sits above the card's button, hidden from assistive technology
  // (the panel header offers the same link).
  const id = cardFace(page, "Long titles overflow the column").locator(
    'a[href="#/ticket/AL-5"]',
  );
  await expect(id).toHaveAttribute("href", "#/ticket/AL-5");
  const [full] = await Promise.all([context.waitForEvent("page"), id.click()]);
  // The board stays as it was: the id does not open the panel.
  await expect(page.getByRole("complementary")).toHaveCount(0);

  const main = full.getByRole("main", { name: "Ticket AL-5" });
  await expect(
    main.getByRole("heading", {
      level: 1,
      name: "Long titles overflow the column",
    }),
  ).toBeVisible();
  await expect(full).toHaveTitle(/^AL-5 · Long titles overflow the column/);
  // One continuous page with an outline, not the panel's tabs (FH-65).
  await expect(main.getByRole("tab")).toHaveCount(0);
  await expect(
    main.getByRole("navigation", { name: "Sections" }),
  ).toBeVisible();
  // Ticket links on the full page stay in its tab.
  const blocker = main
    .getByRole("listitem")
    .filter({ hasText: "AL-4" })
    .getByRole("link", { name: "AL-4", exact: true });
  await expect(blocker).not.toHaveAttribute("target", "_blank");
  await blocker.click();
  await expect(full).toHaveURL(/#\/ticket\/AL-4$/);
  await expect(
    full.getByRole("main", { name: "Ticket AL-4" }).getByRole("heading", {
      level: 1,
    }),
  ).toBeVisible();
  // Back to the ticket beside its board.
  await full.getByRole("link", { name: /^Show on the .* board$/ }).click();
  await expect(full).toHaveURL(/#\/p\/alpha\/board\?t=AL-4$/);
  await expect(
    full.getByRole("complementary", { name: "Ticket AL-4" }),
  ).toBeVisible();
  await full.close();

  // Clicking the card itself still opens the panel, with a way to the page.
  await card(page, "Long titles overflow the column").click();
  const panel = page.getByRole("complementary", { name: "Ticket AL-5" });
  await expect(
    panel.getByRole("link", { name: "Open full page in a new tab" }),
  ).toHaveAttribute("href", "#/ticket/AL-5");
  await expect(
    panel.getByRole("link", { name: "AL-5", exact: true }),
  ).toHaveAttribute("target", "_blank");
  await page.keyboard.press("Escape");

  // Table rows and stations link their ids too.
  await open(page, "#/p/flashheart/table");
  await expect(
    page.getByRole("table").getByRole("link", { name: "FH-14", exact: true }),
  ).toHaveAttribute("href", "#/ticket/FH-14");
  await open(page, "#/p/flashheart/workstreams");
  await expect(
    page
      .getByRole("article", { name: "Board core" })
      .locator('a[href="#/ticket/FH-11"]'),
  ).toHaveCount(1);
  expect(context.pages()).toHaveLength(1);
});

// sectionHeadings are the full page's section headings in page order.
async function sectionHeadings(page) {
  return page
    .getByRole("main")
    .locator("[data-section] :is(h2, h3)")
    .evaluateAll((list) =>
      list.map((heading) =>
        [...heading.childNodes]
          .filter((node) => node.nodeType === Node.TEXT_NODE)
          .map((node) => node.textContent)
          .join("")
          .trim(),
      ),
    );
}

test("the full page shows the ticket as one page with an outline and properties (FH-65)", async () => {
  const page = shared;
  const main = page.getByRole("main", { name: "Ticket FH-8" });
  const outline = main.getByRole("navigation", { name: "Sections" });
  const properties = main.getByRole("region", { name: "Properties" });

  for (const theme of ["light", "dark"]) {
    for (const width of [1440, 1280]) {
      await open(page, "#/ticket/FH-8", { width, height: 900, theme });
      await expect(outline).toBeVisible();
      await expect(properties).toBeVisible();
      await expect(main.getByRole("tab")).toHaveCount(0);
      await expect(main.getByRole("button", { name: "Jump to" })).toBeHidden();
      await expectNoAxeViolations(page, `ticket page ${theme} ${width}`);
      await shot(page, `ticket-outline-${width}-${theme}`);
    }
  }

  await open(page, "#/ticket/FH-8");
  // The ticket's sections, then the review's own, then Files and Activity.
  expect(await sectionHeadings(page)).toEqual([
    "Handoff",
    "Acceptance criteria",
    "Description",
    "Review",
    "Summary",
    "Evidence",
    "How to verify",
    "Risks / things to watch",
    "Files",
    "Activity",
  ]);
  await expect(
    outline.getByRole("button", { name: "Acceptance criteria 2 of 3" }),
  ).toBeVisible();
  await expect(
    outline.getByRole("button", { name: "Evidence 1" }),
  ).toBeVisible();
  await expect(
    outline.getByRole("button", { name: "How to verify 0 of 2" }),
  ).toBeVisible();
  await expect(outline.getByRole("button", { name: "Files 1" })).toBeVisible();
  await expect(
    outline.getByRole("button", { name: "Activity 4" }),
  ).toBeVisible();
  // The template's empty sections are listed as empty, with nowhere to go.
  await expect(outline.getByText("Test plan, empty")).toBeVisible();
  await expect(outline.getByText("Context, empty")).toBeVisible();

  // The properties panel: status as Move to, the ticket's fields, Archive.
  await expect(
    properties.getByRole("combobox", { name: "Move to" }),
  ).toHaveValue("review");
  for (const term of [
    "Priority",
    "Type",
    "Workstream",
    "Branch",
    "Depends on",
    "Tags",
    "Agent",
    "Created",
    "Changed",
  ])
    await expect(properties.locator("dt", { hasText: term })).toBeVisible();
  await expect(
    properties.getByRole("button", { name: "Archive" }),
  ).toBeVisible();

  // Outline entries by keyboard: Tab passes over the empty ones, and focus
  // moves to the chosen section.
  const description = outline.getByRole("button", { name: "Description" });
  await description.focus();
  await page.keyboard.press("Tab");
  await expect(outline.getByRole("button", { name: "Review" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(
    main.getByRole("heading", { level: 2, name: "Review" }),
  ).toBeFocused();
  await outline.getByRole("button", { name: "Activity 4" }).focus();
  await page.keyboard.press("Enter");
  await expect(
    main.getByRole("heading", { level: 2, name: "Activity" }),
  ).toBeFocused();
  await expect(
    outline.getByRole("button", { name: "Activity 4" }),
  ).toHaveAttribute("aria-current", "location");
  await expect(
    main.getByRole("heading", { level: 2, name: "Activity" }),
  ).toBeInViewport();
  // The outline and the properties stay in view down the page.
  await expect(outline).toBeInViewport();
  await expect(properties).toBeInViewport();

  // Activity: notes, the question with its answer and the handoff update,
  // oldest first, then the comment box (FH-125).
  const activity = main.getByRole("region", { name: "Activity" });
  const entries = activity.getByRole("listitem");
  await expect(entries).toHaveCount(4);
  await expect(entries.nth(0)).toContainText("one os.Root per project");
  await expect(entries.nth(1)).toContainText(
    "Treat a symlink out of the root as needs repair?",
  );
  await expect(entries.nth(1)).toContainText("Yes, and say where it points");
  await expect(entries.nth(3)).toContainText("updated the handoff");
  await expect(
    activity.getByRole("textbox", { name: "Add a comment" }),
  ).toBeVisible();
  await shot(page, "ticket-activity-1440-light");

  // Each screenshot once: the Evidence gallery opens the lightbox, and the
  // log the review does not show is under Files.
  await outline.getByRole("button", { name: "Evidence 1" }).click();
  const evidence = main.getByRole("region", { name: /^Evidence/ });
  await expect(
    main.locator('img[src$="20261004T1412-store-tests.png"]'),
  ).toHaveCount(1);
  await expect(
    main
      .getByRole("region", { name: "Files" })
      .getByRole("link", { name: /Read benchmark/ }),
  ).toBeVisible();
  await page.waitForTimeout(150);
  await shot(page, "ticket-review-1440-light");
  await evidence
    .getByRole("button", { name: "Open Store tests passing" })
    .click();
  const lightbox = page.getByRole("dialog", { name: "Store tests passing" });
  await expect(lightbox.getByRole("img")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(lightbox).toHaveCount(0);

  // The outline follows the scroll: halfway down, a later section is
  // current.
  await open(page, "#/ticket/FH-8");
  await main.evaluate((element) => {
    const target = element.querySelector('[data-section="review-summary"]');
    element.scrollTop +=
      target.getBoundingClientRect().top - element.getBoundingClientRect().top;
  });
  await expect(
    outline.getByRole("button", { name: "Summary" }),
  ).toHaveAttribute("aria-current", "location");
  await shot(page, "ticket-outline-midscroll-1440-light");

  // A ticket without a review has no Review part.
  await open(page, "#/ticket/FH-11");
  await expect(
    page.getByRole("main", { name: "Ticket FH-11" }).getByRole("heading", {
      level: 2,
      name: "Review",
    }),
  ).toHaveCount(0);
  await expect(
    page
      .getByRole("main", { name: "Ticket FH-11" })
      .getByRole("navigation", { name: "Sections" })
      .getByRole("button", { name: "Review" }),
  ).toHaveCount(0);
  await shot(page, "ticket-no-review-1440-light");

  // Narrow: a Jump to menu, the properties above the page, nothing sideways.
  // Each size starts from the top of a freshly opened page.
  const fresh = async (size) => {
    await open(page, "#/ticket/FH-11", size);
    await open(page, "#/ticket/FH-8", size);
    await main.evaluate((element) => {
      element.scrollTop = 0;
    });
  };
  for (const theme of ["light", "dark"]) {
    await fresh({ width: 390, height: 844, theme });
    await expect(outline).toBeHidden();
    const jump = main.getByRole("button", { name: "Jump to" });
    await expect(jump).toBeVisible();
    const sideways = await page.evaluate(
      () => document.scrollingElement.scrollWidth - window.innerWidth,
    );
    expect(sideways).toBeLessThanOrEqual(0);
    const above = await properties.boundingBox();
    const handoff = await main
      .getByRole("heading", { level: 2, name: "Handoff" })
      .boundingBox();
    expect(above.y + above.height).toBeLessThanOrEqual(handoff.y);
    await expectNoAxeViolations(page, `ticket page narrow ${theme}`);
    await shot(page, `ticket-outline-390-${theme}`);
    await jump.click();
    await shot(page, `ticket-jump-to-390-${theme}`, { keepFocus: true });
    await page.getByRole("button", { name: "Activity 4" }).click();
    await expect(
      main.getByRole("heading", { level: 2, name: "Activity" }),
    ).toBeFocused();
  }
  // Between phone and wide, Jump to replaces the outline beside the page.
  await fresh({ width: 1100, height: 900 });
  await expect(outline).toBeHidden();
  await expect(main.getByRole("button", { name: "Jump to" })).toBeVisible();
  const sideways = await page.evaluate(
    () => document.scrollingElement.scrollWidth - window.innerWidth,
  );
  expect(sideways).toBeLessThanOrEqual(0);
  await shot(page, "ticket-outline-1100-light");
});

test("the panel sits beside the board and keeps the card's column in view", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/board", { width: 1280, height: 800 });
  await card(page, "axe-core checks in both themes").click();
  const panel = page.getByRole("complementary", { name: "Ticket FH-31" });
  await expect(panel).toBeVisible();
  const origin = column(page, "Ready to review");
  const board = page.locator(".board-grid");
  await expect(async () => {
    const [box, panelBox, boardBox] = await Promise.all([
      origin.boundingBox(),
      panel.boundingBox(),
      board.boundingBox(),
    ]);
    // The panel does not cover the board, and the origin column is whole.
    expect(boardBox.x + boardBox.width).toBeLessThanOrEqual(panelBox.x + 1);
    expect(box.x).toBeGreaterThanOrEqual(boardBox.x - 1);
    expect(box.x + box.width).toBeLessThanOrEqual(
      boardBox.x + boardBox.width + 1,
    );
  }).toPass();
  // The rail is always its slim key badges (FH-150), leaving three whole
  // columns beside the panel.
  const rail = page.getByRole("navigation", { name: "Projects" });
  expect((await rail.boundingBox()).width).toBeLessThan(80);
  await expect(
    rail.getByRole("button", { name: /^Flashheart \(flashheart\):/ }),
  ).toBeVisible();
  await expect(async () => {
    const boardBox = await board.boundingBox();
    let whole = 0;
    for (const name of [
      "Backlog",
      "Up next",
      "In progress",
      "Ready to review",
      "Done",
    ]) {
      const box = await column(page, name).boundingBox();
      if (
        box.x >= boardBox.x - 1 &&
        box.x + box.width <= boardBox.x + boardBox.width + 1
      )
        whole++;
    }
    expect(whole).toBeGreaterThanOrEqual(3);
  }).toPass();

  // Every other column is reachable by scrolling the board.
  await board.evaluate((element) => {
    element.scrollLeft = 0;
  });
  await expect(column(page, "Backlog")).toBeInViewport();
  await board.evaluate((element) => {
    element.scrollLeft = element.scrollWidth;
  });
  await expect(column(page, "Done")).toBeInViewport();
  await page.keyboard.press("Escape");
});

// innerScrollers counts elements inside board columns that scroll on their
// own; the board is meant to be the only vertical scroller.
function innerScrollers(page) {
  return page.locator("section[data-column]").evaluateAll(
    (sections) =>
      sections
        .flatMap((section) => [section, ...section.querySelectorAll("*")])
        .filter((element) => {
          const overflow = getComputedStyle(element).overflowY;
          return (
            (overflow === "auto" || overflow === "scroll") &&
            element.scrollHeight > element.clientHeight + 1
          );
        }).length,
  );
}

// columnTops reads the top edge of every visible column, to show they move
// together.
function columnTops(page) {
  return page
    .locator("section[data-column]")
    .evaluateAll((sections) =>
      sections
        .filter((section) => section.offsetParent !== null)
        .map((section) => Math.round(section.getBoundingClientRect().top)),
    );
}

test("board columns scroll together on one vertical surface", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/board", { width: 1280, height: 800 });
  const board = page.locator(".board-grid");
  await board.evaluate((element) => {
    element.scrollLeft = 0;
  });
  await expect(
    column(page, "Backlog").getByRole("button").first(),
  ).toBeVisible();
  expect(await innerScrollers(page)).toBe(0);
  expect(
    await board.evaluate(
      (element) => element.scrollHeight - element.clientHeight,
    ),
  ).toBeGreaterThan(100);

  // Points over a short column's card, the empty space below it, and the gap
  // between two columns all scroll the one board.
  const points = await page.evaluate(() => {
    const grid = document.querySelector(".board-grid").getBoundingClientRect();
    const sections = [...document.querySelectorAll("section[data-column]")]
      .map((section) => ({
        section,
        box: section.getBoundingClientRect(),
        cards: section.querySelectorAll("[data-ticket]").length,
      }))
      .filter(({ box }) => box.left >= grid.left && box.right <= grid.right);
    const short = sections.reduce((a, b) => (b.cards < a.cards ? b : a));
    const last = [...short.section.querySelectorAll("[data-ticket]")].at(-1);
    const lastBox = last?.getBoundingClientRect();
    const below = lastBox ? lastBox.bottom + 40 : short.box.top + 160;
    return [
      lastBox
        ? { x: lastBox.left + 20, y: lastBox.top + 10, where: "short card" }
        : null,
      {
        x: short.box.left + short.box.width / 2,
        y: Math.min(below, grid.bottom - 20),
        where: "below the short column",
      },
      {
        x: sections[0].box.right + 6,
        y: grid.top + 200,
        where: "column gap",
      },
    ].filter(Boolean);
  });
  for (const point of points) {
    await board.evaluate((element) => {
      element.scrollTop = 0;
    });
    await page.mouse.move(point.x, point.y);
    await page.mouse.wheel(0, 300);
    await expect
      .poll(() => board.evaluate((element) => element.scrollTop), {
        message: point.where,
      })
      .toBeGreaterThan(0);
    expect(new Set(await columnTops(page)).size, point.where).toBe(1);
  }

  // The last card of the tallest column is reachable; headings stay usable.
  await board.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  const backlog = column(page, "Backlog");
  await expect(backlog.getByRole("button").last()).toBeInViewport();
  await expect(
    backlog.getByRole("heading", { name: /Backlog/ }),
  ).toBeInViewport();

  // Opening a card deep in the column keeps it in view; closing the panel
  // leaves the board scrolling.
  await backlog.getByRole("button").last().click();
  await expect(page.getByRole("complementary")).toBeVisible();
  await expect(backlog.getByRole("button").last()).toBeInViewport();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("complementary")).toHaveCount(0);
  await board.evaluate((element) => {
    element.scrollTop = 0;
  });
  await page.mouse.move(points[0].x, points[0].y);
  await page.mouse.wheel(0, 300);
  await expect
    .poll(() => board.evaluate((element) => element.scrollTop))
    .toBeGreaterThan(0);
  // Sideways scrolling still reaches the last column.
  await board.evaluate((element) => {
    element.scrollLeft = element.scrollWidth;
  });
  await expect(
    column(page, "Done").getByRole("heading", { name: /Done/ }),
  ).toBeInViewport({ ratio: 1 });
  await board.evaluate((element) => {
    element.scrollLeft = 0;
    element.scrollTop = 0;
  });

  // A drag held at the bottom edge scrolls the board; Escape puts it back.
  const first = await backlog.getByRole("button").first().boundingBox();
  const gridBox = await board.boundingBox();
  await page.mouse.move(first.x + first.width / 2, first.y + 20);
  await page.mouse.down();
  await page.mouse.move(first.x + first.width / 2 + 10, first.y + 30, {
    steps: 4,
  });
  await page.mouse.move(
    first.x + first.width / 2,
    gridBox.y + gridBox.height - 8,
    { steps: 10 },
  );
  await expect
    .poll(() => board.evaluate((element) => element.scrollTop))
    .toBeGreaterThan(0);
  await page.keyboard.press("Escape");
  await page.mouse.up();

  // The narrow layout scrolls its one column on the same surface.
  await open(page, "#/p/flashheart/board", { width: 390, height: 844 });
  await page.getByRole("combobox", { name: "Column" }).selectOption("backlog");
  expect(await innerScrollers(page)).toBe(0);
  await board.evaluate((element) => {
    element.scrollTop = element.scrollHeight;
  });
  await expect(backlog.getByRole("button").last()).toBeInViewport();
});

test("cards are coloured by type, priority, age or not at all", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/board");
  const locked = cardFace(page, "Locked atomic ticket writes");
  const focus = cardFace(page, "Focus ring invisible on the dark signage band");
  await expect(locked).toHaveAttribute("data-paint", "type-feature");
  await expect(focus).toHaveAttribute("data-paint", "type-bug");
  await expect(
    page.getByRole("group", { name: "Filter by type" }),
  ).toContainText("bug");

  await colourBy(page, "Priority");
  await expect(locked).toHaveAttribute("data-paint", "priority-high");
  await expect(
    locked.locator('[data-mark="priority"][title="High priority"]'),
  ).toBeVisible();
  await colourBy(page, "Age");
  await expect(locked).toHaveAttribute("data-paint", "age-today");
  await colourBy(page, "None");
  await expect(locked).not.toHaveAttribute("data-paint");
  await expect(
    page.getByRole("group", { name: /^Filter by (type|priority|age)$/ }),
  ).toHaveCount(0);
  await colourBy(page, "Type");
});

test("arrow keys move between cards and filters narrow the board", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/board");
  const first = column(page, "Backlog").getByRole("button").first();
  await first.focus();
  await page.keyboard.press("ArrowDown");
  await expect(
    column(page, "Backlog").getByRole("button").nth(1),
  ).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect(
    column(page, "Up next").getByRole("button").nth(1),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("complementary")).toBeVisible();
  await page.keyboard.press("Escape");

  await page.getByRole("searchbox", { name: "Search tickets" }).fill("handoff");
  await expect(page.getByText(/of 50 tickets/)).toBeVisible();
  await expect(card(page, "Opt-in handoff enforcement at Stop")).toBeVisible();
  await page.getByRole("searchbox", { name: "Search tickets" }).fill("");
  const state = await filterMenu(page, "State");
  await state.getByRole("button", { name: "Blocked", exact: true }).click();
  await expect(state).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "State: Blocked" }),
  ).toBeVisible();
  for (const button of await column(page, "Backlog")
    .getByRole("button")
    .all()) {
    await expect(button).toHaveAccessibleName(/blocked/);
  }
  await page.getByRole("button", { name: "Clear filters" }).click();
});

test("chips and filter menus show only or hide values (FH-39)", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/board");
  const locked = card(page, "Locked atomic ticket writes");
  const focusRing = card(page, "Focus ring invisible on the dark signage band");
  const lines = page.getByRole("group", { name: /^Filter by workstream/ });
  const types = page.getByRole("group", { name: "Filter by type" });
  const typeButton = filterButton(page, "Type");

  // A workstream chip hides the other tickets, and a second click restores.
  const boardCore = lines.getByRole("button", { name: /^Board core/ });
  await boardCore.click();
  await expect(boardCore).toHaveAttribute("aria-pressed", "true");
  await expect(locked).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: /^Workstream: 1 chosen/ }),
  ).toBeVisible();
  await boardCore.click();
  await expect(locked).toBeVisible();

  // Cmd or Ctrl click hides a value; a click restores it.
  const editing = lines.getByRole("button", { name: /^Board editing/ });
  await editing.click({ modifiers: ["ControlOrMeta"] });
  await expect(editing).toHaveAccessibleName(/, hidden$/);
  await expect(locked).toHaveCount(0);
  await expect(focusRing).toBeVisible();

  // The colour key filters too, and the Type menu shows the same choice.
  const bug = types.getByRole("button", { name: "bug", exact: true });
  await bug.click();
  await expect(bug).toHaveAttribute("aria-pressed", "true");
  await expect(typeButton).toHaveAccessibleName("Type: 1 chosen");
  await expect(focusRing).toBeVisible();
  for (const shown of await page.locator("[data-paint]").all())
    await expect(shown).toHaveAttribute("data-paint", "type-bug");

  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/flashheart/board", {
      width: 1440,
      height: 900,
      theme,
    });
    await shot(page, `board-chips-${theme}`);
    const menu = await filterMenu(page, "Type");
    await expect(
      menu.getByRole("button", { name: "bug", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    await shot(page, `board-type-menu-${theme}`);
    await page.keyboard.press("Escape");
    await expect(menu).toHaveCount(0);
    await expect(typeButton).toBeFocused();
    await page.getByRole("button", { name: "View options" }).click();
    await expect(page.getByRole("group", { name: "Colour by" })).toBeVisible();
    await shot(page, `board-view-options-${theme}`);
    await expect(
      page.getByRole("checkbox", { name: "Agent working" }),
    ).toHaveCount(0);
    await page.locator("h1").click();
    const states = await filterMenu(page, "State");
    await expect(
      states.getByRole("button", { name: "Agent working" }),
    ).toBeVisible();
    await shot(page, `board-state-menu-${theme}`);
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "View options" }).click();
    // A click outside closes a menu.
    await page.locator("h1").click();
    await expect(page.getByRole("group", { name: "Colour by" })).toHaveCount(0);
  }

  // A menu item toggles as its chip does, by mouse or by keyboard.
  const menu = await filterMenu(page, "Type");
  await menu.getByRole("button", { name: "bug", exact: true }).click();
  await expect(bug).toHaveAttribute("aria-pressed", "false");
  await menu.getByRole("button", { name: "feature", exact: true }).focus();
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(
    types.getByRole("button", { name: "feature, hidden" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");

  // Clear filters resets chips and menus alike.
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(locked).toBeVisible();
  await expect(typeButton).toHaveAccessibleName("Type");
  await expect(lines.getByRole("button", { name: /, hidden$/ })).toHaveCount(0);

  // The chip row never wraps: chips that do not fit leave the row, colour
  // chips before workstreams.
  await open(page, "#/p/flashheart/board", { width: 900, height: 900 });
  const shownChips = (group) =>
    group
      .getByRole("button", { includeHidden: true })
      .evaluateAll((nodes) =>
        nodes
          .filter((node) => getComputedStyle(node).visibility !== "hidden")
          .map((node) => Math.round(node.getBoundingClientRect().top)),
      );
  const lineTops = await shownChips(lines);
  const typeTops = await shownChips(types);
  expect(lineTops.length).toBe(
    await lines.getByRole("button", { includeHidden: true }).count(),
  );
  expect(typeTops.length).toBeLessThan(
    await types.getByRole("button", { includeHidden: true }).count(),
  );
  expect(new Set([...lineTops, ...typeTops]).size).toBe(1);
  await expectNoAxeViolations(page, "board chips");
});

test("workstreams draw as lines with stations", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/workstreams");
  const boardCore = page.getByRole("article", { name: "Board core" });
  await expect(
    boardCore
      .getByRole("list", { name: "Board core stations" })
      .getByRole("listitem"),
  ).toHaveCount(8);
  await expect(
    boardCore.getByRole("button", {
      name: /Read API for projects, boards and tickets, In progress, next stop/,
    }),
  ).toBeVisible();
  await expect(page.getByRole("article", { name: "Agent runs" })).toContainText(
    "Depends on workstream board-editing",
  );
  // Track follows depends-on: Table view and Transit map UI both branch off
  // the read API, so both lead out of it in parallel.
  await expect(
    boardCore.getByRole("button", {
      name: /^Sortable table view, Up next, blocked, needs Read API/,
    }),
  ).toBeVisible();
  await expect(boardCore.locator("[data-track]")).toHaveCount(8);
  // A ticket with no links on its line waits below the graph.
  const editing = page.getByRole("article", { name: "Board editing" });
  await expect(
    editing
      .getByRole("list", {
        name: "Board editing stations with no dependencies on the line",
      })
      .getByRole("listitem"),
  ).toHaveCount(1);
  await boardCore
    .getByRole("button", { name: /^Transit map board UI,/ })
    .click();
  await expect(
    page.getByRole("complementary", { name: "Ticket FH-11" }),
  ).toBeVisible();
});

test("table sorts and opens tickets", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/table");
  await page
    .getByRole("table")
    .getByRole("button", { name: "Priority" })
    .click();
  await expect(
    page.getByRole("columnheader", { name: "Priority" }),
  ).toHaveAttribute("aria-sort", "ascending");
  await expect(page.getByRole("row").nth(1)).toContainText("High");
  await page
    .getByRole("button", { name: "Locked atomic ticket writes" })
    .click();
  await expect(
    page.getByRole("complementary", { name: "Ticket FH-14" }),
  ).toBeVisible();
});

// The card's type icon and priority bars beside their words wherever
// there is room for both (FH-154, ui-layout.md §2, §3, §6).
test("the table, panel, page and priority chips show type and priority as cards do (FH-154)", async () => {
  const page = shared;
  // mark is the drawing just before a word, in the same tag or row.
  const marked = (scope, selector, word) =>
    scope
      .locator(selector)
      .locator("xpath=..")
      .filter({ hasText: new RegExp(`^${word}$`) });
  for (const theme of ["light", "dark"]) {
    for (const [width, height] of [
      [1440, 900],
      [390, 844],
    ]) {
      const size = { width, height, theme };
      const at = `${width === 390 ? "narrow" : width}-${theme}`;

      await open(page, "#/p/flashheart/table", size);
      const row = page
        .getByRole("row")
        .filter({ hasText: "Locked atomic ticket writes" });
      await expect(
        marked(row, '[data-type="feature"]', "feature"),
      ).toBeVisible();
      await expect(marked(row, '[data-priority="high"]', "High")).toBeVisible();
      await expectNoAxeViolations(page, `table marks ${at}`);
      await shot(page, `marks-table-${at}`);

      await open(page, "#/p/flashheart/board?t=FH-14", size);
      const panel = page.getByRole("complementary", { name: "Ticket FH-14" });
      await expect(
        marked(panel, '[data-priority="high"]', "High priority"),
      ).toBeVisible();
      await expect(
        marked(panel, '[data-type="feature"]', "feature"),
      ).toBeVisible();
      await expectNoAxeViolations(page, `panel marks ${at}`);
      await shot(page, `marks-panel-${at}`);

      await open(page, "#/ticket/FH-14", size);
      const properties = page.getByRole("region", { name: "Properties" });
      await expect(
        properties.locator('dd:has([data-priority="high"])'),
      ).toContainText("High");
      await expect(
        properties.locator('dd:has([data-type="feature"])'),
      ).toContainText("Feature");
      await expectNoAxeViolations(page, `page marks ${at}`);
      await shot(page, `marks-ticket-page-${at}`);
    }

    await open(page, "#/p/flashheart/board", {
      width: 1440,
      height: 900,
      theme,
    });
    await colourBy(page, "Priority");
    try {
      const chips = page.getByRole("group", { name: "Filter by priority" });
      for (const level of ["high", "medium", "low"]) {
        await expect(
          chips.locator(`button:has([data-priority="${level}"])`),
        ).toBeVisible();
      }
      await expectNoAxeViolations(page, `priority chips ${theme}`);
      await shot(page, `marks-priority-chips-${theme}`);
    } finally {
      await colourBy(page, "Type");
    }
  }
});

test("every view names its scope in the page header, not the band", async () => {
  const page = shared;
  for (const view of ["board", "overview", "workstreams", "table"]) {
    await open(page, `#/p/flashheart/${view}`);
    await expect(
      page.getByRole("main").getByRole("heading", { level: 1 }),
    ).toHaveText(/Flashheart/);
    await expect(page.getByRole("banner").getByRole("heading")).toHaveCount(0);
  }
  await open(page, "#/all/board");
  await expect(
    page.getByRole("main").getByRole("heading", { level: 1 }),
  ).toHaveText(/All projects/);
});

for (const theme of ["light", "dark"]) {
  test(`board views are accessible and render cleanly in ${theme}`, async () => {
    const page = shared;
    for (const [width, height] of [
      [1280, 800],
      [1920, 1080],
    ]) {
      const size = { width, height, theme };
      await open(page, "#/p/flashheart/board", size);
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      await expectNoAxeViolations(page, `board ${theme} ${width}`);
      await shot(page, `board-${width}-${theme}`);

      await open(page, "#/all/board", size);
      await shot(page, `all-${width}-${theme}`);

      await open(page, "#/p/flashheart/board?t=FH-11", size);
      await expect(page.getByRole("complementary")).toBeVisible();
      await expectNoAxeViolations(page, `panel ${theme} ${width}`);
      await shot(page, `panel-${width}-${theme}`);

      await open(page, "#/p/flashheart/board?t=FH-8", size);
      await page.getByRole("tab", { name: "Review" }).click();
      await expect(
        page.getByRole("img", { name: "Store tests passing" }).first(),
      ).toBeVisible();
      await expectNoAxeViolations(page, `review ${theme} ${width}`);
      await shot(page, `review-${width}-${theme}`);

      await open(page, "#/ticket/FH-8", size);
      await expect(
        page.getByRole("button", { name: "Open Store tests passing" }),
      ).toBeVisible();
      await expectNoAxeViolations(page, `full page ${theme} ${width}`);
      await shot(page, `ticket-page-${width}-${theme}`);

      await open(page, "#/p/alpha/board?t=AL-7", size);
      await expect(
        page.getByRole("heading", { name: "Needs repair" }),
      ).toBeVisible();
      await shot(page, `repair-${width}-${theme}`);

      await open(page, "#/p/flashheart/workstreams", size);
      await expect(page.getByRole("article").first()).toBeVisible();
      await expectNoAxeViolations(page, `workstreams ${theme} ${width}`);
      await shot(page, `workstreams-${width}-${theme}`);

      await open(page, "#/p/flashheart/table", size);
      await expect(page.getByRole("table")).toBeVisible();
      await expectNoAxeViolations(page, `table ${theme} ${width}`);
      await shot(page, `table-${width}-${theme}`);
    }
    await open(page, "#/p/flashheart/board", {
      width: 1440,
      height: 900,
      theme,
    });
    for (const density of ["Compact", "Detailed"]) {
      await viewOption(page, "Density", density);
      await shot(page, `board-${density.toLowerCase()}-${theme}`);
    }
    await viewOption(page, "Density", "Normal");
    await open(page, "#/p/flashheart/board", {
      width: 390,
      height: 844,
      theme,
    });
    await page
      .getByRole("combobox", { name: "Column" })
      .selectOption("up-next");
    // One chosen column fills the width; nothing scrolls sideways.
    const overflow = await page.evaluate(
      () => document.scrollingElement.scrollWidth - window.innerWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
    const column = await page
      .locator('section[data-column="up-next"]')
      .boundingBox();
    expect(column?.width ?? 0).toBeGreaterThan(300);
    await shot(page, `board-narrow-${theme}`);
    // Every view and the ticket panel fit a phone without sideways scroll.
    for (const [name, hash] of [
      ["workstreams", "#/p/flashheart/workstreams"],
      ["table", "#/p/flashheart/table"],
      ["overview", "#/all/overview"],
      ["panel", "#/p/flashheart/board?t=FH-11"],
      ["ticket-page", "#/ticket/FH-11"],
    ]) {
      await open(page, hash, { width: 390, height: 844, theme });
      const sideways = await page.evaluate(
        () => document.scrollingElement.scrollWidth - window.innerWidth,
      );
      expect(sideways, name).toBeLessThanOrEqual(0);
      await shot(page, `${name}-narrow-${theme}`);
    }
  });
}

test("marginal remarks: one per screen, only in quiet states, and stable", async () => {
  const page = shared;
  const asides = page.locator("[data-aside]:visible");
  for (const hash of [
    "#/p/flashheart/board",
    "#/all/board",
    "#/all/overview",
    "#/p/flashheart/workstreams",
    "#/p/flashheart/table",
    "#/p/flashheart/board?t=FH-11",
    "#/p/alpha/board?t=AL-3",
  ]) {
    await open(page, hash);
    await page.waitForTimeout(150);
    expect(await asides.count(), hash).toBeLessThanOrEqual(1);
  }

  // A search that finds nothing earns one remark, which stays put while
  // the board refreshes and re-renders.
  await open(page, "#/p/flashheart/board");
  await page
    .getByRole("searchbox", { name: "Search tickets" })
    .fill("no ticket says this");
  const empty = page.getByText("No tickets match these filters");
  await expect(empty).toBeVisible();
  await expect(asides).toHaveCount(1);
  const first = await asides.textContent();
  await viewOption(page, "Density", "Compact");
  await page.waitForTimeout(1500);
  await viewOption(page, "Density", "Normal");
  expect(await asides.textContent()).toBe(first);
  // Remarks never sit in alerts, blockers or controls.
  await expect(page.locator("[role=alert] [data-aside]")).toHaveCount(0);
  await expect(page.locator("button [data-aside]")).toHaveCount(0);
  await page.getByRole("searchbox", { name: "Search tickets" }).fill("");
  await expect(empty).toHaveCount(0);
});

test("no request leaves the loopback origin", async () => {
  const page = shared;
  const foreign = [];
  page.on("request", (request) => {
    if (!/^http:\/\/ss-[0-9a-f]+\.localhost:\d+\//.test(request.url()))
      foreign.push(request.url());
  });
  await open(page, "#/p/flashheart/board?t=FH-8");
  await page.getByRole("tab", { name: "Review" }).click();
  await open(page, "#/p/flashheart/workstreams");
  expect(foreign).toEqual([]);
});
