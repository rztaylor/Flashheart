import { describe, expect, it } from "vitest";

import type { Run, TimelineEntry } from "../api/runs";
import {
  agentName,
  compactTimeline,
  counted,
  describeEntry,
  liveReason,
  needsReason,
  permissionReason,
  planStations,
  questionReason,
  sessionsOf,
  shortID,
} from "./runs";

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

describe("describeEntry", () => {
  const entry = (patch: Partial<TimelineEntry>): TimelineEntry => ({
    time: "2026-10-05T10:00:00Z",
    kind: "tool.used",
    ...patch,
  });
  it("names each kind of event in plain words", () => {
    expect(describeEntry(entry({ tool: "Edit", path: "src/app.ts" }))).toBe(
      "Edit src/app.ts",
    );
    expect(describeEntry(entry({ tool: "Read", failed: true }))).toBe(
      "Read failed",
    );
    expect(describeEntry(entry({ kind: "turn.start" }))).toBe("Prompt");
    expect(
      describeEntry(entry({ kind: "turn.start", detail: "background" })),
    ).toBe("Background task finished");
    expect(describeEntry(entry({ kind: "turn.end" }))).toBe("Turn finished");
    expect(
      describeEntry(entry({ kind: "permission.requested", tool: "Bash" })),
    ).toBe("Asked permission for Bash");
    expect(describeEntry(entry({ kind: "permission.requested" }))).toBe(
      "Asked for permission",
    );
    expect(
      describeEntry(entry({ kind: "permission.resolved", detail: "denied" })),
    ).toBe("Permission denied");
    expect(describeEntry(entry({ kind: "run.start", detail: "resume" }))).toBe(
      "Session resumed",
    );
    expect(
      describeEntry(entry({ kind: "run.end", detail: "prompt_input_exit" })),
    ).toBe("Ended (prompt input exit)");
    expect(describeEntry(entry({ kind: "compact", detail: "pre" }))).toBe(
      "Compacting context",
    );
    expect(
      describeEntry(entry({ kind: "plan.updated", detail: "Fix header" })),
    ).toBe("Plan: Fix header");
    expect(describeEntry(entry({ kind: "notification", detail: "idle" }))).toBe(
      "Idle at the prompt",
    );
    expect(describeEntry(entry({ kind: "checkpoint", ticket: "FH-1" }))).toBe(
      "Checkpoint on FH-1",
    );
    expect(
      describeEntry(
        entry({ kind: "question.asked", detail: "decision", ticket: "FH-1" }),
      ),
    ).toBe("Asked for a decision on FH-1");
    expect(describeEntry(entry({ kind: "question.answered" }))).toBe(
      "Answered on the board",
    );
    expect(describeEntry(entry({ kind: "question.delivered" }))).toBe(
      "Answer delivered",
    );
    expect(
      describeEntry(
        entry({ kind: "question.answered-in-session", ticket: "FH-1" }),
      ),
    ).toBe("Question on FH-1 answered in the session");
    expect(describeEntry(entry({ kind: "future.kind" }))).toBe("future.kind");
  });
});

describe("compactTimeline", () => {
  it("folds runs of the same non-edit tool and lists the newest first", () => {
    const at = (minute: number) => `2026-10-05T10:0${minute}:00Z`;
    const compact = compactTimeline([
      { time: at(0), kind: "turn.start" },
      { time: at(1), kind: "tool.used", tool: "Read" },
      { time: at(2), kind: "tool.used", tool: "Read" },
      { time: at(3), kind: "tool.used", tool: "Read" },
      { time: at(4), kind: "tool.used", tool: "Edit", path: "a.ts" },
      { time: at(5), kind: "tool.used", tool: "Edit", path: "b.ts" },
    ]);
    expect(
      compact.map((item) => [describeEntry(item.entry), item.count]),
    ).toEqual([
      ["Edit b.ts", 1],
      ["Edit a.ts", 1],
      ["Read", 3],
      ["Prompt", 1],
    ]);
    expect(compact[2]?.entry.time).toBe(at(3));
  });
});

describe("planStations", () => {
  it("draws the plan as stations with the current stop marked", () => {
    expect(
      planStations([
        { text: "a", status: "completed" },
        { text: "b", status: "in_progress" },
        { text: "c", status: "pending" },
      ]),
    ).toEqual(["served", "current", "ahead"]);
    // Nothing in progress: the first pending step is current.
    expect(
      planStations([
        { text: "a", status: "completed" },
        { text: "b", status: "pending" },
      ]),
    ).toEqual(["served", "current"]);
  });
  it("is empty for long plans, which show as numbers instead", () => {
    const long = Array.from({ length: 13 }, () => ({
      status: "pending" as const,
    }));
    expect(planStations(long)).toEqual([]);
  });
});

