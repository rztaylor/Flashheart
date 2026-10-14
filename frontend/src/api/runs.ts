// Agent runs (RUN-2, VIEW-3, VIEW-8): typed requests and response checks.
import {
  type AuthenticatedFetch,
  getJSON,
  isArrayOf,
  isBoolean,
  isNumber,
  isOptional,
  isRecord,
  isString,
  isStringArray,
} from "./client";

export type RunState = "needs-you" | "working" | "quiet" | "waiting" | "ended";

export const RUN_STATES: RunState[] = [
  "needs-you",
  "working",
  "quiet",
  "waiting",
  "ended",
];

export interface RunCounts {
  working: number;
  needsYou: number;
  waiting: number;
  quiet: number;
  ended: number;
  live: number;
  // needsYouUnticketed are the runs in Needs you with no ticket on the
  // board, which the Needs you filter cannot show (FH-44).
  needsYouUnticketed: number;
}

export const noRuns: RunCounts = {
  working: 0,
  needsYou: 0,
  waiting: 0,
  quiet: 0,
  ended: 0,
  live: 0,
  needsYouUnticketed: 0,
};

// NeedsReason is why a run in Needs you needs you, derived once on the
// server so the card, the Runs tab and the Overview word the same blocker
// (FH-98, D39). kind is "" when the run does not need you.
export interface NeedsReason {
  kind: "permission" | "question" | "";
  // permission is the tool awaiting permission ("?" when unknown).
  permission: string;
  // question is the open question's kind; answered means its answer waits
  // for delivery to the session.
  question: QuestionKind | "";
  answered: boolean;
  // waitingOn is the subagent whose prompt or question it is, with its
  // type ("" when unknown); both are empty when it is the run's own.
  waitingOn: string;
  agentType: string;
  // mcpOnly is set with answered when the run reports through the MCP
  // server alone (no hooks): the answer goes with its next tool call.
  mcpOnly?: boolean;
}

// noReason is the reason of a run that does not need you.
export const noReason: NeedsReason = {
  kind: "",
  permission: "",
  question: "",
  answered: false,
  waitingOn: "",
  agentType: "",
};

// Live is a card's badge: the linked run that most needs attention.
export interface Live {
  run: string;
  // short is the run id as people see it (agent-protocol §2).
  short: string;
  agent: string;
  state: RunState;
  reason: NeedsReason;
  lastActivity: string;
}

// TicketSessions summarises the sessions working for a ticket (RUN-9): the
// run that speaks for it (the live one that most needs attention, else the
// most recently active; "" with no run), its state ("ended" when none is
// live, "" with no run), the latest activity of any of them, its handoff
// flags, and the ticket's subagents by state (subagents never wait).
export interface TicketSessions {
  run: string;
  short: string;
  agent: string;
  state: RunState | "";
  lastActivity: string;
  // noLiveSession: the ticket is in progress and no live run works on it.
  noLiveSession: boolean;
  dirty: boolean;
  noHandoff: boolean;
  subagents: { done: number; running: number; needsYou: number };
}

export type QuestionKind = "question" | "decision" | "review" | "blocked";

// Question is an agent's ask_human question (RUN-8): open until its answer
// reaches the session with its next prompt.
export interface Question {
  id: string;
  run: string;
  // runShort is the asking run's short id (agent-protocol §2).
  runShort: string;
  ticket?: string;
  kind: QuestionKind;
  text: string;
  options?: string[];
  asked: string;
  answer?: string;
  answeredBy?: string;
  answeredAt?: string;
  delivered?: boolean;
  // answeredInSession: the user replied in the session's own chat before
  // answering on the board, so the question no longer waits (RUN-8).
  answeredInSession?: boolean;
  // sessionEnded: the asking session has ended; an answer waits for it to
  // resume, and the question does not need you (RUN-8, VIEW-2).
  sessionEnded?: boolean;
  // mcpOnly: the asking run reports through the MCP server alone (no
  // hooks), so the answer goes with its next Flashheart tool call (D34).
  mcpOnly?: boolean;
}

