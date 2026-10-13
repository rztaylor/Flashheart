import { appendFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import {
  callTool,
  connectMCP,
  IDLE_SESSION,
  promptHook,
  seedBranchRun,
  seedClaimedRun,
  seedCodexPair,
  seedRuns,
  TASK_NOTIFICATION,
} from "./agent-runs.mjs";
import { writeDemoBoard } from "./demo-board.mjs";
import {
  expectNoAxeViolations,
  filterButton,
  filterMenu,
  launch,
  makeSandbox,
  open,
  shot,
  stopIfRunning,
  waitForManualURL,
} from "./support.mjs";

// Agent runs end to end: sessions are recorded by `flashheart hook claude`,
// exactly as Claude Code runs it, then read back by the board: the
// Overview, cards, the Runs tab and questions. The demo board gives the
// Overview realistic tickets; its sample projects carry the runs.
test.describe.configure({ mode: "serial" });

let sandbox;
let seeded;
let claimed;
let server;
let context;
let page;

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  // A project with no tickets yet shows every Overview section empty.
  await mkdir(join(demo, "quiet", "tickets"), { recursive: true });
  // Answers are recorded under the configured name (FH-8).
  await appendFile(
    join(sandbox.root, ".flashheart", "config.yaml"),
    "user_name: Robin\n",
  );
  seeded = await seedRuns(sandbox.home, sandbox.root);
  claimed = await seedClaimedRun(sandbox.home, sandbox.root, "FH-25");
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

const section = (name) => page.getByRole("region", { name, exact: true });

test("the Overview puts tickets first: decisions, review, risk, progress, then tiles (FH-51)", async () => {
  await open(page, "#/all/overview");
  // The band's tabs, Overview first and Board last (FH-61); Agents is gone.
  await expect(
    page.getByRole("navigation", { name: "Views" }).getByRole("link"),
  ).toHaveText(["Overview", "Workstreams", "Table", "Board"]);
  await expect(page.getByRole("link", { name: "Overview" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  const sections = page.locator("[data-section]");
  await expect(sections).toHaveCount(6);
  expect(
    await sections.evaluateAll((all) =>
      all.map((item) => item.dataset.section),
    ),
  ).toEqual(["decision", "review", "risk", "progress", "unticketed", "upNext"]);

  // A permission prompt says where to answer it, and offers no button but
  // the ticket's title: Flashheart never grants permissions.
  const decision = section("Needs your decision");
  const prompt = decision.locator('[data-row="AL-3"]');
  await expect(prompt.getByText("Permission for Bash")).toBeVisible();
  await expect(
    prompt.getByText("Answer in the session · Alpha · feature/card-panel"),
  ).toBeVisible();
  await expect(prompt.getByRole("button")).toHaveText(["Card panel"]);
  await expect(
    prompt.getByRole("link", { name: "AL-3", exact: true }),
  ).toHaveAttribute("href", "#/ticket/AL-3");

  // In progress says the session's state and its subagents in words.
  const progress = section("In progress");
  await expect(
    progress
      .locator('[data-row="FH-25"]')
      .getByText("Working · subagents 1 done · 2 running"),
  ).toBeVisible();
  await expect(
    progress
      .locator('[data-row="AL-3"]')
      .getByText("Needs you · subagents 1 done · 2 running"),
  ).toBeVisible();

  // At risk gives its reasons in words, with no agent where no session
  // works on the ticket.
  const risk = section("At risk");
  await expect(
    risk.locator('[data-row="FH-10"]').getByText("No session has worked on it"),
  ).toBeVisible();
  await expect(
    risk
      .locator('[data-row="AL-4"]')
      .getByText("Top of Up next · blocked by AL-3"),
  ).toBeVisible();
  await expect(risk.getByText("Claude", { exact: true })).toHaveCount(0);
  await expectNoAxeViolations(page, "overview light");

  for (const [width, height] of [
    [1280, 800],
    [1920, 1080],
  ]) {
    for (const theme of ["light", "dark"]) {
      await open(page, "#/all/overview", { width, height, theme });
      await shot(page, `overview-${width}-${theme}`);
    }
  }
  // The whole page, for review against the approved concept.
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/overview", { width: 1440, height: 2000, theme });
    await shot(page, `overview-full-1440-${theme}`);
  }
  await open(page, "#/all/overview", { theme: "dark" });
  await expectNoAxeViolations(page, "overview dark");
  // A phone stacks each row under its title; nothing scrolls sideways.
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/overview", { width: 390, height: 844, theme });
    const overflow = await page.evaluate(
      () => document.scrollingElement.scrollWidth - window.innerWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
    await shot(page, `overview-390-${theme}`);
    await open(page, "#/all/overview", { width: 390, height: 3000, theme });
    await shot(page, `overview-full-390-${theme}`);
  }
  await expectNoAxeViolations(page, "overview at phone width");

  // Review results opens the ticket on its Review tab.
  const review = section("Ready for your review");
  await review.getByRole("button", { name: "Review results of AL-2" }).click();
  const panel = page.getByRole("complementary", { name: "Ticket AL-2" });
  await expect(panel.getByRole("tab", { name: "Review" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.keyboard.press("Escape");
  await expect(panel).toHaveCount(0);

  // The tiles open: sessions with no ticket, each with Create ticket.
  const unticketed = section("Work with no ticket");
  const tile = unticketed.getByRole("button", { name: /Work with no ticket/ });
  await expect(tile).toHaveAttribute("aria-expanded", "false");
  await tile.click();
  await expect(tile).toHaveAttribute("aria-expanded", "true");
  const beta = unticketed
    .getByRole("listitem")
    .filter({ hasText: "beta · main" });
  await expect(beta.getByText("Session b7c4e9f2")).toBeVisible();
  const upNext = section("Up next");
  await upNext.getByRole("button", { name: /Up next/ }).click();
  await expect(upNext.locator('[data-row="AL-4"]')).toBeVisible();
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/overview", { width: 1440, height: 2600, theme });
    await shot(page, `overview-tiles-open-1440-${theme}`);
  }
  await open(page, "#/all/overview");
  await beta.getByRole("button", { name: "Create ticket" }).click();
  const dialog = page.getByRole("dialog", { name: "New ticket" });
  // The new ticket goes to the session's project.
  await dialog.getByRole("textbox", { name: "Title" }).fill("Greeting flow");
  await dialog.getByRole("button", { name: "Create ticket" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(
    page.getByRole("complementary", { name: "Ticket BE-2" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: "Dismiss" }).click();

  // An old Agents route lands on the Overview.
  await open(page, "#/p/alpha/agents");
  await expect(page).toHaveURL(/#\/p\/alpha\/overview$/);
  await expect(page.getByRole("link", { name: "Overview" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(section("Needs your decision")).toBeVisible();
});

test("every Overview section has a calm empty state (FH-51)", async () => {
  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/quiet/overview", { theme });
    for (const sentence of [
      "Nothing needs your decision.",
      "Nothing is waiting for your review.",
      "Nothing is at risk.",
      "Nothing is in progress.",
      "Every live session has a ticket.",
      "Up next is empty.",
    ])
      await expect(page.getByText(sentence)).toBeVisible();
    // Empty sections have nothing to open.
    await expect(page.locator("main [aria-expanded]")).toHaveCount(0);
    await expectNoAxeViolations(page, `empty overview ${theme}`);
    await shot(page, `overview-empty-1440-${theme}`);
    await open(page, "#/p/quiet/overview", { width: 390, height: 1100, theme });
    await shot(page, `overview-empty-390-${theme}`);
  }
});

test("the band's Needs you pill opens the Board, filtered, from other views (FH-44)", async () => {
  await open(page, "#/all/overview");
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
  await open(page, "#/p/alpha/board");
  const real = page
    .getByRole("region", { name: /^In progress/ })
    .getByRole("button", { name: /^Card panel, AL-3, Claude needs you/ });
  await expect(real).toBeVisible();
  // The card says what the run needs on a line of its own and keeps its
  // plan count on the meta line, the step in the mark's tooltip (FH-147);
  // the card's button covers its drawing, which sits beside it.
  const face = real.locator("xpath=..");
  await expect(face.getByText("Permission for Bash")).toBeVisible();
  const run = face.locator('[data-mark="live"]');
  await expect(run).toHaveText("2/5");
  await expect(run).toHaveAttribute("title", /plan 2 of 5: Runs tab timeline/);
  await expect(real).toHaveAccessibleName(/plan 2 of 5: Runs tab timeline/);

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
    await open(page, "#/all/board", { theme });
    await shot(page, `board-needs-you-1440-${theme}`);
  }
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/board", { width: 390, height: 844, theme });
    await expect(
      page.getByRole("button", { name: "1 agent needs you", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    await shot(page, `board-needs-you-390-${theme}`);
  }
  await open(page, "#/all/board");

  // A second press goes back to the project it was pressed in.
  await pill.click();
  await expect(page).toHaveURL(/#\/p\/alpha\/board/);
  await expect(pill).toHaveAttribute("aria-pressed", "false");
  await expect(cards).toHaveCount(total);

  // On the Table it opens All projects' Table; Clear filters switches it
  // off too, staying put.
  await open(page, "#/p/alpha/table");
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
  await open(page, "#/p/alpha/board");
  await pill.click();
  await page.waitForTimeout(600);
  expect(
    await readFile(join(sandbox.root, ".flashheart", "config.yaml"), "utf8"),
  ).not.toContain("needs");
  await page.reload();
  await open(page, "#/p/alpha/board");
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
  await expectNoAxeViolations(page, "board with runs");
  for (const theme of ["light", "dark"]) {
    await page.reload();
    await open(page, "#/p/alpha/board", { theme });
    await expect(real).toBeInViewport();
    await shot(page, `board-runs-1440-${theme}`);
  }

  // The Runs tab shows the ticket's runs and the handoff warning.
  await open(page, "#/p/alpha/board");
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
  await expectNoAxeViolations(page, "runs tab");
  await shot(page, "runs-tab-1440-light");
  await helper.scrollIntoViewIfNeeded();
  await shot(page, "runs-tab-subagents-1440-light");
  await open(page, "#/p/alpha/board?t=AL-3", { theme: "dark" });
  await page.getByRole("tab", { name: /^Runs/ }).click();
  const darkHelper = panel
    .getByRole("list", { name: /^Subagents of/ })
    .getByRole("button", { name: "general-purpose" });
  // The row stays open across the theme change.
  await expect(darkHelper).toHaveAttribute("aria-expanded", "true");
  await expectNoAxeViolations(page, "runs tab dark");
  await shot(page, "runs-tab-1440-dark");
  await darkHelper.scrollIntoViewIfNeeded();
  await shot(page, "runs-tab-subagents-1440-dark");
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

  await open(page, "#/p/alpha/board");
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
  await expectNoAxeViolations(page, "card panel with a question");
  for (const theme of ["light", "dark"]) {
    await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
    await shot(page, `card-question-1440-${theme}`);
  }
  // The Overview asks for the same decision, answerable in place.
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/overview", { width: 1440, height: 2000, theme });
    await expect(
      section("Needs your decision").getByRole("region", {
        name: "Needs a decision",
      }),
    ).toBeVisible();
    await shot(page, `overview-question-full-1440-${theme}`);
  }
  await open(page, "#/p/alpha/board?t=AL-1");

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

  // The Overview keeps it under Needs your decision until the session
  // takes the answer.
  await open(page, "#/all/overview");
  await expect(
    section("Needs your decision").getByRole("region", {
      name: "Answer waits for its next prompt",
    }),
  ).toContainText("“Keep it”");

  // The session's next prompt delivers the answer, once.
  const output = promptHook(sandbox.root, IDLE_SESSION, seeded.gamma);
  expect(output).toContain(
    'AL-1: \\"Keep the old project skeleton, or start from the new template?\\" → \\"Keep it\\" (Robin)',
  );
  expect(promptHook(sandbox.root, IDLE_SESSION, seeded.gamma)).toBe("");
  await open(page, "#/p/alpha/board");
  await expect(card).toHaveCount(0);
});

test("a question is answered from the Overview, and fits a phone (FH-51)", async () => {
  await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "question",
    text: "Ship the offline spike behind a flag?",
  });

  // At phone width the question card fits without sideways scrolling.
  await open(page, "#/all/overview", { width: 390, height: 844 });
  const decision = section("Needs your decision");
  const question = decision.getByRole("region", { name: "Has a question" });
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
  await expectNoAxeViolations(page, "question card at phone width");
  await question.scrollIntoViewIfNeeded();
  await shot(page, "overview-question-390-light");

  // Answering in place records it like the panel does.
  await question.getByRole("textbox", { name: "Your answer" }).fill("Yes");
  await question.getByRole("button", { name: "Send answer" }).click();
  await expect(
    decision.getByRole("region", { name: "Answer waits for its next prompt" }),
  ).toContainText("“Yes”");
  const output = promptHook(sandbox.root, IDLE_SESSION, seeded.gamma);
  expect(output).toContain('\\"Yes\\" (Robin)');
  // Delivered, it leaves Needs your decision.
  await expect(decision.getByText("Ship the offline spike")).toHaveCount(0);
});

test("a question answered in the session's own chat leaves Needs you", async () => {
  await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "decision",
    text: "Rename the skeleton package?",
    options: ["Rename", "Keep"],
  });
  await open(page, "#/p/alpha/board");
  const card = page.getByRole("button", {
    name: /^Project skeleton, AL-1, needs you: question waiting/,
  });
  await expect(card).toBeVisible();

  // A background command finishing is not the user answering.
  promptHook(sandbox.root, IDLE_SESSION, seeded.gamma, TASK_NOTIFICATION);
  await page.reload();
  await open(page, "#/p/alpha/board");
  await expect(card).toBeVisible();

  // The user replies in the session's chat instead of on the board.
  expect(promptHook(sandbox.root, IDLE_SESSION, seeded.gamma)).toBe("");
  await expect(card).toHaveCount(0);
  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/alpha/board", { theme });
    await expect(card).toHaveCount(0);
    await shot(page, `board-answered-in-session-1440-${theme}`);
  }

  // The Overview no longer asks for it.
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/overview", { theme });
    await expect(
      section("Needs your decision").getByText("Rename the skeleton package?"),
    ).toHaveCount(0);
  }
});

test("the Needs you filter points to agents with no ticket (FH-44)", async () => {
  // The gamma session has no ticket; its question is about none either.
  await callTool(sandbox.root, seeded.gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    kind: "question",
    text: "Which repository should the offline spike live in?",
  });
  await open(page, "#/p/alpha/board");
  const pill = page.getByRole("button", { name: "2 need you", exact: true });
  await pill.click();
  await expect(page).toHaveURL(/#\/all\/board/);
  const notice = page.getByRole("region", { name: "Agents without a ticket" });
  await expect(notice).toContainText(
    "1 agent needs you with no ticket on the board.",
  );
  // AL-3's card still shows; the gamma agent cannot.
  await expect(page.locator("[data-ticket]")).toHaveCount(1);
  await expectNoAxeViolations(page, "needs you notice");
  for (const theme of ["light", "dark"]) {
    await open(page, "#/all/board", { theme });
    await shot(page, `board-needs-you-unticketed-1440-${theme}`);
  }
  await open(page, "#/all/board", { width: 390, height: 844 });
  await shot(page, "board-needs-you-unticketed-390-light");
  await open(page, "#/all/board");
  await notice.getByRole("button", { name: "Open Overview" }).click();
  await expect(page).toHaveURL(/#\/all\/overview/);
  await expect(
    section("Needs your decision").getByText(
      "Which repository should the offline spike live in?",
    ),
  ).toBeVisible();
  await expect(
    section("Needs your decision").getByText("Session e2d8f6a4"),
  ).toBeVisible();

  // Answered in the session's own chat, it no longer needs you.
  promptHook(sandbox.root, IDLE_SESSION, seeded.gamma);
  await open(page, "#/all/board");
  await expect(notice).toHaveCount(0);
  await page.getByRole("button", { name: "1 needs you", exact: true }).click();
  await expect(page).toHaveURL(/#\/p\/alpha\/board/);
});

test("the Overview's pipeline counts every column of the scope (FH-63)", async () => {
  await open(page, "#/all/overview");
  const pipeline = page.getByRole("navigation", { name: "Pipeline" });
  const stages = pipeline.getByRole("link");
  await expect(stages).toHaveText([
    /^\d+Backlog$/,
    /^\d+Up next$/,
    /^\d+In progress$/,
    /^\d+Ready to review$/,
    /^\d+Done$/,
  ]);
  const counts = () =>
    stages.evaluateAll((links) =>
      links.map((link) =>
        Number(link.getAttribute("aria-label").match(/: (\d+) tickets?$/)[1]),
      ),
    );
  const before = await counts();

  // An agent creates a ticket, then moves it to review, through the real
  // MCP server: it leaves Backlog for Ready to review.
  const created = await callTool(sandbox.root, claimed.cwd, "create_ticket", {
    run: claimed.run,
    type: "feature",
    title: "Pipeline headline",
    description: "Count every column.",
    criteria: ["Counts"],
    priority: "medium",
  });
  const id = created.match(/ticket=(\S+)/)[1];
  await expect.poll(counts).toEqual([before[0] + 1, ...before.slice(1)]);
  await callTool(sandbox.root, claimed.cwd, "move", {
    run: claimed.run,
    ticket: id,
    to: "review",
  });
  await expect
    .poll(counts)
    .toEqual([before[0], before[1], before[2], before[3] + 1, before[4]]);

  // The stages are interlocking chevrons in their state colours.
  const review = pipeline.locator('[data-stage="review"]');
  await expect(review).toHaveCSS("clip-path", /^polygon/);
  await expect(review).toHaveAttribute("data-tone", "review");
  await expectNoAxeViolations(page, "pipeline");
  for (const theme of ["light", "dark"]) {
    for (const width of [1280, 1920]) {
      await open(page, "#/all/overview", { width, theme });
      await shot(page, `overview-pipeline-${width}-${theme}`);
    }
    // At phone width the five stages stay on one row with no page scroll.
    await open(page, "#/all/overview", { width: 390, height: 844, theme });
    const overflow = await page.evaluate(
      () => document.scrollingElement.scrollWidth - window.innerWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
    const tops = await stages.evaluateAll((links) =>
      links.map((link) => link.getBoundingClientRect().top),
    );
    expect(new Set(tops).size).toBe(1);
    await shot(page, `overview-pipeline-390-${theme}`);
  }

  // One project counts only its own tickets, and leads to its own board.
  await open(page, "#/p/quiet/overview");
  await expect(stages).toHaveText([
    "0Backlog",
    "0Up next",
    "0In progress",
    "0Ready to review",
    "0Done",
  ]);
  await pipeline
    .getByRole("link", { name: "Ready to review: 0 tickets" })
    .click();
  await expect(page).toHaveURL(/#\/p\/quiet\/board$/);
});

test("an agent without hooks gets a run from its first write, and its answers with its next call (FH-15)", async () => {
  // Codex without Flashheart's hooks, in a real `flashheart mcp` session.
  const codex = await connectMCP(sandbox.root, seeded.beta, "codex-mcp-client");
  try {
    expect(
      await codex.call("claim", {
        ticket: "BE-1",
        force: true,
        reason: "synthetic: its dependency is nearly done",
      }),
    ).toContain("ok ticket=BE-1 column=in-progress");
    const asked = await codex.call("ask_human", {
      ticket: "BE-1",
      kind: "question",
      text: "Greet in English only?",
    });
    expect(asked).toContain(
      "with the result of your next flashheart tool call",
    );

    // The Overview asks for the decision, naming the session MCP only.
    const decision = section("Needs your decision");
    const question = decision.getByRole("region", { name: "Has a question" });
    // Screenshots at the project's widths, both themes, and a phone.
    const sizes = [
      [1280, 800, "light"],
      [1280, 800, "dark"],
      [1920, 1080, "light"],
      [1920, 1080, "dark"],
      [390, 844, "light"],
    ];
    for (const [width, height, theme] of sizes) {
      await open(page, "#/all/overview", { width, height, theme });
      await expect(question.getByText("Greet in English only?")).toBeVisible();
      await expect(
        decision.getByText("MCP only", { exact: true }),
      ).toBeVisible();
      await expect(
        question.getByText(
          "It reaches the session with its next Flashheart tool call.",
        ),
      ).toBeVisible();
      if (width === 1280 && theme === "light")
        await expectNoAxeViolations(page, "question from a run without hooks");
      await question.scrollIntoViewIfNeeded();
      await shot(page, `overview-mcp-only-${width}-${theme}`);
    }

    // The ticket's Runs tab shows the run and what it did.
    await open(page, "#/p/beta/board");
    await page.getByRole("button", { name: /^Hello, BE-1/ }).click();
    await page.getByRole("tab", { name: /^Runs/ }).click();
    const panel = page.getByRole("complementary", { name: "Ticket BE-1" });
    await expect(panel.getByText("Codex (MCP only)")).toBeVisible();
    await expect(
      panel.getByText("Connected without hooks (MCP only)"),
    ).toBeVisible();
    // Its claim moved the ticket, named in plain words (FH-70).
    await expect(panel.getByText("Claimed BE-1")).toBeVisible();
    await expect(panel.getByText("Moved BE-1 to In progress")).toBeVisible();
    await expect(panel.getByText(/ticket\.moved/)).toHaveCount(0);
    for (const [width, height, theme] of sizes.slice(0, 4)) {
      await page.setViewportSize({ width, height });
      await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
      await expect(panel.getByText("Codex (MCP only)")).toBeVisible();
      await shot(page, `runs-tab-mcp-only-${width}-${theme}`);
    }

    // Answered on the Overview, the answer reaches the session with its
    // next tool call, once.
    await open(page, "#/all/overview");
    await question.getByRole("textbox", { name: "Your answer" }).fill("Yes");
    await question.getByRole("button", { name: "Send answer" }).click();
    await expect(
      decision.getByRole("region", {
        name: "Answer waits for its next tool call",
      }),
    ).toContainText("“Yes”");
    expect(await codex.call("list_tickets", {})).toContain(
      'BE-1: "Greet in English only?" → "Yes" (Robin)',
    );
    expect(await codex.call("list_tickets", {})).not.toContain(
      "Greet in English",
    );
    await expect(decision.getByText("Greet in English only?")).toHaveCount(0);

    // Its agent switches branches: the run follows, and its Activity says
    // so once, not as a second connection (FH-71).
    const head = join(seeded.beta, ".git", "HEAD");
    await writeFile(head, "ref: refs/heads/feature/next\n");
    try {
      await codex.call("list_tickets", {});
      await open(page, "#/p/beta/board", { width: 1280, height: 800 });
      await page.getByRole("button", { name: /^Hello, BE-1/ }).click();
      await page.getByRole("tab", { name: /^Runs/ }).click();
      await expect(
        panel.getByText("Switched to branch feature/next"),
      ).toBeVisible();
      await expect(
        panel.getByText("Connected without hooks (MCP only)"),
      ).toHaveCount(1);
      await panel
        .getByText("Switched to branch feature/next")
        .scrollIntoViewIfNeeded();
      await shot(page, "runs-tab-mcp-branch-switch-1280-light");
    } finally {
      await writeFile(head, "ref: refs/heads/main\n");
    }
  } finally {
    await codex.close();
  }
  // Closing the session ends its run.
  await page.reload();
  await open(page, "#/p/beta/board");
  await page.getByRole("button", { name: /^Hello, BE-1/ }).click();
  await page.getByRole("tab", { name: /^Runs/ }).click();
  await expect(
    page
      .getByRole("complementary", { name: "Ticket BE-1" })
      .getByText("Ended (disconnected)"),
  ).toBeVisible();
});

// tailOf says, for each element showing text, whether the box that clips
// it cuts the text, whether its first and last characters lie inside that
// box, and the full value its title keeps.
function tailOf(locator) {
  return locator.evaluateAll((elements) =>
    elements.map((element) => {
      let clip = element;
      while (clip && getComputedStyle(clip).overflowX === "visible")
        clip = clip.parentElement;
      const box = clip.getBoundingClientRect();
      const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
      const text = walker.nextNode();
      const range = document.createRange();
      const inside = (start) => {
        range.setStart(text, start);
        range.setEnd(text, start + 1);
        const glyph = range.getBoundingClientRect();
        return glyph.left >= box.left - 0.5 && glyph.right <= box.right + 0.5;
      };
      return {
        cut: clip.scrollWidth > clip.clientWidth,
        first: inside(0),
        last: inside(text.length - 1),
        title: element.closest("[title]")?.getAttribute("title"),
      };
    }),
  );
}

test("a long branch and a long edited path keep their tail visible in the panel and the Runs tab (FH-83, FH-117)", async () => {
  const branch =
    "feature/a-very-long-branch-name-that-overflows-the-panel-fh83-tail";
  const file = join(
    sandbox.root,
    "flashheart",
    "tickets",
    "FH-26-slow-index-nfs",
    "FH-26-slow-index-nfs.md",
  );
  // A run links by branch to a ticket in progress on it.
  const ticket = await readFile(file, "utf8");
  await writeFile(
    file,
    ticket
      .replace(/^status: .*$/m, "status: in-progress")
      .replace(/^branch: .*$/m, `branch: ${branch}`),
  );
  // The run edits a file under a deep path; runs show it relative to the checkout.
  const edited =
    "frontend/src/features/card/a-deeply-nested-folder/of-many-parts/RunDetailEditedFile.tsx";
  await seedBranchRun(sandbox.home, sandbox.root, "flashheart", branch, [
    edited,
  ]);

  const panel = page.getByRole("complementary", { name: "Ticket FH-26" });
  const shown = panel.getByText(branch, { exact: true });
  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/flashheart/board?t=FH-26", {
      width: 1280,
      height: 800,
      theme,
    });
    await page.getByRole("tab", { name: /^Runs/ }).click();
    await expect(panel.getByText("linked by branch")).toBeVisible();
    // The panel header, the run's card and its open detail show it, each
    // cut from the start so its distinguishing tail stays in view, the
    // full name in its title.
    await expect(shown).toHaveCount(3);
    for (const where of await tailOf(shown))
      expect(where).toEqual({
        cut: true,
        first: false,
        last: true,
        title: branch,
      });
    // The run's edited files are cut from the start too, so the file name
    // stays in view (FH-117).
    const file = panel.getByText(edited, { exact: true });
    await expect(file).toHaveCount(1);
    expect(await tailOf(file)).toEqual([
      { cut: true, first: false, last: true, title: edited },
    ]);
    if (theme === "light") await expectNoAxeViolations(page, "long branch");
    await shot(page, `branch-tail-runs-1280-${theme}`);
  }
});

test("two Codex sessions started in the same minute have their own short ids (FH-122)", async () => {
  // Codex's session ids start with the time; the short id is their end.
  const cwd = await seedCodexPair(sandbox.home, sandbox.root);

  // The Overview lists both sessions, which have no ticket, apart.
  const unticketed = section("Work with no ticket");
  const rows = unticketed
    .getByRole("listitem")
    .filter({ hasText: "feature/codex-pair" });
  for (const theme of ["light", "dark"]) {
    await open(page, "#/p/flashheart/overview", {
      width: 1440,
      height: 1400,
      theme,
    });
    const tile = unticketed.getByRole("button", {
      name: /Work with no ticket/,
    });
    if ((await tile.getAttribute("aria-expanded")) === "false")
      await tile.click();
    await expect(rows).toHaveCount(2);
    await expect(rows.getByText("Session 00000001")).toBeVisible();
    await expect(rows.getByText("Session 000000ff")).toBeVisible();
    await expect(page.getByText("Session 019a7c00")).toHaveCount(0);
    await rows.first().scrollIntoViewIfNeeded();
    await shot(page, `codex-pair-overview-1440-${theme}`);
  }

  // The run id each recovery note gives names that one session.
  const created = await callTool(sandbox.root, cwd, "create_ticket", {
    title: "Two Codex sessions side by side",
    type: "bug",
    priority: "medium",
    description: "Synthetic ticket for two Codex sessions.",
    criteria: ["Each session shows its own short id"],
    run: "codex:00000001",
  });
  const ticket = /ticket=(FH-\d+)/.exec(created)[1];
  await callTool(sandbox.root, cwd, "claim", {
    ticket,
    run: "codex:00000001",
  });

  // The card's Runs tab shows the holder and the other session on its
  // branch, each by its own short id.
  const panel = page.getByRole("complementary", { name: `Ticket ${ticket}` });
  for (const theme of ["light", "dark"]) {
    await open(page, `#/p/flashheart/board?t=${ticket}`, {
      width: 1280,
      height: 900,
      theme,
    });
    await page.getByRole("tab", { name: /^Runs/ }).click();
    await expect(panel.getByText("00000001", { exact: true })).toBeVisible();
    await expect(panel.getByText("000000ff", { exact: true })).toBeVisible();
    await expect(panel.getByText("019a7c00", { exact: true })).toHaveCount(0);
    if (theme === "light") await expectNoAxeViolations(page, "codex pair");
    await shot(page, `codex-pair-runs-1280-${theme}`);
  }
});