it("names agents", () => {
  expect(agentName("claude")).toBe("Claude");
  expect(agentName("codex")).toBe("Codex");
  expect(agentName("other")).toBe("other");
});

it("says what a run waiting on permission needs", () => {
  expect(permissionReason("Bash")).toBe("Permission for Bash");
  expect(permissionReason("?")).toBe("Waiting for permission");
  expect(permissionReason("")).toBe("Waiting for permission");
});

it("names the subagent a session is waiting on", () => {
  const session = run("claude:s", { state: "needs-you" });
  const explore = run("claude:s/x", {
    state: "needs-you",
    agentType: "Explore",
    permission: "Bash",
  });
  expect(needsReason(session, [explore])).toBe(
    "Explore needs permission for Bash",
  );
  expect(needsReason({ ...session, permission: "Edit" }, [explore])).toBe(
    "Permission for Edit",
  );
  expect(needsReason(session, [])).toBe("Waiting for permission");
  expect(
    needsReason(session, [{ ...explore, agentType: "", permission: "?" }]),
  ).toBe("Subagent needs permission");
});

it("gives a card's live run the same reason as its run on the Runs tab", () => {
  const live = {
    run: "claude:s",
    short: "claude:s",
    agent: "claude",
    state: "needs-you" as const,
    done: 0,
    total: 0,
    step: "",
    permission: "Bash",
    waitingOn: "Explore",
    question: "" as const,
    questionAnswered: false,
    lastActivity: "",
  };
  expect(liveReason(live)).toBe("Explore needs permission for Bash");
  expect(liveReason({ ...live, waitingOn: "" })).toBe("Permission for Bash");
  expect(
    liveReason({
      ...live,
      permission: "",
      waitingOn: "",
      question: "decision",
    }),
  ).toBe("Needs a decision");
  expect(
    liveReason({
      ...live,
      permission: "",
      waitingOn: "",
      question: "decision",
      questionAnswered: true,
    }),
  ).toBe("Answer waits for its next prompt");
});

it("says what a question asks for, and when its answer is on its way", () => {
  const asked = {
    id: "q-1",
    run: "claude:s",
    kind: "review" as const,
    text: "Is the layout right?",
    asked: "2026-10-05T10:00:00Z",
  };
  expect(questionReason(asked)).toBe("Asks for a review");
  expect(questionReason({ ...asked, kind: "blocked" })).toBe("Is blocked");
  expect(questionReason({ ...asked, kind: "question" })).toBe("Has a question");
  expect(
    questionReason({
      ...asked,
      answer: "Yes",
      answeredAt: "2026-10-05T10:01:00Z",
    }),
  ).toBe("Answer waits for its next prompt");
  const session = run("claude:s", { state: "needs-you", questions: [asked] });
  expect(needsReason(session, [])).toBe("Asks for a review");
  // A permission prompt is named before a question.
  expect(needsReason({ ...session, permission: "Bash" }, [])).toBe(
    "Permission for Bash",
  );
  const helper = run("claude:s/x", {
    state: "needs-you",
    agentType: "Plan",
    questions: [{ ...asked, kind: "decision" }],
  });
  expect(needsReason(run("claude:s", { state: "needs-you" }), [helper])).toBe(
    "Plan needs a decision",
  );
});

it("shortens run ids and counts things in words", () => {
  expect(shortID("claude:3f2a9c1e-7b44-4d0e")).toBe("3f2a9c1e");
  expect(shortID("claude:3f2a9c1e-7b44/ae8ecef8da6a4664f")).toBe("ae8ecef8");
  expect(counted(1, "tool")).toBe("1 tool");
  expect(counted(3, "edit")).toBe("3 edits");
});

it("treats a subagent whose session is missing as a session", () => {
  const runs = [run("claude:a"), run("claude:a/x"), run("claude:gone/y")];
  expect(sessionsOf(runs).map((item) => item.id)).toEqual([
    "claude:a",
    "claude:gone/y",
  ]);
});

it("plans with every step completed have no current stop", () => {
  expect(
    planStations([
      { text: "a", status: "completed" },
      { text: "b", status: "completed" },
    ]),
  ).toEqual(["served", "served"]);
});
