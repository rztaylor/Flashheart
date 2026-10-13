// Pure view models for agent runs: why a run needs you and timeline
// wording, for cards, the Runs tab and the Overview.
import { STATUSES } from "../api/board";
import type {
  NeedsReason,
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

// needsYouPhrase words how many runs need you, after their count: "needs
// you" or "need you", and with agents "agent needs you" or "agents need
// you" (the one attention plate's words, ui-layout.md §7).
export function needsYouPhrase(count: number, agents = false): string {
  if (agents) return count === 1 ? "agent needs you" : "agents need you";
  return count === 1 ? "needs you" : "need you";
}

// permissionReason says what a run waiting on permission needs, in words.
export function permissionReason(tool: string): string {
  return tool && tool !== "?"
    ? `Permission for ${tool}`
    : "Waiting for permission";
}

const QUESTION_REASON: Record<QuestionKind, string> = {
  question: "Has a question",
  decision: "Needs a decision",
  review: "Asks for a review",
  blocked: "Is blocked",
};

// startedByMCP reports a run id the MCP server minted for one connection,
// `<agent>:mcp-<random>` (agent-protocol §2): a run that ends with its
// connection and never resumes. Whether a run has hooks is the server's
// mcpOnly, not its id: a Codex session without hooks is named by its own
// session id (FH-74).
export function startedByMCP(runID: string): boolean {
  const [, session = ""] = runID.split(":");
  return session.startsWith("mcp-") && !session.includes("/");
}

// answerWaiting says that an answer waits for the asking session: for its
// next prompt (HOOK-5), or, for a run the server marks MCP-only (no
// hooks), its next Flashheart tool call (D34).
function answerWaiting(mcpOnly: boolean | undefined): string {
  return mcpOnly
    ? "Answer waits for its next tool call"
    : "Answer waits for its next prompt";
}

// questionReason says what an agent's question asks for, or that its
// answer is waiting for the session.
export function questionReason(question: Question): string {
  return question.answeredAt
    ? answerWaiting(question.mcpOnly)
    : QUESTION_REASON[question.kind];
}

// needsReason words why a run needs you, from the reason the server
// derived for it (FH-98): the server picks the blocker, this only words it.
// subagent names the run when it is a subagent shown on its own, as the
// Overview lists it; a subagent the run waits on is always named.
export function needsReason(reason: NeedsReason, subagent = ""): string {
  const who = reason.waitingOn ? reason.agentType || "Subagent" : subagent;
  if (reason.kind === "permission") {
    if (!who) return permissionReason(reason.permission);
    return reason.permission && reason.permission !== "?"
      ? `${who} needs permission for ${reason.permission}`
      : `${who} needs permission`;
  }
  if (reason.kind !== "question" || !reason.question) return "";
  const words = reason.answered
    ? answerWaiting(reason.mcpOnly)
    : QUESTION_REASON[reason.question];
  return who
    ? `${who} ${words.charAt(0).toLowerCase()}${words.slice(1)}`
    : words;
}

// shortID is the session part of a run's short id as the server sends it
// (agent-protocol §2): `claude:3f2a9c1e` is `3f2a9c1e`, for places that
// name the agent beside it.
export function shortID(short: string): string {
  return short.slice(short.indexOf(":") + 1);
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

// mcpOnly reports a run the MCP server started for an agent without
// Flashheart's hooks: it reports tool calls, claims and questions only,
// never a plan, edits or permission prompts (agent-protocol §7.1).
export function mcpOnly(run: Run): boolean {
  return Boolean(run.mcpOnly);
}

// agentNote qualifies a run's agent: "MCP only" for a run without hooks.
export function agentNote(run: Run): string | undefined {
  return mcpOnly(run) ? "MCP only" : undefined;
}

// runAgentName names a session's agent, with its note when it has one.
export function runAgentName(run: Run): string {
  const note = agentNote(run);
  return note ? `${agentName(run.agent)} (${note})` : agentName(run.agent);
}

const source: Record<string, string> = {
  startup: "Session started",
  resume: "Session resumed",
  clear: "Session cleared",
  compact: "Resumed after compaction",
  mcp: "Connected without hooks (MCP only)",
};

const words = (value: string) => value.replaceAll("_", " ");

const asked: Record<string, string> = {
  question: "Asked a question",
  decision: "Asked for a decision",
  review: "Asked for a review",
  blocked: "Said it is blocked",
};

const on = (ticket?: string) => (ticket ? ` on ${ticket}` : "");

// movedTo names the status a ticket moved to, by its title.
const movedTo = (status?: string) =>
  status
    ? ` to ${STATUSES.find((item) => item.id === status)?.title ?? status}`
    : "";

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
      if (entry.detail === "background") return "Background task finished";
      if (entry.detail === "agent") return "Message from an agent";
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
      if (entry.branch) return `Switched to branch ${entry.branch}`;
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
      return `Moved ${entry.ticket ?? "a ticket"}${movedTo(entry.detail)}`;
    case "ticket.updated":
      return `Updated ${entry.ticket ?? "a ticket"}`;
    case "ticket.created":
      return `Created ${entry.ticket ?? "a ticket"}`;
    case "review.written":
      return `Wrote the review of ${entry.ticket ?? "a ticket"}`;
    case "attachment.added":
      return `Attached a file to ${entry.ticket ?? "a ticket"}`;
    case "question.asked":
      return `${asked[entry.detail ?? ""] ?? "Asked a question"}${on(entry.ticket)}`;
    case "question.answered":
      return "Answered on the board";
    case "question.delivered":
      return "Answer delivered";
    case "comment.delivered":
      return `${entry.detail === "change-request" ? "Change request" : "Comment"}${on(entry.ticket)} delivered`;
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
