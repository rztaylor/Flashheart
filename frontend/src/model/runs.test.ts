import { describe, expect, it } from "vitest";

import type { Run, TimelineEntry } from "../api/runs";
import {
  agentName,
  compactTimeline,
  describeEntry,
  laneRuns,
  planStations,
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
    ...patch,
  };
}

describe("laneRuns", () => {
  it("puts Needs you first, nests subagents and orders by urgency", () => {
    const lanes = laneRuns([
      run("claude:a", {
        state: "working",
        lastActivity: "2026-10-05T10:05:00Z",
        children: ["claude:a/x"],
      }),
      run("claude:a/x", { state: "needs-you" }),
      run("claude:b", {
        state: "needs-you",
        lastActivity: "2026-10-05T10:09:00Z",
      }),
      run("claude:c", {
        state: "needs-you",
        lastActivity: "2026-10-05T10:01:00Z",
      }),
      run("claude:d", {
        state: "working",
        lastActivity: "2026-10-05T10:08:00Z",
      }),
      run("claude:e", { state: "ended" }),
      run("claude:gone/y", { state: "working" }),
    ]);
    expect(lanes.map((lane) => lane.state)).toEqual([
      "needs-you",
      "working",
      "quiet",
      "waiting",
      "ended",
    ]);
    // Longest-waiting first in Needs you; most recent first elsewhere.
    expect(lanes[0]?.runs.map((entry) => entry.run.id)).toEqual([
      "claude:c",
      "claude:b",
    ]);
    expect(lanes[1]?.runs.map((entry) => entry.run.id)).toEqual([
      "claude:d",
      "claude:a",
      "claude:gone/y",
    ]);
    // A subagent rides with its session, whatever its own state.
    expect(lanes[1]?.runs[1]?.children.map((child) => child.id)).toEqual([
      "claude:a/x",
    ]);
    expect(lanes[4]?.runs).toHaveLength(1);
  });
});

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
