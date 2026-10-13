import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import type { Comment, History, HistoryChange } from "../api/board";
import type { Question, Run, TimelineEntry } from "../api/runs";
import {
  type ActivityFilter,
  type ActivityInput,
  activityOf,
  filterActivity,
  historyWords,
} from "./activity";

// Entries are dated where the browser is; these tests read UTC.
beforeAll(() => {
  vi.stubEnv("TZ", "UTC");
});
afterAll(() => {
  vi.unstubAllEnvs();
});

const notes = (...bullets: string[]) =>
  [
    "# Ticket",
    "",
    "## Description",
    "",
    "Text.",
    "",
    "## Notes",
    "",
    ...bullets,
    "",
    "## Handoff",
    "",
    "_Updated 2026-10-12 12:30 UTC by claude:aaaa1111 (run) on feature/x._",
    "",
    "**Next**",
    "- Verify",
    "",
  ].join("\n");

function question(overrides: Partial<Question> = {}): Question {
  return {
    id: "q1",
    run: "claude:aaaa1111-2222",
    runShort: "claude:aaaa1111",
    ticket: "FH-65",
    kind: "decision",
    text: "Ship it?",
    asked: "2026-10-12T12:00:00Z",
    ...overrides,
  };
}

function run(overrides: Partial<Run> = {}): Run {
  return {
    id: "claude:aaaa1111-2222",
    short: "claude:aaaa1111",
    questions: [],
    timeline: [],
    ...overrides,
  } as Run;
}

const checkpoint = (time: string, ticket = "FH-65"): TimelineEntry => ({
  time,
  kind: "checkpoint",
  ticket,
});

function input(overrides: Partial<ActivityInput> = {}): ActivityInput {
  return {
    id: "FH-65",
    body: notes(),
    handoff: null,
    runs: [],
    questions: [],
    comments: [],
    history: [],
    ...overrides,
  };
}