export interface TimelineEntry {
  time: string;
  kind: string;
  tool?: string;
  path?: string;
  failed?: boolean;
  // tools and failures count an activity record's tool uses (FH-55).
  tools?: number;
  failures?: number;
  detail?: string;
  ticket?: string;
  // branch marks a hookless run's restated start: its branch switch
  // (FH-71).
  branch?: string;
}

export interface Run {
  id: string;
  // short is the run id as people see it: the agent and 8 characters of
  // the session id (the last for a time-ordered Codex id), a subagent's
  // its session's (agent-protocol §2).
  short: string;
  agent: string;
  kind: "session" | "subagent";
  parent: string;
  children: string[];
  project: string;
  cwd: string;
  branch: string;
  worktree: string;
  source: string;
  agentType: string;
  state: RunState;
  // mcpOnly: a run the MCP server started that no hook reports
  // (agent-protocol §7.1); the server decides it, from its events.
  mcpOnly?: boolean;
  ticket: string;
  ticketTitle: string;
  linkedBy: "" | "claim" | "branch";
  dirty: boolean;
  noHandoff: boolean;
  permission: string;
  reason: NeedsReason;
  started: string;
  lastActivity: string;
  ended: string;
  endReason: string;
  tools: number;
  edits: number;
  files: string[];
  questions: Question[];
  timeline?: TimelineEntry[];
}

export interface RunsResponse {
  revision: number;
  runs: Run[];
  counts: RunCounts;
  hiddenEnded: number;
}

export const isRunState = (value: unknown): value is RunState =>
  RUN_STATES.includes(value as RunState);

export function isRunCounts(value: unknown): value is RunCounts {
  return (
    isRecord(value) &&
    isNumber(value.working) &&
    isNumber(value.needsYou) &&
    isNumber(value.waiting) &&
    isNumber(value.quiet) &&
    isNumber(value.ended) &&
    isNumber(value.live) &&
    isNumber(value.needsYouUnticketed)
  );
}

function isReason(value: unknown): value is NeedsReason {
  return (
    isRecord(value) &&
    (value.kind === "" ||
      value.kind === "permission" ||
      value.kind === "question") &&
    isString(value.permission) &&
    (value.question === "" || isQuestionKind(value.question)) &&
    isBoolean(value.answered) &&
    isString(value.waitingOn) &&
    isString(value.agentType) &&
    isBoolean(value.mcpOnly)
  );
}

export function isLive(value: unknown): value is Live {
  return (
    isRecord(value) &&
    isString(value.run) &&
    isString(value.short) &&
    isString(value.agent) &&
    isRunState(value.state) &&
    isReason(value.reason) &&
    isString(value.lastActivity)
  );
}

export function isTicketSessions(value: unknown): value is TicketSessions {
  return (
    isRecord(value) &&
    isString(value.run) &&
    isString(value.short) &&
    isString(value.agent) &&
    (value.state === "" || isRunState(value.state)) &&
    isString(value.lastActivity) &&
    isBoolean(value.noLiveSession) &&
    isBoolean(value.dirty) &&
    isBoolean(value.noHandoff) &&
    isRecord(value.subagents) &&
    isNumber(value.subagents.done) &&
    isNumber(value.subagents.running) &&
    isNumber(value.subagents.needsYou)
  );
}

export const QUESTION_KINDS: QuestionKind[] = [
  "question",
  "decision",
  "review",
  "blocked",
];
const isQuestionKind = (value: unknown): value is QuestionKind =>
  QUESTION_KINDS.includes(value as QuestionKind);

const optionalString = isOptional(isString);
const optionalNumber = isOptional(isNumber);
const optionalBoolean = isOptional(isBoolean);

