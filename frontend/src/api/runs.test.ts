import { describe, expect, it } from "vitest";

import { isCard } from "./board";
import { isPreferences } from "./preferences";
import { isLive, isRun, isRunCounts, noReason } from "./runs";

const counts = {
  working: 1,
  needsYou: 1,
  waiting: 0,
  quiet: 0,
  ended: 2,
  live: 2,
  needsYouUnticketed: 0,
  needsYouShown: 1,
};
const live = {
  run: "claude:s",
  short: "claude:s",
  agent: "claude",
  state: "needs-you",
  reason: {
    kind: "permission",
    permission: "Bash",
    question: "",
    answered: false,
    waitingOn: "",
    agentType: "",
    mcpOnly: false,
  },
  lastActivity: "2026-10-06T07:00:00Z",
};
const run = {
  id: "claude:s",
  short: "claude:s",
  agent: "claude",
  kind: "session",
  parent: "",
  children: [],
  project: "alpha",
  cwd: "/src/alpha",
  branch: "main",
  worktree: "/src/alpha",
  source: "startup",
  agentType: "",
  state: "working",
  mcpOnly: false,
  ticket: "AL-3",
  ticketTitle: "Card panel",
  linkedBy: "branch",
  dirty: true,
  noHandoff: false,
  permission: "",
  reason: {
    kind: "",
    permission: "",
    question: "",
    answered: false,
    waitingOn: "",
    agentType: "",
    mcpOnly: false,
  },
  started: "2026-10-06T07:00:00Z",
  lastActivity: "2026-10-06T07:05:00Z",
  ended: "",
  endReason: "",
  tools: 3,
  edits: 1,
  files: ["src/a.ts"],
  questions: [
    {
      id: "q-1",
      run: "claude:s",
      runShort: "claude:s",
      ticket: "AL-3",
      kind: "decision",
      text: "Which schema?",
      options: ["v1", "v2"],
      asked: "2026-10-06T07:04:00Z",
    },
  ],
};