describe("activityOf", () => {
  it("reads the notes' dated bullets with their author, continuation lines included", () => {
    const entries = activityOf(
      input({
        body: notes(
          "None.",
          "",
          "- 2026-10-11 · claude:2ff6eb1c: First line",
          "  carried on.",
          "",
          "- 2026-10-12 · Started while blocked: FH-1 is not done",
        ),
      }),
    );
    expect(entries).toEqual([
      {
        kind: "note",
        key: "note:0",
        day: "2026-10-11",
        by: "claude:2ff6eb1c",
        markdown: "First line\n  carried on.",
      },
      {
        kind: "note",
        key: "note:1",
        day: "2026-10-12",
        by: "",
        markdown: "Started while blocked: FH-1 is not done",
      },
    ]);
  });

  it("reads a question answered on the board from its note", () => {
    const [entry] = activityOf(
      input({
        body: notes(
          '- 2026-10-11 · Question from claude:2ff6eb1c — "Keep \\"Done\\"?" Answer (Robert): "Yes".',
        ),
      }),
    );
    expect(entry).toMatchObject({
      kind: "question",
      day: "2026-10-11",
      by: "claude:2ff6eb1c",
      question: 'Keep "Done"?',
      answer: "Yes",
      answeredBy: "Robert",
      state: "answered",
    });
  });

  it("takes the ticket's questions from its runs, with their answers, once each", () => {
    const answered = question({
      answer: "Yes",
      answeredBy: "Robert",
      answeredAt: "2026-10-12T12:05:00Z",
    });
    const entries = activityOf(
      input({
        // The board wrote the answer into the notes too.
        body: notes(
          '- 2026-10-12 · Question from claude:aaaa1111 — "Ship it?" Answer (Robert): "Yes".',
        ),
        runs: [
          run({
            questions: [
              answered,
              question({ id: "q2", ticket: "FH-1", text: "Elsewhere" }),
            ],
          }),
          run({ id: "claude:aaaa1111-2222/sub", questions: [answered] }),
        ],
        questions: [
          question({
            id: "q3",
            text: "Which colour?",
            asked: "2026-10-12T13:00:00Z",
          }),
        ],
      }),
    );
    expect(entries.map((entry) => entry.kind)).toEqual([
      "question",
      "question",
    ]);
    expect(entries[0]).toMatchObject({
      question: "Ship it?",
      time: "2026-10-12T12:00:00Z",
      answer: "Yes",
      answeredAt: "2026-10-12T12:05:00Z",
      state: "answered",
    });
    expect(entries[1]).toMatchObject({
      question: "Which colour?",
      state: "open",
    });
  });

  it("says when a question was answered in the session's own chat", () => {
    const [entry] = activityOf(
      input({
        runs: [run({ questions: [question({ answeredInSession: true })] })],
      }),
    );
    expect(entry).toMatchObject({ state: "in-session" });
  });

  it("lists handoff updates from checkpoints, the current one with its next steps", () => {
    const entries = activityOf(
      input({
        handoff: { markdown: notes(), next: ["Verify"] },
        runs: [
          run({
            timeline: [
              checkpoint("2026-10-12T11:00:00Z"),
              checkpoint("2026-10-12T11:30:00Z", "FH-1"),
              checkpoint("2026-10-12T12:30:20Z"),
            ],
          }),
        ],
      }),
    );
    expect(entries).toMatchObject([
      { kind: "handoff", time: "2026-10-12T11:00:00Z", next: [] },
      {
        kind: "handoff",
        time: "2026-10-12T12:30:20Z",
        by: "claude:aaaa1111",
        next: ["Verify"],
      },
    ]);
  });

  it("keeps the current handoff when its checkpoint is no longer in the runs", () => {
    const entries = activityOf(
      input({ handoff: { markdown: notes(), next: ["Verify"] } }),
    );
    expect(entries).toEqual([
      {
        kind: "handoff",
        key: "handoff:current",
        day: expect.any(String),
        time: "2026-10-12T12:30:00Z",
        by: "claude:aaaa1111",
        next: ["Verify"],
      },
    ]);
  });

  it("orders oldest first: by day, a day's notes before its timed entries", () => {
    const entries = activityOf(
      input({
        body: notes(
          "- 2026-10-13 · claude:aaaa1111: later note",
          "- 2026-10-11 · claude:aaaa1111: early note",
        ),
        handoff: { markdown: notes(), next: [] },
        runs: [
          run({
            questions: [question({ asked: "2026-10-12T13:00:00Z" })],
            timeline: [checkpoint("2026-10-12T12:30:00Z")],
          }),
        ],
      }),
    );
    expect(
      entries.map((entry) =>
        entry.kind === "note" ? entry.markdown : entry.kind,
      ),
    ).toEqual(["early note", "handoff", "question", "later note"]);
  });

  it("is empty for a ticket with no notes, questions or handoff", () => {
    expect(activityOf(input({ body: "# T\n\n## Notes\n\n" }))).toEqual([]);
  });

  it("reads comments and history from the activity file, in time order with the rest", () => {
    const entries = activityOf(
      input({
        body: notes("- 2026-10-12 · claude:aaaa1111: a note"),
        handoff: { markdown: notes(), next: ["Verify"] },
        comments: [
          comment({ id: "c2", time: "2026-10-13T09:00:00Z", body: "Later" }),
          comment({
            id: "c1",
            time: "2026-10-12T12:10:00Z",
            edited: "2026-10-12T12:20:00Z",
            quoteFrom: "Acceptance criteria 2",
            quote: "No tabs",
          }),
        ],
        history: [
          history({
            id: "h1",
            time: "2026-10-11T08:00:00Z",
            changes: [change({ what: "created", to: "backlog" })],
          }),
          history({
            id: "h2",
            time: "2026-10-12T12:40:00Z",
            by: "claude:aaaa1111-2222-3333",
            byPerson: false,
            changes: [change({ what: "moved", from: "up-next", to: "review" })],
          }),
        ],
      }),
    );
    expect(entries.map((entry) => entry.key)).toEqual([
      "history:h1",
      "note:0",
      "comment:c1",
      "handoff:current",
      "history:h2",
      "comment:c2",
    ]);
    expect(entries[2]).toMatchObject({
      kind: "comment",
      id: "c1",
      by: "Robert",
      edited: "2026-10-12T12:20:00Z",
      quoteFrom: "Acceptance criteria 2",
      quote: "No tabs",
      markdown: "Not in Done.",
    });
    // A run is named by its short id, as elsewhere on the board.
    expect(entries[4]).toMatchObject({
      kind: "history",
      by: "claude:aaaa1111",
      byPerson: false,
      words: ["moved it from Up next to Ready to review"],
    });
  });

  it("takes agents' moves from the runs only before the activity file's history begins", () => {
    const moved = (time: string, to: string, ticket = "FH-65") => ({
      time,
      kind: "ticket.moved",
      detail: to,
      ticket,
    });
    const runs = [
      run({
        timeline: [
          moved("2026-10-11T09:00:00Z", "in-progress"),
          moved("2026-10-11T09:30:00Z", "review", "FH-1"),
          // The same move the history records: shown once, from history.
          moved("2026-10-12T10:00:00.400Z", "review"),
        ],
      }),
    ];
    const withHistory = activityOf(
      input({
        runs,
        history: [
          history({
            id: "h1",
            time: "2026-10-12T10:00:00Z",
            by: "claude:aaaa1111-2222",
            byPerson: false,
            changes: [
              change({ what: "moved", from: "in-progress", to: "review" }),
            ],
          }),
        ],
      }),
    );
    expect(
      withHistory.map((entry) =>
        entry.kind === "history" ? entry.words.join("; ") : entry.kind,
      ),
    ).toEqual([
      "moved it to In progress",
      "moved it from In progress to Ready to review",
    ]);
    // Without an activity file the run window is all there is.
    expect(activityOf(input({ runs }))).toHaveLength(2);
  });
});

