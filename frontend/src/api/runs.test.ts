import { describe, expect, it } from "vitest";

import { isCard } from "./board";
import { isPreferences } from "./preferences";
import { isLive, isRun, isRunCounts } from "./runs";

const counts = {
  working: 1,
  needsYou: 1,
  waiting: 0,
  quiet: 0,
  ended: 2,
  live: 2,
  needsYouUnticketed: 0,
};
const live = {
  run: "claude:s",
  short: "claude:s",
  agent: "claude",
  state: "needs-you",
  done: 1,
  total: 3,
  step: "Fix header",
  permission: "Bash",
  waitingOn: "",
  question: "",
  questionAnswered: false,
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
  ticket: "AL-3",
  ticketTitle: "Card panel",
  linkedBy: "branch",
  dirty: true,
  noHandoff: false,
  permission: "",
  started: "2026-10-06T07:00:00Z",
  lastActivity: "2026-10-06T07:05:00Z",
  ended: "",
  endReason: "",
  tools: 3,
  edits: 1,
  files: ["src/a.ts"],
  plan: [{ text: "Fix header", status: "in_progress" }],
  progress: { done: 0, total: 1, current: "Fix header" },
  questions: [
    {
      id: "q-1",
      run: "claude:s",
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
    expect(isLive({ ...live, question: "decision" })).toBe(true);
    expect(isLive({ ...live, question: "chat" })).toBe(false);
  });

  it("reject unknown states and missing fields", () => {
    expect(isRunCounts({ ...counts, live: "2" })).toBe(false);
    expect(isRunCounts({ ...counts, needsYouUnticketed: undefined })).toBe(
      false,
    );
    expect(isLive({ ...live, state: "busy" })).toBe(false);
    expect(isLive({ ...live, waitingOn: undefined })).toBe(false);
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
      workstream: "",
      tags: [],
      modified: "",
      criteria: { done: 0, total: 0 },
      excerpt: "",
      handoffNext: "",
      blocked: false,
      blockedBy: [],
      needsRepair: [],
      warnings: [],
      needsYou: true,
      agentWorking: false,
      openQuestions: 0,
    };
    expect(isCard({ ...card, live })).toBe(true);
    expect(isCard({ ...card, live: { ...live, state: "busy" } })).toBe(false);
    expect(isCard({ ...card, needsYou: undefined })).toBe(false);
  });

  it("check saved preferences", () => {
    const preferences = {
      theme: "system",
      density: "normal",
      colourBy: "type",
      hiddenColumns: [],
      scopes: {},
    };
    expect(isPreferences(preferences)).toBe(true);
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
            view: "agents",
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
