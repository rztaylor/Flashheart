import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import type { Question, Run, TimelineEntry } from "../api/runs";
import { type ActivityInput, activityOf } from "./activity";

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
});