describe("historyWords", () => {
  it("words each change of a write in plain terms, never the ticket's text", () => {
    expect(
      historyWords([
        change({ what: "created", to: "up-next" }),
        change({ what: "moved", from: "review", to: "rejected" }),
        change({
          what: "edited",
          fields: ["title", "acceptance-criteria", "pr", "depends-on", "body"],
        }),
        change({ what: "ticked", criteria: [2], steps: [1, 3] }),
        change({ what: "unticked", criteria: [1, 4] }),
        change({ what: "archived" }),
        change({ what: "restored" }),
      ]),
    ).toEqual([
      "created it in Up next",
      "moved it from Ready to review to Rejected",
      "edited the title, the acceptance criteria, the pull request, its dependencies and the ticket's text",
      "ticked criterion 2 and review steps 1 and 3",
      "unticked criteria 1 and 4",
      "archived it",
      "restored it",
    ]);
  });

  it("words a change request's states by how many comments they cover (FH-126)", () => {
    expect(
      historyWords([
        change({ what: "change-requested", comments: ["a1"] }),
        change({ what: "change-requested", comments: ["a1", "a2"] }),
        change({ what: "addressed", comments: ["a1"] }),
        change({ what: "addressed", comments: ["a1", "a2", "a3"] }),
        change({ what: "resolved", comments: ["a1"] }),
        change({ what: "reopened", comments: ["a1", "a2"] }),
      ]),
    ).toEqual([
      "asked for a change",
      "asked for 2 changes",
      "addressed a change request",
      "addressed 3 change requests",
      "resolved a change request",
      "reopened 2 change requests",
    ]);
  });

  it("words a deleted comment without its text (FH-140)", () => {
    expect(
      historyWords([
        change({ what: "deleted", comments: ["a1"] }),
        change({ what: "deleted", comments: ["a1", "a2"] }),
      ]),
    ).toEqual(["deleted a comment", "deleted 2 comments"]);
  });
});

describe("filterActivity", () => {
  const entries = activityOf(
    input({
      body: notes("- 2026-10-12 · claude:aaaa1111: a note"),
      comments: [comment({ id: "c1", time: "2026-10-12T12:10:00Z" })],
      history: [
        history({
          id: "h1",
          time: "2026-10-12T08:00:00Z",
          changes: [change({ what: "created", to: "backlog" })],
        }),
      ],
    }),
  );

  it("keeps everything for All, people's comments for Comments and the changes for History", () => {
    const kinds = (filter: ActivityFilter) =>
      filterActivity(entries, filter).map((entry) => entry.kind);
    expect(kinds("all")).toEqual(["note", "history", "comment"]);
    expect(kinds("comments")).toEqual(["comment"]);
    expect(kinds("history")).toEqual(["history"]);
  });
});

function comment(overrides: Partial<Comment> = {}): Comment {
  return {
    id: "c1",
    time: "2026-10-12T12:00:00Z",
    by: "Robert",
    byPerson: true,
    state: "open",
    edited: "",
    quoteFrom: "",
    quote: "",
    body: "Not in Done.",
    ...overrides,
  };
}

function history(overrides: Partial<History> = {}): History {
  return {
    id: "h1",
    time: "2026-10-12T12:00:00Z",
    by: "Robert",
    byPerson: true,
    changes: [],
    ...overrides,
  };
}

function change(overrides: Partial<HistoryChange> = {}): HistoryChange {
  return {
    what: "edited",
    from: "",
    to: "",
    fields: [],
    criteria: [],
    steps: [],
    ...overrides,
  };
}
