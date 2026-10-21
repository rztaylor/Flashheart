import { mkdir, utimes, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { expect, test } from "@playwright/test";

import { callTool, hook, repo, session } from "../agent-runs.mjs";
import {
  launch,
  makeSandbox,
  open,
  projectRoot,
  stopIfRunning,
  waitForManualURL,
} from "../support.mjs";
import {
  checkShowcaseBoard,
  RUNS,
  reviewMarkdown,
  writeShowcaseBoard,
} from "./showcase-board.mjs";

// The README's screenshots (FH-180) of the showcase board, with live
// sessions recorded through the real hooks and MCP server. They are written
// to docs/images (or FLASHHEART_README_IMAGES); scripts/readme-screenshots.sh
// runs this and optimises them.
const images =
  process.env.FLASHHEART_README_IMAGES || resolve(projectRoot, "docs/images");
const EVIDENCE = "20261021T0940-question-on-the-overview.png";

test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;
let folders;

// seedSessions records the sessions the board shows: two Claude Code
// sessions working (one with subagents), one waiting on a question for you,
// a Codex session on a ticket's branch, and one in a second project.
async function seedSessions(root, checkouts) {
  const lanes = session(root, RUNS.lanes, checkouts.lanes);
  lanes.send("SessionStart", { source: "resume" });
  lanes.send("UserPromptSubmit", { prompt: "synthetic" });
  await callTool(root, checkouts.lanes, "claim", {
    ticket: "FH-11",
    run: `claude:${RUNS.lanes}`,
  });
  const file = (path) => ({ file_path: join(checkouts.lanes, path) });
  lanes.tool("Read", file("frontend/src/model/overview.ts"));
  lanes.tool("Edit", file("frontend/src/model/overview.ts"));
  lanes.tool("Edit", file("frontend/src/features/overview/InProgress.tsx"));
  const sub = (id, type) => ({ agent_id: id, agent_type: type });
  lanes.send("SubagentStart", sub("e1f2a3b4", "Explore"));
  lanes.tool("Grep", {}, sub("e1f2a3b4", "Explore"));
  lanes.send("SubagentStop", sub("e1f2a3b4", "Explore"));
  lanes.send("SubagentStart", sub("e2f3a4b5", "general-purpose"));
  lanes.tool(
    "Edit",
    file("frontend/src/model/overview.test.ts"),
    sub("e2f3a4b5", "general-purpose"),
  );
  lanes.send("SubagentStart", sub("e3f4a5b6", "Explore"));
  lanes.tool("Read", {}, sub("e3f4a5b6", "Explore"));

  const paste = session(root, RUNS.paste, checkouts.paste);
  paste.send("SessionStart", { source: "startup" });
  paste.send("UserPromptSubmit", { prompt: "synthetic" });
  await callTool(root, checkouts.paste, "claim", {
    ticket: "FH-19",
    run: `claude:${RUNS.paste}`,
  });
  paste.tool("Edit", {
    file_path: join(checkouts.paste, "frontend/src/features/editor/paste.ts"),
  });
  await callTool(root, checkouts.paste, "ask_human", {
    run: `claude:${RUNS.paste}`,
    ticket: "FH-19",
    kind: "decision",
    text: "Screenshots pasted from some apps arrive as JPEG. Convert them to PNG so every screenshot is stored the same way, or keep them as they came?",
    options: ["Convert to PNG", "Keep as they came"],
  });

  const codex = (event, extra = {}) =>
    hook(
      root,
      event,
      { session_id: RUNS.railway, cwd: checkouts.railway, ...extra },
      "codex",
    );
  codex("SessionStart", { source: "startup" });
  codex("UserPromptSubmit", {
    turn_id: "019a8f10-2c4e-7a61-9b3d-5e7f1a2c4d70",
    prompt: "synthetic",
  });
  codex("PostToolUse", {
    turn_id: "019a8f10-2c4e-7a61-9b3d-5e7f1a2c4d70",
    tool_name: "apply_patch",
    tool_input: {
      command: `*** Begin Patch\n*** Update File: ${join(checkouts.railway, "frontend/src/features/workstreams/graph.ts")}\n*** End Patch`,
    },
    tool_response: "Success.",
    tool_use_id: "exec-1",
  });

  const tides = session(root, RUNS.tides, checkouts.tides);
  tides.send("SessionStart", { source: "startup" });
  tides.send("UserPromptSubmit", { prompt: "synthetic" });
  await callTool(root, checkouts.tides, "claim", {
    ticket: "TL-2",
    run: `claude:${RUNS.tides}`,
  });
  tides.tool("Edit", {
    file_path: join(checkouts.tides, "app/src/tides/cache.ts"),
  });
}

test.beforeAll(async ({ browser }) => {
  checkShowcaseBoard();
  sandbox = await makeSandbox();
  const root = join(sandbox.home, "Kanban");
  const checkouts = {
    lanes: await repo(sandbox.home, "flashheart", "feature/overview-lanes"),
    paste: await repo(
      sandbox.home,
      "flashheart",
      "feature/paste-screenshots",
      "worktrees/paste",
    ),
    railway: await repo(
      sandbox.home,
      "flashheart",
      "feature/branch-and-join",
      "worktrees/railway",
    ),
    tides: await repo(sandbox.home, "tideline", "feature/offline-week"),
  };
  folders = await writeShowcaseBoard(root, {
    repos: {
      flashheart: [checkouts.lanes, checkouts.paste, checkouts.railway],
      tideline: [checkouts.tides],
      dotfiles: [join(sandbox.home, "src", "dotfiles")],
    },
  });
  await seedSessions(root, checkouts);
  sandbox.root = root;
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext({
    locale: "en-GB",
    timezoneId: "UTC",
    deviceScaleFactor: 2,
  });
  page = await context.newPage();
  await page.goto(url);
  await page.waitForURL((current) => current.pathname === "/");
  await mkdir(images, { recursive: true });
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

// capture shows hash at a size and theme, waits for shown and for loading
// to finish, and saves the page at rest: no focus ring, hover, caret or
// animation. end gives the height, in CSS pixels, the image is cut at.
async function capture(
  name,
  hash,
  shown,
  { width = 1440, height = 900, theme = "light", end } = {},
) {
  await open(page, hash, { width, height, theme });
  await expect(shown(page)).toBeVisible();
  await expect(page.locator("[data-skeleton]")).toHaveCount(0);
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement)
      document.activeElement.blur();
  });
  await page.mouse.move(width - 1, height - 1);
  await page.waitForTimeout(300);
  const clip = end
    ? { x: 0, y: 0, width, height: Math.min(height, await end(page)) }
    : undefined;
  await page.screenshot({
    path: join(images, `${name}.png`),
    animations: "disabled",
    caret: "hide",
    clip,
  });
}

