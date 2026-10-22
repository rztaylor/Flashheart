import { appendFile, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import { callTool, IDLE_SESSION, promptHook, seedRuns } from "./agent-runs.mjs";
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

// A ticket's history keeps the questions agents asked about it and what
// became of them (FH-190): asked through the real MCP server, answered on
// the board or in the session's own chat, then still shown once the run has
// left the run window, read from the ticket's activity file alone.
test.describe.configure({ mode: "serial" });

let sandbox;
let server;
let context;
let page;
let gamma;

async function serve(browser) {
  server = launch(sandbox, ["serve", "--foreground"]);
  const url = await waitForManualURL(server.child, server.output);
  context = await browser.newContext();
  page = await context.newPage();
  await page.goto(url);
  await page.waitForURL((current) => current.pathname === "/");
}

test.beforeAll(async ({ browser }) => {
  sandbox = await makeSandbox();
  const demo = join(sandbox.home, "demo");
  await writeDemoBoard(demo);
  sandbox.root = demo;
  // Answers are recorded under the configured name (FH-8).
  await appendFile(
    join(sandbox.root, ".flashheart", "config.yaml"),
    "user_name: Robin\n",
  );
  ({ gamma } = await seedRuns(sandbox.home, sandbox.root));
  await serve(browser);
});

test.afterAll(async () => {
  await context?.close();
  if (server) await stopIfRunning(server.child);
  await sandbox?.cleanup();
});

const readActivity = () =>
  readFile(
    join(
      sandbox.root,
      "alpha",
      "tickets",
      "AL-1-project-skeleton",
      "activity.md",
    ),
    "utf8",
  ).catch(() => "");

const main = () => page.getByRole("main", { name: "Ticket AL-1" });
const activity = () => main().getByRole("region", { name: "Activity" });
const entries = () => activity().getByRole("listitem");
const show = async (name) => {
  await activity()
    .getByRole("group", { name: "Show in Activity" })
    .getByText(name, { exact: true })
    .click();
};

// age moves every recorded event three days back, into the file of its
// new day, so no run is in the run window any more; a line that is not an
// event stays where it was. Serve must not be running.
async function age() {
  const day = 24 * 60 * 60 * 1000;
  for (const project of await readdir(sandbox.root)) {
    const dir = join(sandbox.root, project, ".flashheart", "events");
    const files = await readdir(dir).catch(() => []);
    for (const file of files.filter((name) => name.endsWith(".jsonl"))) {
      const lines = (await readFile(join(dir, file), "utf8"))
        .split("\n")
        .filter(Boolean);
      await rm(join(dir, file));
      const moved = new Map();
      for (const line of lines) {
        let name = file;
        let text = line;
        try {
          const event = JSON.parse(line);
          event.ts = new Date(Date.parse(event.ts) - 3 * day).toISOString();
          name = `${event.ts.slice(0, 10)}.jsonl`;
          text = JSON.stringify(event);
        } catch {}
        moved.set(name, [...(moved.get(name) ?? []), text]);
      }
      for (const [name, list] of moved)
        await writeFile(join(dir, name), `${list.join("\n")}\n`, { flag: "a" });
    }
  }
}

test("a question and its answer stay in the ticket's history after the run leaves the window", async ({
  browser,
}) => {
  // The idle session asks about AL-1 through the real MCP server.
  await callTool(sandbox.root, gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "decision",
    text: "Keep the old project skeleton, or start from the new template?",
    options: ["Keep it", "New template"],
  });
  expect(await readActivity()).toContain("- asked: question q-");

  // It is answered on the board.
  await open(page, "#/p/alpha/board?t=AL-1");
  const panel = page.getByRole("complementary", { name: "Ticket AL-1" });
  const question = panel.getByRole("region", { name: "Needs a decision" });
  await question.getByRole("button", { name: "Keep it" }).click();
  await question.getByRole("button", { name: "Send answer" }).click();
  await expect(panel.getByText(/Answer \(Robin\): "Keep it"/)).toBeVisible();
  expect(await readActivity()).toContain("  answer: Keep it");

  // A second question is answered in the session's own chat; serve keeps
  // that in the history too.
  await callTool(sandbox.root, gamma, "ask_human", {
    run: `claude:${IDLE_SESSION}`,
    ticket: "AL-1",
    kind: "question",
    text: "Rename the skeleton package?",
  });
  promptHook(sandbox.root, IDLE_SESSION, gamma);
  await expect.poll(readActivity).toContain("- answered-in-session: question");

  // The runs leave the run window: serve starts again over an older log.
  await context.close();
  await stopIfRunning(server.child);
  await age();
  await serve(browser);

  for (const theme of ["light", "dark"]) {
    await open(page, "#/ticket/AL-1", { theme });
    await show("History");
    const asked = entries().filter({
      hasText: "Keep the old project skeleton, or start from the new template?",
    });
    await expect(asked).toHaveCount(1);
    await expect(asked).toContainText("asked you");
    await expect(asked).toContainText("Robin answered: Keep it");
    const chat = entries().filter({ hasText: "Rename the skeleton package?" });
    await expect(chat).toHaveCount(1);
    await expect(chat).toContainText("Answered in the session's own chat.");
    // Under All the question shows once, though its note says it too.
    await show("All");
    await expect(
      entries().filter({
        hasText:
          "Keep the old project skeleton, or start from the new template?",
      }),
    ).toHaveCount(1);
    await show("History");
    await expectNoAxeViolations(page, `question history ${theme}`);
    // Both questions in view, the first near the top.
    await asked.evaluate((item) => item.scrollIntoView({ block: "center" }));
    await shot(page, `question-history-1440-${theme}`);
  }
});