export function isQuestion(value: unknown): value is Question {
  return (
    isRecord(value) &&
    isString(value.id) &&
    isString(value.run) &&
    isString(value.runShort) &&
    optionalString(value.ticket) &&
    isQuestionKind(value.kind) &&
    isString(value.text) &&
    isOptional(isStringArray)(value.options) &&
    isString(value.asked) &&
    optionalString(value.answer) &&
    optionalString(value.answeredBy) &&
    optionalString(value.answeredAt) &&
    optionalBoolean(value.delivered) &&
    optionalBoolean(value.answeredInSession) &&
    optionalBoolean(value.sessionEnded) &&
    optionalBoolean(value.mcpOnly)
  );
}

function isEntry(value: unknown): value is TimelineEntry {
  return (
    isRecord(value) &&
    isString(value.time) &&
    isString(value.kind) &&
    optionalString(value.tool) &&
    optionalString(value.path) &&
    optionalBoolean(value.failed) &&
    optionalNumber(value.tools) &&
    optionalNumber(value.failures) &&
    optionalString(value.detail) &&
    optionalString(value.ticket) &&
    optionalString(value.branch)
  );
}

export function isRun(value: unknown): value is Run {
  return (
    isRecord(value) &&
    isString(value.id) &&
    isString(value.short) &&
    isString(value.agent) &&
    (value.kind === "session" || value.kind === "subagent") &&
    isString(value.parent) &&
    isStringArray(value.children) &&
    isString(value.project) &&
    isString(value.cwd) &&
    isString(value.branch) &&
    isString(value.worktree) &&
    isString(value.source) &&
    isString(value.agentType) &&
    isRunState(value.state) &&
    isBoolean(value.mcpOnly) &&
    isString(value.ticket) &&
    isString(value.ticketTitle) &&
    (value.linkedBy === "" ||
      value.linkedBy === "claim" ||
      value.linkedBy === "branch") &&
    isBoolean(value.dirty) &&
    isBoolean(value.noHandoff) &&
    isString(value.permission) &&
    isReason(value.reason) &&
    isString(value.started) &&
    isString(value.lastActivity) &&
    isString(value.ended) &&
    isString(value.endReason) &&
    isNumber(value.tools) &&
    isNumber(value.edits) &&
    isStringArray(value.files) &&
    isArrayOf(isQuestion)(value.questions) &&
    isOptional(isArrayOf(isEntry))(value.timeline)
  );
}

export const isRuns = (value: unknown): value is RunsResponse =>
  isRecord(value) &&
  isNumber(value.revision) &&
  isArrayOf(isRun)(value.runs) &&
  isRunCounts(value.counts) &&
  isNumber(value.hiddenEnded);

// RunResponse is GET /api/run: one run with its timeline.
export interface RunResponse {
  revision: number;
  run: Run;
}

export const isRunResponse = (value: unknown): value is RunResponse =>
  isRecord(value) && isNumber(value.revision) && isRun(value.run);

// fetchRuns lists a project's runs, or every project's; Ended runs older
// than a day are left out unless endedAll is set (VIEW-3).
export function fetchRuns(
  fetcher: AuthenticatedFetch,
  project: string,
  endedAll: boolean,
  signal?: AbortSignal,
) {
  const query = new URLSearchParams();
  if (project) query.set("project", project);
  if (endedAll) query.set("ended", "all");
  const suffix = query.size > 0 ? `?${query}` : "";
  return getJSON(
    fetcher,
    `/api/runs${suffix}`,
    isRuns,
    "Runs response was invalid",
    signal,
  );
}

export async function fetchRun(
  fetcher: AuthenticatedFetch,
  id: string,
  signal?: AbortSignal,
): Promise<Run> {
  const response = await getJSON(
    fetcher,
    `/api/run?id=${encodeURIComponent(id)}`,
    isRunResponse,
    "Run response was invalid",
    signal,
  );
  return response.run;
}
