import { describe, expect, it } from "vitest";

import { type Live, type NeedsReason, noReason, type Run } from "../api/runs";
import { overview } from "./overview";
import { needsReason } from "./runs";

// One session, three surfaces (FH-98): the card's live badge, the Runs tab
// and the Overview word the reason the server derived for the same runs,
// so they cannot disagree. Before FH-98 the card named the Explore
// subagent's prompt and the Runs tab the session's own decision.
const base = {
  agent: "claude",
  project: "alpha",
  children: [],
  cwd: "",
  branch: "main",
  worktree: "",
  source: "",
  ticket: "AL-1",
  ticketTitle: "One",
  linkedBy: "claim" as const,
  dirty: false,
  noHandoff: false,
  started: "2026-10-05T10:00:00Z",
  lastActivity: "2026-10-05T10:00:00Z",
  ended: "",
  endReason: "",
  tools: 0,
  edits: 0,
  files: [],
  plan: [],
  progress: { done: 0, total: 0 },
};
const decision = {
  id: "q-1",
  run: "claude:s",
  runShort: "claude:s",
  kind: "decision" as const,
  text: "Ship it?",
  asked: "2026-10-05T10:00:00Z",
};

// fixture is a session with its Explore subagent waiting on a Bash prompt,
// with or without an open decision of its own, as the server sends it.
function fixture(ownQuestion: boolean) {
  const reason: NeedsReason = ownQuestion
    ? { ...noReason, kind: "question", question: "decision" }
    : {
        ...noReason,
        kind: "permission",
        permission: "Bash",
        waitingOn: "claude:s/x",
        agentType: "Explore",
      };
  const session: Run = {
    ...base,
    id: "claude:s",
    short: "claude:s",
    kind: "session",
    parent: "",
    agentType: "",
    state: "needs-you",
    permission: "",
    reason,
    questions: ownQuestion ? [decision] : [],
  };
  const explore: Run = {
    ...base,
    id: "claude:s/x",
    short: "claude:s",
    kind: "subagent",
    parent: "claude:s",
    agentType: "Explore",
    state: "needs-you",
    permission: "Bash",
    reason: { ...noReason, kind: "permission", permission: "Bash" },
    questions: [],
  };
  const live: Live = {
    run: session.id,
    short: session.short,
    agent: "claude",
    state: "needs-you",
    done: 0,
    total: 0,
    step: "",
    reason,
    lastActivity: "",
  };
  return { session, explore, live };
}

describe("one reason on every surface", () => {
  it.each([
    ["its own decision first", true, "Needs a decision"],
    ["its subagent's prompt", false, "Explore needs permission for Bash"],
  ])("names %s: %s", (_, ownQuestion, want) => {
    const { session, explore, live } = fixture(ownQuestion);
    const card = needsReason(live.reason);
    const runsTab = needsReason(session.reason);
    const reasons = overview([], [session, explore]).decision.map(
      (item) => item.reason,
    );
    expect(card).toBe(want);
    expect(runsTab).toBe(want);
    expect(reasons).toContain(want);
  });
});
