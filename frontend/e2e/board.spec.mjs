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

async function open(
  page,
  hash = "#/all/board",
  { width = 1440, height = 900, theme = "light" } = {},
) {
  await page.setViewportSize({ width, height });
  await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
  await page.evaluate((target) => {
    window.location.hash = target;
  }, hash);
  await expect(
    page.getByRole("button", { name: "Backend connected. Check connection" }),
  ).toBeVisible();
}

function column(page, name) {
  return page.getByRole("region", { name });
}

async function colourBy(page, option) {
  await page.getByRole("combobox", { name: "Colour by" }).selectOption({
    label: option,
  });
}

function card(page, title) {
  return page.getByRole("button", { name: new RegExp(`^${title},`) });
}

async function expectNoAxeViolations(page, label) {
  const results = await new AxeBuilder({ page }).analyze();
  expect(
    results.violations,
    `${label}: ${results.violations.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`,
  ).toEqual([]);
}

async function shot(page, name) {
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement)
      document.activeElement.blur();
  });
  await page.screenshot({ path: resolve(screenshotDir, `${name}.png`) });
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
  await expect(card(page, "Drag and drop")).toContainText(
    "Comes after AL-3 in workstream board-ui, which is In progress",
  );
  await expect(card(page, "Long titles overflow the column")).toContainText(
    "Depends on AL-4, which is Up next",
  );
  await expect(card(page, "Offline mode")).toContainText(
    "Depends on feat--does-not-exist, which does not exist",
  );
  await expect(card(page, "Broken frontmatter")).toHaveAccessibleName(
    /needs repair/,
  );
  await expect(card(page, "Broken frontmatter")).toContainText(
    "frontmatter does not parse",
  );
  await expect(card(page, "Card panel")).not.toContainText("Comes after");
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
  await review.getByRole("link", { name: "AL-2", exact: true }).click();
  await expect(page).toHaveURL(/t=AL-2/);
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

  // A bare ticket id in markdown links to that ticket (KEY-3).
  await storePanel.getByRole("link", { name: "FH-26", exact: true }).click();
  await expect(page).toHaveURL(/t=FH-26/);
  await expect(
    page.getByRole("complementary", { name: "Ticket FH-26" }),
  ).toBeVisible();
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
  // Below 1440px the rail shrinks to its key badges, leaving three whole
  // columns beside the panel.
  const rail = page.getByRole("navigation", { name: "Projects" });
  expect((await rail.boundingBox()).width).toBeLessThan(80);
  await expect(rail.getByRole("button", { name: /Flashheart/ })).toBeVisible();
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
  const locked = card(page, "Locked atomic ticket writes");
  const focus = card(page, "Focus ring invisible on the dark signage band");
  await expect(locked).toHaveAttribute("data-paint", "type-feature");
  await expect(focus).toHaveAttribute("data-paint", "type-bug");
  await expect(
    page.getByRole("list", { name: "Card colours by type" }),
  ).toContainText("bug");

  await colourBy(page, "Priority");
  await expect(locked).toHaveAttribute("data-paint", "priority-high");
  await expect(locked).toContainText("High");
  await colourBy(page, "Age");
  await expect(locked).toHaveAttribute("data-paint", "age-today");
  await colourBy(page, "None");
  await expect(locked).not.toHaveAttribute("data-paint");
  await expect(page.getByRole("list", { name: /Card colours/ })).toHaveCount(0);
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
  await page.getByRole("combobox", { name: "State" }).selectOption("blocked");
  for (const button of await column(page, "Backlog")
    .getByRole("button")
    .all()) {
    await expect(button).toHaveAccessibleName(/blocked/);
  }
  await page.getByRole("button", { name: "Clear filters" }).click();
  await page.getByRole("button", { name: /Board core/ }).click();
  await expect(card(page, "Locked atomic ticket writes")).toHaveClass(
    /opacity-35/,
  );
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
  await boardCore.getByRole("button", { name: /Transit map board UI/ }).click();
  await expect(
    page.getByRole("complementary", { name: "Ticket FH-11" }),
  ).toBeVisible();
});

test("table sorts and opens tickets", async () => {
  const page = shared;
  await open(page, "#/p/flashheart/table");
  await page.getByRole("button", { name: "Priority" }).click();
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

test("every view names its scope in the page header, not the band", async () => {
  const page = shared;
  for (const view of ["board", "agents", "workstreams", "table"]) {
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
      await page.getByText(density, { exact: true }).click();
      await shot(page, `board-${density.toLowerCase()}-${theme}`);
    }
    await page.getByText("Normal", { exact: true }).click();
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
      ["agents", "#/all/agents"],
      ["panel", "#/p/flashheart/board?t=FH-11"],
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
