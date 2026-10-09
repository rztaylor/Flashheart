// Agent runs (RUN-2, VIEW-3, VIEW-8): typed requests and response checks.
import { type AuthenticatedFetch, getJSON, isRecord } from "./client";

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

// Live is a card's badge: the linked run that most needs attention.
export interface Live {
  run: string;
  short: string;
  agent: string;
  state: RunState;
  done: number;
  total: number;
  step: string;
  permission: string;
  // waitingOn names the subagent (by type) whose permission prompt the run
  // is waiting on; empty when it is the session's own.
  waitingOn: string;
  // question is the kind of open question the run waits on, when that and
  // not a permission prompt is why it needs you.
  question: QuestionKind | "";
  // questionAnswered: that question's answer waits for the next prompt.
  questionAnswered: boolean;
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
}

export interface PlanItem {
  id?: string;
  text?: string;
  status: "pending" | "in_progress" | "completed";
}

export interface TimelineEntry {
  time: string;
  kind: string;
  tool?: string;
  path?: string;
  failed?: boolean;
  detail?: string;
  ticket?: string;
}

export interface Run {
  id: string;
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
  ticket: string;
  ticketTitle: string;
  linkedBy: "" | "claim" | "branch";
  dirty: boolean;
  noHandoff: boolean;
  permission: string;
  started: string;
  lastActivity: string;
  ended: string;
  endReason: string;
  tools: number;
  edits: number;
  files: string[];
  plan: PlanItem[];
  progress: { done: number; total: number; current?: string };
  questions: Question[];
  timeline?: TimelineEntry[];
}

export interface RunsResponse {
  revision: number;
  runs: Run[];
  counts: RunCounts;
  hiddenEnded: number;
}

const isString = (value: unknown): value is string => typeof value === "string";
const isNumber = (value: unknown): value is number => typeof value === "number";
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

export function isLive(value: unknown): value is Live {
  return (
    isRecord(value) &&
    isString(value.run) &&
    isString(value.short) &&
    isString(value.agent) &&
    isRunState(value.state) &&
    isNumber(value.done) &&
    isNumber(value.total) &&
    isString(value.step) &&
    isString(value.permission) &&
    isString(value.waitingOn) &&
    (value.question === "" || isQuestionKind(value.question)) &&
    typeof value.questionAnswered === "boolean" &&
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
    typeof value.noLiveSession === "boolean" &&
    typeof value.dirty === "boolean" &&
    typeof value.noHandoff === "boolean" &&
    isRecord(value.subagents) &&
    isNumber(value.subagents.done) &&
    isNumber(value.subagents.running) &&
    isNumber(value.subagents.needsYou)
  );
}

const QUESTION_KINDS: QuestionKind[] = [
  "question",
  "decision",
  "review",
  "blocked",
];
const isQuestionKind = (value: unknown): value is QuestionKind =>
  QUESTION_KINDS.includes(value as QuestionKind);

export function isQuestion(value: unknown): value is Question {
  return (
    isRecord(value) &&
    isString(value.id) &&
    isString(value.run) &&
    isQuestionKind(value.kind) &&
    isString(value.text) &&
    isString(value.asked) &&
    (value.options === undefined ||
      (Array.isArray(value.options) && value.options.every(isString))) &&
    (value.answer === undefined || isString(value.answer))
  );
}

function isEntry(value: unknown): value is TimelineEntry {
  return isRecord(value) && isString(value.time) && isString(value.kind);
}

export function isRun(value: unknown): value is Run {
  return (
    isRecord(value) &&
    isString(value.id) &&
    isString(value.short) &&
    isString(value.agent) &&
    (value.kind === "session" || value.kind === "subagent") &&
    isString(value.parent) &&
    Array.isArray(value.children) &&
    isString(value.project) &&
    isString(value.branch) &&
    isRunState(value.state) &&
    isString(value.ticket) &&
    isString(value.ticketTitle) &&
    typeof value.dirty === "boolean" &&
    typeof value.noHandoff === "boolean" &&
    isString(value.permission) &&
    isString(value.lastActivity) &&
    isNumber(value.tools) &&
    isNumber(value.edits) &&
    Array.isArray(value.files) &&
    Array.isArray(value.plan) &&
    isRecord(value.progress) &&
    Array.isArray(value.questions) &&
    value.questions.every(isQuestion) &&
    (value.timeline === undefined ||
      (Array.isArray(value.timeline) && value.timeline.every(isEntry)))
  );
}

const isRuns = (value: unknown): value is RunsResponse =>
  isRecord(value) &&
  isNumber(value.revision) &&
  Array.isArray(value.runs) &&
  value.runs.every(isRun) &&
  isRunCounts(value.counts) &&
  isNumber(value.hiddenEnded);

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
    (value: unknown): value is { run: Run } =>
      isRecord(value) && isRun(value.run),
    "Run response was invalid",
    signal,
  );
  return response.run;
}
