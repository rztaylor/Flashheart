import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import type { Question, Run, TicketSessions } from "../api/runs";
import {
  overview,
  riskReasons,
  SECTIONS,
  sectionOpen,
  sessionWords,
  subagentWords,
} from "./overview";

function card(patch: Partial<Card>): Card {
  return {
    project: "alpha",
    id: "AL-99",
    slug: "x",
    column: "backlog",
    title: "X",
    type: "feature",
    priority: "medium",
    workstream: "",
    tags: [],
    created: "2026-10-01",
    updated: "",
    modified: "2026-10-05T09:00:00Z",
    branch: "",
    dependsOn: [],
    criteria: { done: 0, total: 0 },
    excerpt: "",
    handoffNext: "",
    attachments: 0,
    hasReview: false,
    blocked: false,
    blockedBy: [],
    needsRepair: [],
    warnings: [],
    needsYou: false,
    agentWorking: false,
    openQuestions: 0,
    ...patch,
  };
}

function sessions(patch: Partial<TicketSessions>): TicketSessions {
  return {
    run: "claude:s1",
    short: "claude:s1",
    agent: "claude",
    state: "working",
    lastActivity: "2026-10-05T11:50:00Z",
    noLiveSession: false,
    dirty: false,
    noHandoff: false,
    subagents: { done: 0, running: 0, needsYou: 0 },
    ...patch,
  };
}

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
    branch: "main",
    worktree: "",
    source: "",
    agentType: "",
    state: "working",
    ticket: "",
    ticketTitle: "",
    linkedBy: "",
    dirty: false,
    noHandoff: false,
    permission: "",
    started: "2026-10-05T10:00:00Z",
    lastActivity: "2026-10-05T10:00:00Z",
    ended: "",
    endReason: "",
    tools: 0,
    edits: 0,
    files: [],
    plan: [],
    progress: { done: 0, total: 0 },
    questions: [],
    ...patch,
  };
}

function question(id: string, patch: Partial<Question> = {}): Question {
  return {
    id,
    run: "claude:q",
    kind: "decision",
    text: "Which way?",
    asked: "2026-10-05T11:00:00Z",
    ...patch,
  };
}

describe("SECTIONS", () => {
  it("are in the approved order with a calm empty sentence each", () => {
    expect(SECTIONS.map((section) => section.id)).toEqual([
      "decision",
      "review",
      "risk",
      "progress",
      "unticketed",
      "upNext",
    ]);
    expect(SECTIONS.map((section) => section.title)).toEqual([
      "Needs your decision",
      "Ready for your review",
      "At risk",
      "In progress",
      "Work with no ticket",
      "Up next",
    ]);
    for (const section of SECTIONS) {
      expect(section.empty).toMatch(/^[A-Z][^.]*\.$/);
    }
  });

  it("opens decision and review always, at risk and in progress when not empty, and keeps the tiles closed", () => {
    expect(sectionOpen("decision", 0)).toBe(true);
    expect(sectionOpen("review", 3)).toBe(true);
    expect(sectionOpen("risk", 0)).toBe(false);
    expect(sectionOpen("risk", 1)).toBe(true);
    expect(sectionOpen("progress", 2)).toBe(true);
    expect(sectionOpen("unticketed", 2)).toBe(false);
    expect(sectionOpen("upNext", 5)).toBe(false);
  });
});

describe("subagentWords", () => {
  it("says the counts in words, leaving out zero parts", () => {
    expect(subagentWords({ done: 3, running: 2, needsYou: 1 })).toBe(
      "subagents 3 done · 2 running · 1 needs you",
    );
    expect(subagentWords({ done: 0, running: 1, needsYou: 0 })).toBe(
      "subagents 1 running",
    );
    expect(subagentWords({ done: 0, running: 0, needsYou: 2 })).toBe(
      "subagents 2 need you",
    );
    expect(subagentWords({ done: 0, running: 0, needsYou: 0 })).toBe("");
  });
});

describe("sessionWords", () => {
  it("says the session's state and its subagents", () => {
    expect(
      sessionWords(
        sessions({ subagents: { done: 3, running: 2, needsYou: 0 } }),
      ),
    ).toBe("Working · subagents 3 done · 2 running");
    expect(sessionWords(sessions({ state: "waiting" }))).toBe(
      "Waiting for a prompt",
    );
    expect(sessionWords(sessions({ state: "needs-you" }))).toBe("Needs you");
  });
});

