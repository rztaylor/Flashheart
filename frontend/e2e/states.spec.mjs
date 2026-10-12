import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { writeDemoBoard } from "./demo-board.mjs";
import {
  expectConnected,
  expectNoAxeViolations,
  launch,
  makeSandbox,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// The load states of ui-layout.md §8 in every view: a failed load says why
// with Try again, a failed reload keeps the last good copy with a note, and
// loading shows a skeleton in the view's own shape. Requests are failed or
// held in the browser (page.route); the server is the real binary.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let origin;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext();
  // The first page holds the session for the file; each test opens its
  // own tabs, which share it.
  const first = await context.newPage();
  await first.goto(url);
  await first.waitForURL((current) => current.pathname === "/");
  origin = new URL(first.url()).origin;
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

const reason = "permission denied reading the board folder";

// at matches a request by its path, whatever its query. The same matcher
// must be passed to unroute, so each path keeps one.
const matchers = new Map();
function at(path) {
  if (!matchers.has(path)) matchers.set(path, (url) => url.pathname === path);
  return matchers.get(path);
}

function fail(route) {
  return route.fulfill({
    status: 500,
    contentType: "application/json",
    body: JSON.stringify({ error: { code: "internal", message: reason } }),
  });
}

// tab opens a new tab on hash, after setup has routed its requests.
async function tab(
  hash,
  { width = 1440, height = 900, theme = "light" } = {},
  setup = async () => {},
) {
  const page = await context.newPage();
  await page.setViewportSize({ width, height });
  await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
  await setup(page);
  await page.goto(`${origin}/${hash}`);
  await expectConnected(page);
  return page;
}

// reload asks every view to load again, as returning to the tab does.
const reload = (page) =>
  page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));

// The views, the request that loads each, what its error says and what it
// shows once loaded. shots marks the states this change adds, which are
// photographed for review.
const views = [
  {
    name: "projects",
    hash: "#/p/flashheart/board",
    path: "/api/projects",
    error: "The board could not be read",
    loaded: (page) =>
      page
        .getByRole("navigation", { name: "Projects" })
        .getByText("Flashheart", { exact: true }),
    shots: true,
  },
  {
    name: "board",
    hash: "#/p/flashheart/board",
    path: "/api/projects/flashheart/board",
    error: "This board could not be loaded",
    loaded: (page) => page.locator('section[data-column="up-next"]'),
  },
  {
    name: "overview",
    hash: "#/p/flashheart/overview",
    path: "/api/projects/flashheart/board",
    error: "The overview could not be loaded",
    loaded: (page) => page.getByText("Needs your decision").first(),
  },
  {
    name: "workstreams",
    hash: "#/p/flashheart/workstreams",
    path: "/api/projects/flashheart/workstreams",
    error: "Workstreams could not be loaded",
    loaded: (page) => page.getByRole("article").first(),
    shots: true,
  },
  {
    name: "archive",
    hash: "#/p/flashheart/archive",
    path: "/api/projects/flashheart/archive",
    error: "The archive could not be loaded",
    loaded: (page) => page.getByText("Nothing archived"),
    stale: true,
  },
  {
    name: "projects-archive",
    hash: "#/all/archive",
    path: "/api/archived-projects",
    error: "The archive could not be loaded",
    loaded: (page) => page.getByText("No archived projects"),
    stale: true,
  },
  {
    name: "panel",
    hash: "#/p/flashheart/board?t=FH-11",
    path: "/api/tickets/FH-11",
    error: "This ticket could not be loaded",
    loaded: (page) =>
      page
        .getByRole("complementary")
        .getByRole("heading", { name: "Transit map board UI" }),
    shots: true,
  },
  {
    name: "ticket-page",
    hash: "#/ticket/FH-11",
    path: "/api/tickets/FH-11",
    error: "This ticket could not be loaded",
    loaded: (page) => page.getByRole("main").getByRole("heading", { level: 1 }),
    shots: true,
  },
];

