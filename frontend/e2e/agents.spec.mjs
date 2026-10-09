import { appendFile, readFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import {
  callTool,
  IDLE_SESSION,
  promptHook,
  seedRuns,
  TASK_NOTIFICATION,
} from "./agent-runs.mjs";
import {
  filterButton,
  filterMenu,
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
let seeded;
let server;
let context;
let page;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  // Answers are recorded under the configured name (FH-8).
  await appendFile(
    join(sandbox.root, ".flashheart", "config.yaml"),
    "user_name: Robin\n",
  );
  seeded = await seedRuns(sandbox.home, sandbox.root);
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
  // The title opens the panel; the id, the ticket's full page (CARD-7).
  await expect(
    needsYou.getByRole("button", { name: "Card panel", exact: true }),
  ).toBeVisible();
  await expect(
    needsYou.getByRole("link", { name: "AL-3", exact: true }),
  ).toHaveAttribute("href", "#/ticket/AL-3");
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

  // The band shows Needs you from any view, with one accessible name.
  await expect(
    page.getByRole("button", { name: "1 needs you", exact: true }),
  ).toBeVisible();

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

test("the band's Needs you pill opens the Board, filtered, from other views (FH-44)", async () => {
  await open("#/all/agents");
  const pill = page.getByRole("button", { name: "1 needs you", exact: true });
  await expect(pill).toHaveAttribute("aria-pressed", "false");
  await pill.click();
  await expect(page).toHaveURL(/#\/all\/board/);
  await expect(pill).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator("[data-ticket]")).toHaveCount(1);
  await expect(
    page.getByRole("button", { name: /^Card panel, AL-3,/ }),
  ).toBeVisible();
  await pill.click();
  await expect(pill).toHaveAttribute("aria-pressed", "false");
});

test("cards carry live runs and the band's Needs you pill filters to them (FH-44)", async () => {
  await open("#/p/alpha/board");
  const real = page
    .getByRole("region", { name: /^In progress/ })
    .getByRole("button", { name: /^Card panel, AL-3, Claude needs you/ });
  await expect(real).toBeVisible();
  // The live badge says what the run needs and keeps its plan step; the
  // card's button covers its drawing, which sits beside it.
  const face = real.locator("xpath=..");
  await expect(face.getByText("Permission for Bash")).toBeVisible();
  await expect(face.getByText("2/5 · Runs tab timeline")).toBeVisible();

  // Needs you and Agent working are filters, never columns (FH-42, FH-44);
  // neither the View options menu nor the chip row offers them.
  await expect(page.getByRole("region", { name: /^Needs you/ })).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: /^Agent working/ }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "View options" }).click();
  await expect(page.getByRole("checkbox", { name: "Needs you" })).toHaveCount(
    0,
  );
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: /^Needs you/ })).toHaveCount(0);

  // The band's pill is an on/off filter over All projects, where its count
  // is taken: pressed, the Board shows only the tickets that need you.
  const cards = page.locator("[data-ticket]");
  const total = await cards.count();
  const pill = page.getByRole("button", { name: "1 needs you", exact: true });
  await pill.click();
  await expect(page).toHaveURL(/#\/all\/board/);
  await expect(pill).toHaveAttribute("aria-pressed", "true");
  await expect(real).toBeVisible();
  await expect(cards).toHaveCount(1);
  await expect(page.getByText(/^1 of \d+ tickets$/)).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Clear filters" }),
  ).toBeVisible();
  // Every agent that needs you has its ticket on the board, so no notice.
  await expect(
    page.getByRole("region", { name: "Agents without a ticket" }),
  ).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    await open("#/all/board", { theme });
    await shot(`board-needs-you-1440-${theme}`);
  }
  for (const theme of ["light", "dark"]) {
    await open("#/all/board", { width: 390, height: 844, theme });
    await expect(
      page.getByRole("button", { name: "1 agent needs you", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    await shot(`board-needs-you-390-${theme}`);
  }
  await open("#/all/board");

  // A second press goes back to the project it was pressed in.
  await pill.click();
  await expect(page).toHaveURL(/#\/p\/alpha\/board/);
  await expect(pill).toHaveAttribute("aria-pressed", "false");
  await expect(cards).toHaveCount(total);

  // On the Table it opens All projects' Table; Clear filters switches it
  // off too, staying put.
  await open("#/p/alpha/table");
  await pill.click();
  await expect(page).toHaveURL(/#\/all\/table/);
  await expect(page.getByRole("row")).toHaveCount(2);
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(pill).toHaveAttribute("aria-pressed", "false");
  await expect(page).toHaveURL(/#\/all\/table/);

  // Choosing a project while it is on keeps it on there, and a press then
  // switches it off in place.
  await pill.click();
  await page
    .getByRole("button", { name: /^Alpha/ })
    .first()
    .click();
  await expect(page).toHaveURL(/#\/p\/alpha\//);
  await expect(pill).toHaveAttribute("aria-pressed", "true");
  await pill.click();
  await expect(pill).toHaveAttribute("aria-pressed", "false");
  await expect(page).toHaveURL(/#\/p\/alpha\//);

  // It is a lens, not a saved filter: a reload starts with it off.
  await open("#/p/alpha/board");
  await pill.click();
  await page.waitForTimeout(600);
  expect(
    await readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
  ).not.toContain("needs");
  await page.reload();
  await open("#/p/alpha/board");
  await expect(pill).toHaveAttribute("aria-pressed", "false");
  await expect(cards).toHaveCount(total);

  // Agent working is a State option; AL-3's run waits on a permission, so
  // it is not at work.
  const state = await filterMenu(page, "State");
  await state.getByRole("button", { name: "Agent working" }).click();
  await expect(filterButton(page, "State")).toHaveAccessibleName(
    "State: Agent working",
  );
  await expect(real).toHaveCount(0);
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(real).toBeVisible();
  await expectNoAxeViolations("board with runs");
  for (const theme of ["light", "dark"]) {
    await page.reload();
    await open("#/p/alpha/board", { theme });
    await expect(real).toBeInViewport();
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
  // Only the session with a plan shows one; the ended session recorded
  // none, so it has no plan section (FH-54).
  await expect(
    panel.getByRole("heading", { name: "Plan", level: 4 }),
  ).toHaveCount(1);
  await expect(panel.getByText(/No plan/)).toHaveCount(0);
  const scrolled = await page.evaluate(
    () => document.scrollingElement?.scrollTop ?? 0,
  );
  expect(scrolled).toBe(0);
  // Its three subagents hang beneath it; one opens to its own plan.
  const tree = panel.getByRole("list", { name: /^Subagents of/ });
  await expect(tree.getByRole("listitem")).toHaveCount(3);
  await expect(tree.getByRole("listitem").first()).toContainText("Ended");
  await expect(tree).toContainText("1/2 · Write the timeline test");
  const helper = tree.getByRole("button", { name: "general-purpose" });
  await helper.click();
  await expect(helper).toHaveAttribute("aria-expanded", "true");
  await expect(
    tree.getByRole("heading", { name: "Plan", level: 5 }),
  ).toBeVisible();
  await expect(
    tree.getByRole("heading", { name: /^Edited/, level: 5 }),
  ).toBeVisible();
  await expect(tree.getByText(/RunsTab\.test\.tsx$/).first()).toBeVisible();
  await expectNoAxeViolations("runs tab");
  await shot("runs-tab-1440-light");
  await helper.scrollIntoViewIfNeeded();
  await shot("runs-tab-subagents-1440-light");
  await open("#/p/alpha/board?t=AL-3", { theme: "dark" });
  await page.getByRole("tab", { name: /^Runs/ }).click();
  const darkHelper = panel
    .getByRole("list", { name: /^Subagents of/ })
    .getByRole("button", { name: "general-purpose" });
  // The row stays open across the theme change.
  await expect(darkHelper).toHaveAttribute("aria-expanded", "true");
  await expectNoAxeViolations("runs tab dark");
  await shot("runs-tab-1440-dark");
  await darkHelper.scrollIntoViewIfNeeded();
  await shot("runs-tab-subagents-1440-dark");
});

test("a question from an agent is answered on the board and delivered with its next prompt", async () => {
  // The gamma session asks through the real MCP server, as Claude Code would.
  const asked = await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "decision",
    text: "Keep the old project skeleton, or start from the new template?",
    options: ["Keep it", "New template"],
  });
  expect(asked).toContain("the answer will arrive in a later prompt");

  await open("#/p/alpha/board");
  const card = page.getByRole("button", {
    name: /^Project skeleton, AL-1, needs you: question waiting/,
  });
  await expect(card).toBeVisible();
  await card.click();
  const panel = page.getByRole("complementary", { name: "Ticket AL-1" });
  const question = panel.getByRole("region", { name: "Needs a decision" });
  await expect(question).toBeVisible();
  await expect(
    question.getByText(
      "Keep the old project skeleton, or start from the new template?",
    ),
  ).toBeVisible();
  await expect(question.getByText(/from claude:e2d8f6a4/)).toBeVisible();
  await expectNoAxeViolations("card panel with a question");
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(`card-question-1440-${theme}`);
  }

  // A choice fills the answer; sending it says it is on its way.
  await question.getByRole("button", { name: "Keep it" }).click();
  await expect(
    question.getByRole("button", { name: "Keep it" }),
  ).toHaveAttribute("aria-pressed", "true");
  await question.getByRole("button", { name: "Send answer" }).click();
  await expect(
    panel.getByRole("region", { name: "Answer waits for its next prompt" }),
  ).toContainText("“Keep it”");
  await expect(panel.getByText(/Answer \(Robin\): "Keep it"/)).toBeVisible();

  // The Agents view shows the session waiting for its next prompt.
  await open("#/all/agents");
  await expect(
    page
      .getByRole("region", { name: /^Needs you/ })
      .getByText("Answer waits for its next prompt"),
  ).toBeVisible();

  // The session's next prompt delivers the answer, once.
  const output = promptHook(sandbox.root, IDLE_SESSION, seeded.gamma);
  expect(output).toContain(
    'AL-1: \\"Keep the old project skeleton, or start from the new template?\\" → \\"Keep it\\" (Robin)',
  );
  expect(promptHook(sandbox.root, IDLE_SESSION, seeded.gamma)).toBe("");
  await open("#/p/alpha/board");
  await expect(card).toHaveCount(0);
});

test("a question is answered from the Agents view, and fits a phone", async () => {
  await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "question",
    text: "Ship the offline spike behind a flag?",
  });

  // At phone width the question card fits without sideways scrolling.
  await open("#/all/agents", { width: 390, height: 844 });
  const needsYou = page.getByRole("region", { name: /^Needs you/ });
  await needsYou.getByRole("button", { name: /e2d8f6a4/ }).click();
  const question = needsYou.getByRole("region", { name: "Has a question" });
  await expect(
    question.getByText("Ship the offline spike behind a flag?"),
  ).toBeVisible();
  const sideways = await page.evaluate(
    () => document.scrollingElement.scrollWidth - window.innerWidth,
  );
  expect(sideways).toBeLessThanOrEqual(0);
  const box = await question.boundingBox();
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(390);
  await expectNoAxeViolations("question card at phone width");
  await shot("agents-question-390-light");

  // Answering from the run's row records it like the panel does.
  await question.getByRole("textbox", { name: "Your answer" }).fill("Yes");
  await question.getByRole("button", { name: "Send answer" }).click();
  await expect(
    needsYou.getByRole("region", { name: "Answer waits for its next prompt" }),
  ).toContainText("“Yes”");
  const output = promptHook(sandbox.root, IDLE_SESSION, seeded.gamma);
  expect(output).toContain('\\"Yes\\" (Robin)');
});

test("a question answered in the session's own chat leaves Needs you", async () => {
  await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "decision",
    text: "Rename the skeleton package?",
    options: ["Rename", "Keep"],
  });
  await open("#/p/alpha/board");
  const card = page.getByRole("button", {
    name: /^Project skeleton, AL-1, needs you: question waiting/,
  });
  await expect(card).toBeVisible();

  // A background command finishing is not the user answering.
  promptHook(sandbox.root, IDLE_SESSION, seeded.gamma, TASK_NOTIFICATION);
  await page.reload();
  await open("#/p/alpha/board");
  await expect(card).toBeVisible();

  // The user replies in the session's chat instead of on the board.
  expect(promptHook(sandbox.root, IDLE_SESSION, seeded.gamma)).toBe("");
  await expect(card).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    await open("#/p/alpha/board", { theme });
    await expect(card).toHaveCount(0);
    await shot(`board-answered-in-session-1440-${theme}`);
  }

  // The run keeps the question in its activity, marked.
  for (const [width, theme] of [
    [1440, "light"],
    [1440, "dark"],
    [390, "light"],
  ]) {
    await open("#/all/agents", { width, height: 900, theme });
    const row = page.getByRole("button", { name: /e2d8f6a4/ }).first();
    if ((await row.getAttribute("aria-expanded")) !== "true") await row.click();
    const mark = page.getByText("Question on AL-1 answered in the session");
    await expect(mark).toBeVisible();
    await expect(
      page.getByRole("region", { name: /^Needs you/ }).getByRole("button", {
        name: /e2d8f6a4/,
      }),
    ).toHaveCount(0);
    await mark.scrollIntoViewIfNeeded();
    if (width === 1440 && theme === "light")
      await expectNoAxeViolations("run answered in the session");
    await shot(`agents-answered-in-session-${width}-${theme}`);
  }
});

