// Pure view models for agent runs: lanes for the Agents view (VIEW-3),
// timeline wording and plan stations.
import {
  type PlanItem,
  RUN_STATES,
  type Run,
  type RunState,
  type TimelineEntry,
} from "../api/runs";

export const STATE_LABEL: Record<RunState, string> = {
  "needs-you": "Needs you",
  working: "Working",
  quiet: "Quiet",
  waiting: "Waiting",
  ended: "Ended",
};

// permissionReason says what a run waiting on permission needs, in words.
export function permissionReason(tool: string): string {
  return tool && tool !== "?"
    ? `Permission for ${tool}`
    : "Waiting for permission";
}

// needsReason says why a session needs you: its own permission prompt, or
// the subagent that is waiting on one.
export function needsReason(run: Run, children: Run[]): string {
  if (run.permission) return permissionReason(run.permission);
  const child = children.find((item) => item.state === "needs-you");
  if (!child) return permissionReason("");
  const name = child.agentType || "Subagent";
  return child.permission && child.permission !== "?"
    ? `${name} needs permission for ${child.permission}`
    : `${name} needs permission`;
}

export function agentName(agent: string): string {
  if (agent === "claude") return "Claude";
  if (agent === "codex") return "Codex";
  return agent;
}

export interface LaneRun {
  run: Run;
  children: Run[];
}

export interface Lane {
  state: RunState;
  runs: LaneRun[];
}

const time = (iso: string) => Date.parse(iso) || 0;

// laneRuns groups sessions into lanes by state, Needs you first, with each
// session's subagents nested under it whatever their own state. Needs you
// lists the longest waiting first; the other lanes the most recent first.
export function laneRuns(runs: Run[]): Lane[] {
  const byID = new Map(runs.map((run) => [run.id, run]));
  const children = new Map<string, Run[]>();
  const tops: Run[] = [];
  for (const run of runs) {
    if (run.parent && byID.has(run.parent)) {
      children.set(run.parent, [...(children.get(run.parent) ?? []), run]);
    } else {
      tops.push(run);
    }
  }
  return RUN_STATES.map((state) => {
    const members = tops
      .filter((run) => run.state === state)
      .sort((a, b) =>
        state === "needs-you"
          ? time(a.lastActivity) - time(b.lastActivity)
          : time(b.lastActivity) - time(a.lastActivity),
      );
    return {
      state,
      runs: members.map((run) => ({
        run,
        children: (children.get(run.id) ?? []).sort(
          (a, b) => time(a.started) - time(b.started),
        ),
      })),
    };
  });
}

const source: Record<string, string> = {
  startup: "Session started",
  resume: "Session resumed",
  clear: "Session cleared",
  compact: "Resumed after compaction",
};

const words = (value: string) => value.replaceAll("_", " ");

// describeEntry says what one timeline entry was, in plain words.
export function describeEntry(entry: TimelineEntry): string {
  switch (entry.kind) {
    case "tool.used": {
      const tool = entry.tool || "Tool";
      if (entry.failed) return `${tool} failed`;
      return entry.path ? `${tool} ${entry.path}` : tool;
    }
    case "turn.start":
      return "Prompt";
    case "turn.end":
      return "Turn finished";
    case "permission.requested":
      return entry.tool
        ? `Asked permission for ${entry.tool}`
        : "Asked for permission";
    case "permission.resolved":
      return `Permission ${entry.detail || "resolved"}`;
    case "run.start":
      return (
        source[entry.detail ?? ""] ??
        (entry.detail ? `Subagent started (${entry.detail})` : "Started")
      );
    case "run.end":
      return entry.detail ? `Ended (${words(entry.detail)})` : "Ended";
    case "compact":
      return entry.detail === "pre"
        ? "Compacting context"
        : "Context compacted";
    case "plan.updated":
      return entry.detail ? `Plan: ${entry.detail}` : "Plan updated";
    case "notification":
      return entry.detail === "idle"
        ? "Idle at the prompt"
        : `Notification: ${words(entry.detail ?? "")}`;
    case "checkpoint":
      return `Checkpoint on ${entry.ticket ?? "a ticket"}`;
    case "claim":
      return `Claimed ${entry.ticket ?? "a ticket"}`;
    case "release":
      return `Released ${entry.ticket ?? "a ticket"}`;
    default:
      return entry.ticket ? `${entry.kind} ${entry.ticket}` : entry.kind;
  }
}

export interface CompactEntry {
  entry: TimelineEntry;
  count: number;
}

// compactTimeline folds consecutive uses of the same tool that edited
// nothing into one line with a count, and lists the newest first.
export function compactTimeline(entries: TimelineEntry[]): CompactEntry[] {
  const folded: CompactEntry[] = [];
  for (const entry of entries) {
    const last = folded[folded.length - 1];
    const repeat =
      last &&
      entry.kind === "tool.used" &&
      last.entry.kind === "tool.used" &&
      !entry.path &&
      !last.entry.path &&
      !entry.failed &&
      !last.entry.failed &&
      entry.tool === last.entry.tool;
    if (repeat) {
      last.entry = entry;
      last.count++;
    } else {
      folded.push({ entry, count: 1 });
    }
  }
  return folded.reverse();
}

export type Station = "served" | "current" | "ahead";

// MAX_STATIONS is the longest plan drawn as a route; longer plans show as
// numbers.
export const MAX_STATIONS = 12;

// planStations draws a plan as a route: completed steps served, the step in
// progress (or else the first pending one) current, the rest ahead.
export function planStations(plan: PlanItem[]): Station[] {
  if (plan.length > MAX_STATIONS) return [];
  let current = plan.findIndex((item) => item.status === "in_progress");
  if (current < 0)
    current = plan.findIndex((item) => item.status !== "completed");
  return plan.map((item, index) =>
    item.status === "completed" && index !== current
      ? "served"
      : index === current
        ? "current"
        : "ahead",
  );
}
