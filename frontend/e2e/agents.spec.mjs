import { resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { seedRuns } from "./agent-runs.mjs";
import {
  launch,
  makeSandbox,
  screenshotDir,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// Agent runs end to end: sessions are recorded by `flashheart hook claude`,
// exactly as Claude Code runs it, then read back by the board.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  await seedRuns(sandbox.home, sandbox.root);
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

async function open(
  hash,
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

async function expectNoAxeViolations(label) {
  const results = await new AxeBuilder({ page }).analyze();
  expect(
    results.violations,
    `${label}: ${results.violations.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`,
  ).toEqual([]);
}

async function shot(name) {
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement)
      document.activeElement.blur();
  });
  await page.screenshot({ path: resolve(screenshotDir, `${name}.png`) });
}

const lane = (name) => page.getByRole("region", { name, exact: true });

test("the Agents view lists runs by state, Needs you first", async () => {
  await open("#/all/agents");
  const lanes = page.locator("[data-lane]");
  await expect(lanes).toHaveCount(5);
  await expect(lanes.first()).toHaveAttribute("data-lane", "needs-you");

  const needsYou = lane("Needs you 1");
  await expect(needsYou.getByText("Permission for Bash")).toBeVisible();
  await expect(
    needsYou.getByRole("button", { name: /AL-3 Card panel/ }),
  ).toBeVisible();
  await expect(
    needsYou.getByRole("img", { name: "Plan 2 of 5 done" }),
  ).toBeVisible();

  const working = lane("Working 1");
  await expect(working.getByText("Unassigned")).toBeVisible();
  await expect(
    working.getByRole("list", { name: /Subagents of/ }).getByText("Explore"),
  ).toBeVisible();
  await expect(lane("Waiting 1").getByText("spike/offline-sync")).toBeVisible();
  await expect(
    lane("Ended 1").getByText("No handoff since its edits"),
  ).toBeVisible();

  // The band shows Needs you from any view.
  await expect(page.getByRole("button", { name: /1 needs you/ })).toBeVisible();

  // A row opens to what the run did.
  await needsYou.getByRole("button", { name: /Claude/ }).click();
  await expect(needsYou.getByText("Asked permission for Bash")).toBeVisible();
  await expect(
    needsYou.getByText("src/panel/RunsTab.tsx").first(),
  ).toBeVisible();
  await expectNoAxeViolations("agents light");

  for (const [width, height] of [
    [1280, 800],
    [1920, 1080],
  ]) {
    for (const theme of ["light", "dark"]) {
      await open("#/all/agents", { width, height, theme });
      await shot(`agents-${width}-${theme}`);
    }
  }
  await open("#/all/agents", { theme: "dark" });
  await expectNoAxeViolations("agents dark");
  // Narrow widths stack each row's cells; nothing scrolls sideways.
  await open("#/all/agents", { width: 390, height: 844 });
  const overflow = await page.evaluate(
    () => document.scrollingElement.scrollWidth - window.innerWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
  await shot("agents-390-light");
});

test("cards carry live runs and the Needs you column mirrors them", async () => {
  await open("#/p/alpha/board");
  const needsColumn = page.getByRole("region", { name: /^Needs you/ });
  const mirror = needsColumn.getByRole("button", {
    name: /^Card panel, AL-3, .*also in In progress/,
  });
  await expect(mirror).toBeVisible();
  const real = page
    .getByRole("region", { name: /^In progress/ })
    .getByRole("button", { name: /^Card panel, AL-3, Claude needs you/ });
  await expect(real).toBeVisible();
  // The live badge says what the run needs and keeps its plan step.
  await expect(real.getByText("Permission for Bash")).toBeVisible();
  await expect(real.getByText("2/5 · Runs tab timeline")).toBeVisible();

  // Virtual columns appear only while they hold tickets, and can be hidden.
  await expect(
    page.getByRole("region", { name: /^Agent working/ }),
  ).toHaveCount(0);
  await page.getByRole("checkbox", { name: "Needs you" }).uncheck();
  await expect(needsColumn).toHaveCount(0);
  await page.getByRole("checkbox", { name: "Needs you" }).check();
  await expect(mirror).toBeVisible();
  // Turning it back on keeps it in view rather than off to the left.
  await expect(needsColumn.getByRole("heading")).toBeInViewport();
  await expectNoAxeViolations("board with runs");
  for (const theme of ["light", "dark"]) {
    await page.reload();
    await open("#/p/alpha/board", { theme });
    await expect(needsColumn.getByRole("heading")).toBeInViewport();
    await expect(mirror).toBeInViewport();
    await shot(`board-runs-1440-${theme}`);
  }

  // The Runs tab shows the ticket's runs and the handoff warning.
  await open("#/p/alpha/board");
  await real.click();
  await page.getByRole("tab", { name: /^Runs/ }).click();
  const panel = page.getByRole("complementary", { name: "Ticket AL-3" });
  await expect(
    panel.getByText(
      "Ended without a handoff: 2 edits since its last checkpoint.",
    ),
  ).toBeVisible();
  await expect(
    panel.getByText("Asked permission for Bash").first(),
  ).toBeVisible();
  const scrolled = await page.evaluate(
    () => document.scrollingElement?.scrollTop ?? 0,
  );
  expect(scrolled).toBe(0);
  await expectNoAxeViolations("runs tab");
  await shot("runs-tab-1440-light");
  await open("#/p/alpha/board?t=AL-3", { theme: "dark" });
  await page.getByRole("tab", { name: /^Runs/ }).click();
  await shot("runs-tab-1440-dark");
});