test("the Needs you filter points to agents with no ticket (FH-44)", async () => {
  // The gamma session has no ticket; its question is about none either.
  await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    kind: "question",
    text: "Which repository should the offline spike live in?",
  });
  await open("#/p/alpha/board");
  const pill = page.getByRole("button", { name: "2 need you", exact: true });
  await pill.click();
  await expect(page).toHaveURL(/#\/all\/board/);
  const notice = page.getByRole("region", { name: "Agents without a ticket" });
  await expect(notice).toContainText(
    "1 agent needs you with no ticket on the board.",
  );
  // AL-3's card still shows; the gamma agent cannot.
  await expect(page.locator("[data-ticket]")).toHaveCount(1);
  await expectNoAxeViolations("needs you notice");
  for (const theme of ["light", "dark"]) {
    await open("#/all/board", { theme });
    await shot(`board-needs-you-unticketed-1440-${theme}`);
  }
  await open("#/all/board", { width: 390, height: 844 });
  await shot("board-needs-you-unticketed-390-light");
  await open("#/all/board");
  await notice.getByRole("button", { name: "Open Agents" }).click();
  await expect(page).toHaveURL(/#\/all\/agents/);
  await expect(
    page
      .getByRole("region", { name: /^Needs you/ })
      .getByRole("button", { name: /e2d8f6a4/ }),
  ).toBeVisible();

  // Answered in the session's own chat, it no longer needs you.
  promptHook(sandbox.root, IDLE_SESSION, seeded.gamma);
  await open("#/all/board");
  await expect(notice).toHaveCount(0);
  await page.getByRole("button", { name: "1 needs you", exact: true }).click();
  await expect(page).toHaveURL(/#\/p\/alpha\/board/);
});