for (const view of views) {
  test(`the ${view.name} says why it could not load, and Try again recovers`, async () => {
    const page = await tab(view.hash, {}, (page) =>
      page.route(at(view.path), fail),
    );
    const alert = page
      .getByRole("alert")
      .filter({ hasText: `${view.error}: ${reason}` });
    await expect(alert).toBeVisible();
    const retry = page.getByRole("button", { name: "Try again" });
    await expect(retry).toBeVisible();
    await expectNoAxeViolations(page, `${view.name} error`);
    if (view.shots) {
      await shot(page, `state-error-${view.name}-1440-light`);
      await page.emulateMedia({ colorScheme: "dark" });
      await shot(page, `state-error-${view.name}-1440-dark`);
    }
    await page.unroute(at(view.path));
    await retry.click();
    await expect(view.loaded(page)).toBeVisible();
    await expect(alert).toHaveCount(0);
    await page.close();
  });
}

test("the projects' error fits a phone", async () => {
  for (const theme of ["light", "dark"]) {
    const page = await tab(
      "#/p/flashheart/board",
      { width: 390, height: 844, theme },
      (page) => page.route(at("/api/projects"), fail),
    );
    await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
    const sideways = await page.evaluate(
      () => document.scrollingElement.scrollWidth - window.innerWidth,
    );
    expect(sideways).toBeLessThanOrEqual(0);
    await shot(page, `state-error-projects-390-${theme}`);
    await page.close();
  }
});

for (const view of views.filter((item) => item.shots || item.stale)) {
  test(`the ${view.name} keeps the last good copy with a note when a reload fails`, async () => {
    const page = await tab(view.hash);
    await expect(view.loaded(page)).toBeVisible();
    await page.route(at(view.path), fail);
    await reload(page);
    const note = page.getByRole("status").filter({
      hasText: `Showing the last good copy: ${reason}`,
    });
    await expect(note).toBeVisible();
    await expect(view.loaded(page)).toBeVisible();
    await expectNoAxeViolations(page, `${view.name} stale`);
    await shot(page, `state-stale-${view.name}-1440-light`);
    await page.emulateMedia({ colorScheme: "dark" });
    await shot(page, `state-stale-${view.name}-1440-dark`);
    await page.unroute(at(view.path));
    await reload(page);
    await expect(note).toHaveCount(0);
    await page.close();
  });
}

// Skeletons: the request a view waits on is held until the skeleton has
// been checked and photographed.
const skeletons = [
  ["overview", "#/p/flashheart/overview", "/api/projects", "overview"],
  ["workstreams", "#/p/flashheart/workstreams", "/api/projects", "workstreams"],
  [
    "workstreams-lines",
    "#/p/flashheart/workstreams",
    "/api/projects/flashheart/workstreams",
    "route-card",
  ],
  ["archive", "#/p/flashheart/archive", "/api/projects", "archive"],
  ["projects-archive", "#/all/archive", "/api/archived-projects", "archive"],
  ["board", "#/p/flashheart/board", "/api/projects/flashheart/board", "board"],
  ["table", "#/p/flashheart/table", "/api/projects/flashheart/board", "table"],
];

for (const [name, hash, path, shape] of skeletons) {
  test(`the ${name} loads in its own shape`, async () => {
    let release;
    const held = new Promise((resolveHeld) => {
      release = resolveHeld;
    });
    const page = await tab(hash, {}, (page) =>
      page.route(at(path), async (route) => {
        await held;
        await route.continue().catch(() => undefined);
      }),
    );
    const skeleton = page.locator(`[data-skeleton="${shape}"]`).first();
    await expect(skeleton).toBeVisible();
    for (const other of [
      "overview",
      "workstreams",
      "archive",
      "board",
      "table",
    ])
      if (
        other !== shape &&
        !(shape === "route-card" && other === "workstreams")
      )
        await expect(page.locator(`[data-skeleton="${other}"]`)).toHaveCount(0);
    await shot(page, `state-loading-${name}-1440-light`);
    await page.emulateMedia({ colorScheme: "dark" });
    await shot(page, `state-loading-${name}-1440-dark`);
    release();
    await expect(page.locator("[data-skeleton]")).toHaveCount(0);
    await page.close();
  });
}
