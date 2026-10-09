import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Run } from "../../api/runs";
import { RunsTab } from "./RunsTab";

const session = "claude:3f2a9c1e-0000-0000-0000-000000000000";

function run(id: string, patch: Partial<Run> = {}): Run {
  return {
    id,
    short: id.slice(0, 15),
    agent: "claude",
    kind: id.includes("/") ? "subagent" : "session",
    parent: id.includes("/") ? (id.split("/")[0] ?? "") : "",
    children: [],
    project: "alpha",
    cwd: "",
    branch: "feature/card-panel",
    worktree: "",
    source: "",
    agentType: id.includes("/") ? "Explore" : "",
    state: "working",
    ticket: "AL-3",
    ticketTitle: "Card panel",
    linkedBy: "claim",
    dirty: false,
    noHandoff: false,
    permission: "",
    started: "2026-10-04T13:00:00Z",
    lastActivity: "2026-10-04T13:29:00Z",
    ended: "",
    endReason: "",
    tools: 1,
    edits: 0,
    files: [],
    plan: [],
    progress: { done: 0, total: 0 },
    questions: [],
    timeline: [],
    ...patch,
  };
}

// The Runs tab of an orchestrated run (agent-protocol §10): the session's
// three subagents hang beneath it, each with its own state, ticket and step.
describe("RunsTab with subagents", () => {
  const runs = [
    run(session, {
      state: "needs-you",
      children: [`${session}/b1`, `${session}/b2`, `${session}/b3`],
    }),
    run(`${session}/b1`, { state: "ended", ended: "2026-10-04T13:28:00Z" }),
    run(`${session}/b2`, {
      agentType: "general-purpose",
      ticket: "AL-2",
      ticketTitle: "Board columns",
      plan: [
        { text: "Read the board", status: "completed" },
        { text: "Write the tests", status: "in_progress" },
      ],
      progress: { done: 1, total: 2, current: "Write the tests" },
    }),
    run(`${session}/b3`, { state: "needs-you", permission: "Bash" }),
  ];
  const markup = renderToStaticMarkup(
    <RunsTab detail={{ id: "AL-3", branch: "", runs }} />,
  );

  it("lists the subagents as a tree under their session", () => {
    const tree = markup.slice(markup.indexOf('aria-label="Subagents of'));
    expect(tree.match(/aria-expanded="false"/g)).toHaveLength(3);
    expect(tree.indexOf("Ended")).toBeLessThan(tree.indexOf("general-purpose"));
    expect(tree).toContain("Working");
    expect(tree).toContain("Needs you");
  });

  it("names the ticket a subagent claimed and its current step", () => {
    expect(markup).toContain("AL-2");
    expect(markup).toContain("Write the tests");
    expect(markup).toContain("1/2");
  });

  it("says which subagent holds the session", () => {
    expect(markup).toContain("Explore needs permission for Bash");
  });
});

describe("RunsTab for a subagent on its own ticket", () => {
  it("names the session it belongs to", () => {
    const markup = renderToStaticMarkup(
      <RunsTab
        detail={{
          id: "AL-2",
          branch: "",
          runs: [run(`${session}/b2`, { ticket: "AL-2" })],
        }}
      />,
    );
    expect(markup).toContain("Explore");
    expect(markup).toContain("subagent of");
    expect(markup).toContain("claude:3f2a9c1e");
  });
});

// Most sessions never record a plan (FH-54): the Runs tab shows plan UI
// only for a run that has one.
describe("RunsTab plan", () => {
  const render = (patch: Partial<Run>) =>
    renderToStaticMarkup(
      <RunsTab
        detail={{ id: "AL-3", branch: "", runs: [run(session, patch)] }}
      />,
    );

  it("shows no plan section for a run with no plan", () => {
    const markup = render({});
    expect(markup).not.toMatch(/>Plan<\/h4>/);
    expect(markup).not.toContain("No plan");
    expect(markup).toContain("Edited files");
  });

  it("shows the route and current step of a run with a plan", () => {
    const markup = render({
      plan: [
        { text: "Read the board", status: "completed" },
        { text: "Write the tests", status: "in_progress" },
        { text: "Open the PR", status: "pending" },
      ],
      progress: { done: 1, total: 3, current: "Write the tests" },
    });
    expect(markup).toMatch(/>Plan<\/h4>/);
    expect(markup).toContain("Done: </span>Read the board");
    expect(markup).toContain("In progress: </span>Write the tests");
    expect(markup).toContain("To do: </span>Open the PR");
  });
});