const heading = (page) => page.locator("h1").first();

// under ends an image margin pixels below what locate finds; above ends it
// margin pixels above.
const under = (locate, margin) => async (page) => {
  const box = await locate(page).boundingBox();
  return Math.ceil(box.y + box.height + margin);
};
const above = (locate, margin) => async (page) => {
  const box = await locate(page).boundingBox();
  return Math.floor(box.y - margin);
};

// The question on the Overview, taken first: it is FH-10's review evidence,
// at the window width that makes its section closest to the review
// thumbnail's 16:9, so the thumbnail crops little of it.
test("the question, as FH-10's evidence", async () => {
  await open(page, "#/all/overview");
  const section = page
    .getByRole("heading", { name: "Needs your decision" })
    .locator("xpath=ancestor::section[1]");
  await expect(
    section.getByRole("region", { name: "Needs a decision" }),
  ).toBeVisible();
  let best;
  for (const width of [1040, 980, 920, 860, 800, 760]) {
    await page.setViewportSize({ width, height: 900 });
    const box = await section.boundingBox();
    const off = Math.abs(box.width / box.height - 16 / 9);
    if (!best || off < best.off) best = { width, off };
  }
  await page.setViewportSize({ width: best.width, height: 900 });
  await page.mouse.move(0, 0);
  const folder = folders.flashheart.get("FH-10");
  const dir = join(sandbox.root, "flashheart", "tickets", folder);
  await mkdir(join(dir, "files"), { recursive: true });
  await section.screenshot({
    path: join(dir, "files", EVIDENCE),
    animations: "disabled",
  });
  await writeFile(
    join(dir, "files", "index.yaml"),
    [
      `- file: ${EVIDENCE}`,
      "  caption: A question waiting on the Overview",
      "  kind: screenshot",
      `  run: claude:${RUNS.review}`,
      "  added: 2026-10-21T09:40:12Z",
      "",
    ].join("\n"),
  );
  await writeFile(join(dir, "review.md"), reviewMarkdown(folder, EVIDENCE));
  const earlier = new Date(Date.now() - 20 * 3_600_000);
  for (const path of [
    join(dir, "files", EVIDENCE),
    join(dir, "files", "index.yaml"),
    join(dir, "files"),
    join(dir, "review.md"),
    dir,
  ])
    await utimes(path, earlier, earlier);
});

const backlog = (page) => page.getByRole("region", { name: "Backlog" });

test("the board, light", async () => {
  await capture("board", "#/p/flashheart/board", backlog, { height: 960 });
});

test("the board, dark", async () => {
  await capture("board-dark", "#/p/flashheart/board", backlog, {
    height: 960,
    theme: "dark",
  });
});

test("a question waiting in the card panel", async () => {
  await capture(
    "question",
    "#/p/flashheart/board?t=FH-19",
    (page) =>
      page
        .getByRole("complementary", { name: "Ticket FH-19" })
        .getByRole("region", { name: "Needs a decision" }),
    { height: 960 },
  );
});

// The Workstreams view down to its third line: a running line's graph, a
// finished one folded away and one waiting for two others.
test("the workstreams", async () => {
  await capture("workstreams", "#/p/flashheart/workstreams", heading, {
    height: 2000,
    end: under((page) => page.locator("article").nth(2), 12),
  });
});

test("a workstream's page", async () => {
  await capture("workstream", "#/workstream/flashheart/editing", heading, {
    height: 1500,
    end: under((page) => page.getByText(/^The files stay the source/), 48),
  });
});

test("the Overview of every project", async () => {
  // Down to the sessions In progress, above Work with no ticket.
  await capture("overview", "#/all/overview", heading, {
    height: 2400,
    end: above(
      (page) =>
        page
          .getByRole("heading", { name: "Work with no ticket" })
          .locator("xpath=ancestor::section[1]"),
      4,
    ),
  });
});

test("a ticket's page", async () => {
  // Down to the review's evidence: the question card taken above.
  await capture("ticket", "#/ticket/FH-10", heading, {
    height: 2400,
    end: under((page) => page.getByText(EVIDENCE), 32),
  });
});
