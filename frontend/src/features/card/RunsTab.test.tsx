import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { noReason, type Run } from "../../api/runs";
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
    reason: noReason,
    started: "2026-10-04T13:00:00Z",
    lastActivity: "2026-10-04T13:29:00Z",
    ended: "",
    endReason: "",
    tools: 1,
    edits: 0,
    files: [],
    questions: [],
    timeline: [],
    ...patch,
  };
}

// The Runs tab of an orchestrated run (agent-protocol §10): the session's
// three subagents hang beneath it, each with its own state, ticket and
// tool and edit counts.
describe("RunsTab with subagents", () => {
  const runs = [
    run(session, {
      state: "needs-you",
      children: [`${session}/b1`, `${session}/b2`, `${session}/b3`],
      reason: {
        ...noReason,
        kind: "permission",
        permission: "Bash",
        waitingOn: `${session}/b3`,
        agentType: "Explore",
      },
    }),
    run(`${session}/b1`, { state: "ended", ended: "2026-10-04T13:28:00Z" }),
    run(`${session}/b2`, {
      agentType: "general-purpose",
      ticket: "AL-2",
      ticketTitle: "Board columns",
      tools: 4,
      edits: 2,
    }),
    run(`${session}/b3`, {
      state: "needs-you",
      permission: "Bash",
      reason: { ...noReason, kind: "permission", permission: "Bash" },
    }),
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

  it("lists the subagents in the order they started, by time not text", () => {
    // The server trims trailing zeros, so .3 (300 ms) sorts after .315 as
    // text though it started first.
    const started = [
      run(session, { children: [`${session}/b1`, `${session}/b2`] }),
      run(`${session}/b2`, {
        agentType: "general-purpose",
        started: "2026-10-12T13:35:37.315Z",
      }),
      run(`${session}/b1`, {
        state: "ended",
        started: "2026-10-12T13:35:37.3Z",
        ended: "2026-10-12T13:35:37.31Z",
      }),
    ];
    const tree = renderToStaticMarkup(
      <RunsTab detail={{ id: "AL-3", branch: "", runs: started }} />,
    ).split('aria-label="Subagents of')[1];
    expect(tree?.indexOf("Ended")).toBeLessThan(
      tree?.indexOf("general-purpose") ?? -1,
    );
  });

  it("names the ticket a subagent claimed and its tool and edit counts", () => {
    expect(markup).toContain("AL-2");
    expect(markup).toContain("4 tools, 2 edits");
  });

  it("says which subagent holds the session", () => {
    expect(markup).toContain("Explore needs permission for Bash");
    // The subagent's own row says what it needs.
    const tree = markup.slice(markup.indexOf('aria-label="Subagents of'));
    expect(tree).toContain("Permission for Bash");
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

// A ticket's plan is its criteria and handoff (D51): no run shows a plan,
// even one whose log recorded one before.
describe("RunsTab plan", () => {
  it("shows no plan section, even for a run from an older log", () => {
    const old = {
      plan: [{ text: "Read the board", status: "completed" }],
      progress: { done: 1, total: 1, current: "Read the board" },
    } as Partial<Run>;
    const markup = renderToStaticMarkup(
      <RunsTab
        detail={{ id: "AL-3", branch: "", runs: [run(session, old)] }}
      />,
    );
    expect(markup).not.toMatch(/>Plan<\/h4>/);
    expect(markup).not.toContain("Read the board");
    expect(markup).toContain("Edited files");
  });
});
