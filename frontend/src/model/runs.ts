// Pure view models for agent runs: why a run needs you, timeline wording
// and plan stations, for cards, the Runs tab and the Overview.
import type {
  Live,
  PlanItem,
  Question,
  QuestionKind,
  Run,
  RunState,
  TimelineEntry,
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

// reasonFor says why a run needs you: its own permission prompt, or that
// of the subagent (by type) it is waiting on.
export function reasonFor(permission: string, subagent: string): string {
  if (!subagent) return permissionReason(permission);
  return permission && permission !== "?"
    ? `${subagent} needs permission for ${permission}`
    : `${subagent} needs permission`;
}

const QUESTION_REASON: Record<QuestionKind, string> = {
  question: "Has a question",
  decision: "Needs a decision",
  review: "Asks for a review",
  blocked: "Is blocked",
};

// questionReason says what an agent's question asks for, or that its
// answer is waiting for the session's next prompt (HOOK-5).
const ANSWER_WAITING = "Answer waits for its next prompt";

export function questionReason(question: Question): string {
  return question.answeredAt ? ANSWER_WAITING : QUESTION_REASON[question.kind];
}

// openQuestion is a run's first question still waiting on the human or on
// delivery.
export function openQuestion(run: Run): Question | undefined {
  return run.questions.find((question) => !question.delivered);
}

// needsReason says why a session needs you, from it and its subagents: a
// permission prompt first, then a question.
export function needsReason(run: Run, children: Run[]): string {
  if (run.permission) return permissionReason(run.permission);
  const own = openQuestion(run);
  if (own) return questionReason(own);
  const child = children.find((item) => item.state === "needs-you");
  if (!child) return permissionReason("");
  const asked = child.permission ? undefined : openQuestion(child);
  if (asked) {
    const reason = questionReason(asked);
    return `${child.agentType || "Subagent"} ${reason.charAt(0).toLowerCase()}${reason.slice(1)}`;
  }
  return reasonFor(child.permission, child.agentType || "Subagent");
}

// liveReason is needsReason for a card's live run.
export function liveReason(live: Live): string {
  if (!live.permission && live.question)
    return live.questionAnswered
      ? ANSWER_WAITING
      : QUESTION_REASON[live.question];
  return reasonFor(live.permission, live.waitingOn);
}

// shortID is a run's short id: the first 8 characters of its session id, or
// of its own id for a subagent (agent-protocol §2).
export function shortID(id: string): string {
  const [, rest = id] = id.split(":");
  const parts = rest.split("/");
  return (parts[parts.length - 1] ?? "").slice(0, 8);
}

// shortRun is a run id as people see it: the agent and its short id.
export function shortRun(id: string): string {
  return `${id.split(":")[0] ?? ""}:${shortID(id)}`;
}

// counted puts a count before a noun, plural unless it is one.
export function counted(count: number, noun: string): string {
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

// sessionsOf lists the runs shown as sessions: sessions, and subagents whose
// session is not in the list.
export function sessionsOf(runs: Run[]): Run[] {
  const ids = new Set(runs.map((run) => run.id));
  return runs.filter((run) => !run.parent || !ids.has(run.parent));
}

export function agentName(agent: string): string {
  if (agent === "claude") return "Claude";
  if (agent === "codex") return "Codex";
  return agent;
}

const source: Record<string, string> = {
  startup: "Session started",
  resume: "Session resumed",
  clear: "Session cleared",
  compact: "Resumed after compaction",
};

const words = (value: string) => value.replaceAll("_", " ");

const asked: Record<string, string> = {
  question: "Asked a question",
  decision: "Asked for a decision",
  review: "Asked for a review",
  blocked: "Said it is blocked",
};

const on = (ticket?: string) => (ticket ? ` on ${ticket}` : "");

// describeEntry says what one timeline entry was, in plain words.
export function describeEntry(entry: TimelineEntry): string {
  switch (entry.kind) {
    case "tool.used": {
      const tool = entry.tool || "Tool";
      if (entry.failed) return `${tool} failed`;
      return entry.path ? `${tool} ${entry.path}` : tool;
    }
    case "activity":
      return describeActivity(entry);
    case "turn.start":
      return entry.detail === "background"
        ? "Background task finished"
        : "Prompt";
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
    case "ticket.moved":
      return `Moved ${entry.ticket ?? "a ticket"}`;
    case "ticket.created":
      return `Created ${entry.ticket ?? "a ticket"}`;
    case "review.written":
      return `Wrote the review of ${entry.ticket ?? "a ticket"}`;
    case "question.asked":
      return `${asked[entry.detail ?? ""] ?? "Asked a question"}${on(entry.ticket)}`;
    case "question.answered":
      return "Answered on the board";
    case "question.delivered":
      return "Answer delivered";
    case "question.answered-in-session":
      return `Question${on(entry.ticket)} answered in the session`;
    default:
      return entry.ticket ? `${entry.kind} ${entry.ticket}` : entry.kind;
  }
}

// describeActivity summarises an activity record: the path it edited, its
// tool uses and how many failed (FH-55).
function describeActivity(entry: TimelineEntry): string {
  const tools = entry.tools ?? 0;
  const parts = entry.path
    ? [
        `Edited ${entry.path}`,
        ...(tools > 1 ? [counted(tools, "tool use")] : []),
      ]
    : [counted(tools, "tool use")];
  if (entry.failures) parts.push(`${entry.failures} failed`);
  return parts.join(", ");
}

export interface CompactEntry {
  entry: TimelineEntry;
  count: number;
}

// compactTimeline folds consecutive uses of the same tool that edited
// nothing into one line with a count, adds up consecutive activity records
// that edited nothing, and lists the newest first.
export function compactTimeline(entries: TimelineEntry[]): CompactEntry[] {
  const folded: CompactEntry[] = [];
  for (const entry of entries) {
    const last = folded[folded.length - 1];
    if (
      last &&
      entry.kind === "activity" &&
      last.entry.kind === "activity" &&
      !entry.path &&
      !last.entry.path
    ) {
      last.entry = {
        ...entry,
        tools: (last.entry.tools ?? 0) + (entry.tools ?? 0),
        failures: (last.entry.failures ?? 0) + (entry.failures ?? 0),
      };
      continue;
    }
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