describe("riskReasons", () => {
  it("names each reason in words", () => {
    expect(
      riskReasons(
        card({
          column: "in-progress",
          sessions: sessions({ state: "ended", noLiveSession: true }),
        }),
        false,
      ),
    ).toEqual(["No live session"]);
    expect(
      riskReasons(
        card({
          column: "in-progress",
          sessions: sessions({ run: "", state: "", noLiveSession: true }),
        }),
        false,
      ),
    ).toEqual(["No session has worked on it"]);
    expect(
      riskReasons(
        card({ column: "in-progress", sessions: sessions({ state: "quiet" }) }),
        false,
      ),
    ).toEqual(["Session gone quiet"]);
    expect(
      riskReasons(
        card({
          column: "review",
          sessions: sessions({ state: "ended", noHandoff: true }),
        }),
        false,
      ),
    ).toEqual(["Ended without a handoff"]);
    expect(
      riskReasons(
        card({
          column: "up-next",
          blocked: true,
          blockedBy: [
            {
              kind: "ticket",
              text: "Depends on AL-16, which is in progress",
              ticket: { project: "alpha", id: "AL-16" },
              missing: false,
            },
          ],
        }),
        true,
      ),
    ).toEqual(["Top of Up next", "blocked by AL-16"]);
    expect(
      riskReasons(
        card({ column: "in-progress", sessions: sessions({}) }),
        false,
      ),
    ).toEqual([]);
    // A finished ticket is never at risk.
    expect(
      riskReasons(
        card({ column: "done", sessions: sessions({ noHandoff: true }) }),
        false,
      ),
    ).toEqual([]);
  });

  it("combines reasons and names a workstream blocker", () => {
    expect(
      riskReasons(
        card({
          column: "in-progress",
          sessions: sessions({
            state: "ended",
            noLiveSession: true,
            noHandoff: true,
          }),
        }),
        false,
      ),
    ).toEqual(["No live session", "ended without a handoff"]);
    expect(
      riskReasons(
        card({
          column: "up-next",
          blocked: true,
          blockedBy: [
            {
              kind: "workstream",
              text: "Depends on workstream storage",
              workstream: "storage",
              missing: false,
            },
            {
              kind: "ticket",
              text: "Depends on AL-2",
              ticket: { project: "alpha", id: "AL-2" },
              missing: false,
            },
          ],
        }),
        true,
      ),
    ).toEqual(["Top of Up next", "blocked by workstream storage and 1 more"]);
  });
});

