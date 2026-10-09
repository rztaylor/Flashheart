import { appendFile, mkdir, readFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import {
  callTool,
  IDLE_SESSION,
  promptHook,
  seedClaimedRun,
  seedRuns,
  TASK_NOTIFICATION,
} from "./agent-runs.mjs";
import { writeDemoBoard } from "./demo-board.mjs";
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
// exactly as Claude Code runs it, then read back by the board: the
// Overview, cards, the Runs tab and questions. The demo board gives the
// Overview realistic tickets; its sample projects carry the runs.
test.describe.configure({ mode: "serial" });

let sandbox;
let seeded;
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

const section = (name) => page.getByRole("region", { name, exact: true });

test("the Overview puts tickets first: decisions, review, risk, progress, then tiles (FH-51)", async () => {
  await open("#/all/overview");
  // The band's tabs: Agents is gone.
  await expect(
    page.getByRole("navigation", { name: "Views" }).getByRole("link"),
  ).toHaveText(["Board", "Overview", "Workstreams", "Table"]);
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
  await expectNoAxeViolations("overview light");

  for (const [width, height] of [
    [1280, 800],
    [1920, 1080],
  ]) {
    for (const theme of ["light", "dark"]) {
      await open("#/all/overview", { width, height, theme });
      await shot(`overview-${width}-${theme}`);
    }
  }
  // The whole page, for review against the approved concept.
  for (const theme of ["light", "dark"]) {
    await open("#/all/overview", { width: 1440, height: 2000, theme });
    await shot(`overview-full-1440-${theme}`);
  }
  await open("#/all/overview", { theme: "dark" });
  await expectNoAxeViolations("overview dark");
  // A phone stacks each row under its title; nothing scrolls sideways.
  for (const theme of ["light", "dark"]) {
    await open("#/all/overview", { width: 390, height: 844, theme });
    const overflow = await page.evaluate(
      () => document.scrollingElement.scrollWidth - window.innerWidth,
    );
    expect(overflow).toBeLessThanOrEqual(0);
    await shot(`overview-390-${theme}`);
    await open("#/all/overview", { width: 390, height: 3000, theme });
    await shot(`overview-full-390-${theme}`);
  }
  await expectNoAxeViolations("overview at phone width");

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
    await open("#/all/overview", { width: 1440, height: 2600, theme });
    await shot(`overview-tiles-open-1440-${theme}`);
  }
  await open("#/all/overview");
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
  await open("#/p/alpha/agents");
  await expect(page).toHaveURL(/#\/p\/alpha\/overview$/);
  await expect(page.getByRole("link", { name: "Overview" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(section("Needs your decision")).toBeVisible();
});

test("every Overview section has a calm empty state (FH-51)", async () => {
  for (const theme of ["light", "dark"]) {
    await open("#/p/quiet/overview", { theme });
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
    await expectNoAxeViolations(`empty overview ${theme}`);
    await shot(`overview-empty-1440-${theme}`);
    await open("#/p/quiet/overview", { width: 390, height: 1100, theme });
    await shot(`overview-empty-390-${theme}`);
  }
});

test("the band's Needs you pill opens the Board, filtered, from other views (FH-44)", async () => {
  await open("#/all/overview");
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
  // The Overview asks for the same decision, answerable in place.
  for (const theme of ["light", "dark"]) {
    await open("#/all/overview", { width: 1440, height: 2000, theme });
    await expect(
      section("Needs your decision").getByRole("region", {
        name: "Needs a decision",
      }),
    ).toBeVisible();
    await shot(`overview-question-full-1440-${theme}`);
  }
  await open("#/p/alpha/board?t=AL-1");

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
  await open("#/all/overview");
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
  await open("#/p/alpha/board");
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
  await open("#/all/overview", { width: 390, height: 844 });
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
  await expectNoAxeViolations("question card at phone width");
  await question.scrollIntoViewIfNeeded();
  await shot("overview-question-390-light");

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

  // The Overview no longer asks for it.
  for (const theme of ["light", "dark"]) {
    await open("#/all/overview", { theme });
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
  await open("#/all/board");
  await expect(notice).toHaveCount(0);
  await page.getByRole("button", { name: "1 needs you", exact: true }).click();
  await expect(page).toHaveURL(/#\/p\/alpha\/board/);
});