describe("run validators", () => {
  it("accept what the API sends", () => {
    expect(isRunCounts(counts)).toBe(true);
    expect(isLive(live)).toBe(true);
    expect(isRun(run)).toBe(true);
    expect(
      isRun({
        ...run,
        timeline: [{ time: "2026-10-06T07:00:00Z", kind: "turn.start" }],
      }),
    ).toBe(true);
  });

  it("check questions", () => {
    expect(isRun({ ...run, questions: undefined })).toBe(false);
    expect(
      isRun({ ...run, questions: [{ ...run.questions[0], kind: "chat" }] }),
    ).toBe(false);
    expect(
      isRun({
        ...run,
        questions: [{ ...run.questions[0], runShort: undefined }],
      }),
    ).toBe(false);
    const asked = { ...live.reason, kind: "question", question: "decision" };
    expect(isLive({ ...live, reason: asked })).toBe(true);
    expect(isLive({ ...live, reason: { ...asked, question: "chat" } })).toBe(
      false,
    );
  });

  it("reject unknown states and missing fields", () => {
    expect(isRunCounts({ ...counts, live: "2" })).toBe(false);
    expect(isRunCounts({ ...counts, needsYouUnticketed: undefined })).toBe(
      false,
    );
    expect(isLive({ ...live, state: "busy" })).toBe(false);
    expect(isLive({ ...live, reason: undefined })).toBe(false);
    expect(isLive({ ...live, reason: { ...live.reason, kind: "busy" } })).toBe(
      false,
    );
    expect(isRun({ ...run, reason: { ...run.reason, waitingOn: 1 } })).toBe(
      false,
    );
    expect(isRun({ ...run, kind: "thread" })).toBe(false);
    expect(isRun({ ...run, timeline: [{ kind: "turn.start" }] })).toBe(false);
  });

  it("check a card's live badge and virtual-column flags", () => {
    const card = {
      project: "alpha",
      id: "AL-3",
      slug: "card-panel",
      column: "in-progress",
      title: "Card panel",
      type: "feature",
      priority: "high",
      rejected: false,
      workstream: "",
      tags: [],
      created: "2026-10-06",
      updated: "",
      modified: "",
      branch: "",
      pr: "",
      dependsOn: [],
      criteria: { done: 0, total: 0 },
      excerpt: "",
      handoffNext: "",
      attachments: 0,
      hasReview: false,
      changeRequests: { open: 0, addressed: 0 },
      blocked: false,
      blockedBy: [],
      needsRepair: [],
      warnings: [],
      needsYou: true,
      agentWorking: false,
      openQuestions: 0,
      reason: noReason,
    };
    expect(isCard({ ...card, live })).toBe(true);
    expect(isCard({ ...card, live: { ...live, state: "busy" } })).toBe(false);
    expect(isCard({ ...card, needsYou: undefined })).toBe(false);
  });

  it("check a card's session summary", () => {
    const card = {
      project: "alpha",
      id: "AL-3",
      slug: "card-panel",
      column: "in-progress",
      title: "Card panel",
      type: "feature",
      priority: "high",
      rejected: false,
      workstream: "",
      tags: [],
      created: "2026-10-06",
      updated: "",
      modified: "",
      branch: "",
      pr: "",
      dependsOn: [],
      criteria: { done: 0, total: 0 },
      excerpt: "",
      handoffNext: "",
      attachments: 0,
      hasReview: false,
      changeRequests: { open: 0, addressed: 0 },
      blocked: false,
      blockedBy: [],
      needsRepair: [],
      warnings: [],
      needsYou: false,
      agentWorking: true,
      openQuestions: 0,
      reason: noReason,
    };
    const sessions = {
      run: "claude:s",
      short: "claude:s",
      agent: "claude",
      state: "working",
      lastActivity: "2026-10-06T07:05:00Z",
      noLiveSession: false,
      dirty: true,
      noHandoff: false,
      subagents: { done: 3, running: 2, needsYou: 0 },
    };
    expect(isCard({ ...card, sessions })).toBe(true);
    // An in-progress ticket with no run at all has no state.
    const nobody = {
      ...sessions,
      run: "",
      short: "",
      agent: "",
      state: "",
      lastActivity: "",
      noLiveSession: true,
      subagents: { done: 0, running: 0, needsYou: 0 },
    };
    expect(isCard({ ...card, sessions: nobody })).toBe(true);
    expect(isCard({ ...card, sessions: { ...sessions, state: "busy" } })).toBe(
      false,
    );
    expect(
      isCard({ ...card, sessions: { ...sessions, subagents: { done: 1 } } }),
    ).toBe(false);
  });

  it("check saved preferences", () => {
    const preferences = {
      theme: "system",
      density: "normal",
      colourBy: "type",
      hiddenColumns: [],
      editor: "formatted",
      scopes: {},
      expandedWorkstreams: {},
    };
    expect(isPreferences(preferences)).toBe(true);
    // The section editor opens formatted or as markdown (FH-16).
    expect(isPreferences({ ...preferences, editor: "markdown" })).toBe(true);
    expect(isPreferences({ ...preferences, editor: "rich" })).toBe(false);
    expect(isPreferences({ ...preferences, editor: undefined })).toBe(false);
    // Expanded finished workstreams, per project (FH-68).
    expect(
      isPreferences({
        ...preferences,
        expandedWorkstreams: { flashheart: ["foundation"] },
      }),
    ).toBe(true);
    expect(
      isPreferences({
        ...preferences,
        expandedWorkstreams: { flashheart: [1] },
      }),
    ).toBe(false);
    expect(
      isPreferences({ ...preferences, expandedWorkstreams: undefined }),
    ).toBe(false);
    // Only the Backlog can be hidden (FH-41).
    expect(isPreferences({ ...preferences, hiddenColumns: ["backlog"] })).toBe(
      true,
    );
    expect(isPreferences({ ...preferences, hiddenColumns: ["done"] })).toBe(
      false,
    );
    expect(isPreferences({ ...preferences, hiddenColumns: undefined })).toBe(
      false,
    );
    expect(
      isPreferences({
        ...preferences,
        scopes: {
          alpha: {
            view: "overview",
            type: { include: [], exclude: [] },
            priority: { include: ["high"], exclude: [] },
            workstream: { include: [], exclude: ["board-ui"] },
            age: { include: [], exclude: [] },
            state: "working",
          },
        },
      }),
    ).toBe(true);
    // Filters are lists since FH-39; the single-value form is read only by
    // the server.
    expect(
      isPreferences({
        ...preferences,
        scopes: { alpha: { view: "board", type: "bug", state: "" } },
      }),
    ).toBe(false);
  });
});