describe("overview", () => {
  const working = card({
    id: "AL-1",
    column: "in-progress",
    sessions: sessions({
      lastActivity: "2026-10-05T11:40:00Z",
      subagents: { done: 1, running: 1, needsYou: 0 },
    }),
  });
  const recent = card({
    id: "AL-2",
    column: "in-progress",
    sessions: sessions({ lastActivity: "2026-10-05T11:55:00Z" }),
  });
  const stale = card({
    id: "AL-3",
    column: "in-progress",
    modified: "2026-10-04T08:00:00Z",
    sessions: sessions({
      run: "",
      state: "",
      lastActivity: "",
      noLiveSession: true,
    }),
  });
  const reviewOld = card({
    id: "AL-4",
    column: "review",
    updated: "2026-10-03T09:00:00Z",
    criteria: { done: 3, total: 3 },
  });
  const reviewNew = card({
    id: "AL-5",
    column: "review",
    updated: "2026-10-05T09:00:00Z",
  });
  const topBlocked = card({
    id: "AL-6",
    column: "up-next",
    blocked: true,
    blockedBy: [
      {
        kind: "ticket",
        text: "Depends on AL-1",
        ticket: { project: "alpha", id: "AL-1" },
        missing: false,
      },
    ],
  });
  const second = card({ id: "AL-7", column: "up-next" });
  const betaTop = card({ project: "beta", id: "BE-1", column: "up-next" });
  const done = card({
    id: "AL-8",
    column: "done",
    sessions: sessions({ state: "ended", noHandoff: true }),
  });
  const cards = [
    working,
    recent,
    stale,
    reviewOld,
    reviewNew,
    topBlocked,
    second,
    betaTop,
    done,
  ];

  it("groups tickets by what they need from you", () => {
    const view = overview(cards, []);
    expect(view.review.map((item) => item.card.id)).toEqual(["AL-4", "AL-5"]);
    expect(view.review[0]?.since).toBe("2026-10-03T09:00:00Z");
    // At risk: the in-progress ticket no session works on (its time is the
    // ticket's own change, not session activity), and the blocked top of
    // Up next.
    expect(view.risk.map((item) => item.card.id)).toEqual(["AL-3", "AL-6"]);
    expect(view.risk[0]).toMatchObject({
      reasons: ["No session has worked on it"],
      since: "2026-10-04T08:00:00Z",
      sinceSession: false,
    });
    // In progress leaves out what is at risk; most recent activity first.
    expect(view.progress.map((item) => item.card.id)).toEqual(["AL-2", "AL-1"]);
    expect(view.progress[1]?.words).toBe(
      "Working · subagents 1 done · 1 running",
    );
    // Up next: the top of each project's column, in its order.
    expect(view.upNext.map((item) => item.id)).toEqual([
      "AL-6",
      "AL-7",
      "BE-1",
    ]);
    expect(view.upNextTotal).toBe(3);
  });

  it("times a risk by session activity when a session has any", () => {
    const quiet = card({
      id: "AL-9",
      column: "in-progress",
      modified: "2026-10-05T11:59:00Z",
      sessions: sessions({
        state: "quiet",
        lastActivity: "2026-10-05T11:20:00Z",
      }),
    });
    expect(overview([quiet], []).risk[0]).toMatchObject({
      reasons: ["Session gone quiet"],
      since: "2026-10-05T11:20:00Z",
      sinceSession: true,
    });
  });

  it("is empty with no tickets and no runs", () => {
    const view = overview([], []);
    for (const section of SECTIONS) {
      expect(view[section.id]).toEqual([]);
    }
  });

  it("lists open questions and permission prompts, oldest first, with or without a ticket", () => {
    const asking = run("claude:asking", {
      state: "needs-you",
      ticket: "AL-1",
      ticketTitle: "Run on AL-1",
      branch: "feature/one",
      questions: [
        question("q-1", { ticket: "AL-2", asked: "2026-10-05T11:30:00Z" }),
        // Answered on the board, it stays until its session's next prompt
        // takes the answer, as the run still needs you until then (RUN-3).
        question("q-2", {
          asked: "2026-10-05T11:20:00Z",
          answeredAt: "2026-10-05T11:31:00Z",
          answer: "x",
        }),
        // Delivered, answered in the session or from an ended session: none
        // needs a decision.
        question("q-3", { delivered: true }),
        question("q-4", { answeredInSession: true }),
      ],
    });
    const permission = run("claude:perm", {
      state: "needs-you",
      permission: "Bash",
      project: "beta",
      branch: "main",
      lastActivity: "2026-10-05T11:10:00Z",
    });
    const parent = run("claude:parent", {
      state: "needs-you",
      ticket: "AL-1",
      ticketTitle: "Run on AL-1",
      children: ["claude:parent/sub"],
      lastActivity: "2026-10-05T11:45:00Z",
    });
    const sub = run("claude:parent/sub", {
      state: "needs-you",
      agentType: "Explore",
      ticket: "AL-1",
      ticketTitle: "Run on AL-1",
      permission: "WebFetch",
      lastActivity: "2026-10-05T11:44:00Z",
    });
    const ended = run("claude:ended", {
      state: "ended",
      questions: [question("q-5", { sessionEnded: true })],
    });
    const view = overview(cards, [asking, permission, parent, sub, ended]);
    expect(view.decision.map((item) => item.key)).toEqual([
      "claude:perm",
      "q-2",
      "q-1",
      "claude:parent/sub",
    ]);
    const [prompt, answered, asked, child] = view.decision;
    expect(answered?.reason).toBe("Answer waits for its next prompt");
    expect(prompt).toMatchObject({
      kind: "permission",
      reason: "Permission for Bash",
      project: "beta",
      branch: "main",
      ticket: undefined,
    });
    // A question names the ticket it is about, titled from the board.
    expect(asked).toMatchObject({
      kind: "question",
      ticket: { id: "AL-2", title: "X" },
    });
    // A session waiting on its subagent is answered through the subagent.
    expect(child).toMatchObject({
      kind: "permission",
      reason: "Explore needs permission for WebFetch",
      ticket: { id: "AL-1", title: "X" },
    });
  });

  it("lists live sessions with no ticket, most recent first", () => {
    const view = overview(cards, [
      run("claude:old", { lastActivity: "2026-10-05T09:00:00Z" }),
      run("claude:new", { lastActivity: "2026-10-05T11:00:00Z" }),
      run("claude:new/sub", { lastActivity: "2026-10-05T11:30:00Z" }),
      run("claude:gone", { state: "ended" }),
      run("claude:linked", { ticket: "AL-1" }),
    ]);
    expect(view.unticketed.map((item) => item.id)).toEqual([
      "claude:new",
      "claude:old",
    ]);
  });
});
