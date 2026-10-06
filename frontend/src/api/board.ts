import { type AuthenticatedFetch, getJSON, isRecord } from "./client";
import {
  isLive,
  isQuestion,
  isRun,
  isRunCounts,
  type Live,
  type Question,
  type Run,
  type RunCounts,
} from "./runs";

export type Column = "backlog" | "up-next" | "in-progress" | "review" | "done";

export const COLUMNS: { id: Column; title: string }[] = [
  { id: "backlog", title: "Backlog" },
  { id: "up-next", title: "Up next" },
  { id: "in-progress", title: "In progress" },
  { id: "review", title: "Ready to review" },
  { id: "done", title: "Done" },
];

// TicketRef names a ticket by id (FH-42); ids are unique across the root.
export interface TicketRef {
  id: string;
}

export const TICKET_ID = /^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$/;

export interface Reason {
  kind: "ticket" | "workstream" | "order";
  text: string;
  ticket?: { project: string; id: string };
  workstream?: string;
  column?: string;
  pending?: number;
  missing: boolean;
  via?: string;
}

export interface Card {
  project: string;
  id: string;
  slug: string;
  column: Column;
  title: string;
  type: string;
  priority: string;
  workstream: string;
  tags: string[];
  created: string;
  updated: string;
  modified: string;
  branch: string;
  dependsOn: string[];
  criteria: { done: number; total: number };
  excerpt: string;
  handoffNext: string;
  attachments: number;
  hasReview: boolean;
  blocked: boolean;
  blockedBy: Reason[];
  needsRepair: string[];
  warnings: string[];
  searchText?: string;
  // live is the linked run that most needs attention (VIEW-8); needsYou and
  // agentWorking place the card in the virtual columns (VIEW-2).
  live?: Live;
  needsYou: boolean;
  agentWorking: boolean;
  // openQuestions counts open questions about the ticket (CARD-6).
  openQuestions: number;
}

export interface WorkstreamBrief {
  slug: string;
  title: string;
  created: string;
  status: "active" | "blocked" | "completed";
  done: number;
  total: number;
}

export interface ProjectSummary {
  name: string;
  displayName: string;
  key: string;
  keyDerived: boolean;
  repos: string[];
  counts: Record<Column, number>;
  needsRepair: number;
  blocked: number;
  stuck: number;
  warnings: string[];
  lastModified: string;
  workstreams: WorkstreamBrief[];
  // archived counts the project's archived tickets (EDIT-8).
  archived: number;
  runs: RunCounts;
}

export interface ProjectsResponse {
  revision: number;
  root: string;
  rootMissing: boolean;
  v1Projects: string[];
  migrateCommand: string;
  projects: ProjectSummary[];
  // archivedProjects counts projects under <root>/.archive/ (PRJ-5).
  archivedProjects: number;
  runs: RunCounts;
}

export interface BoardResponse {
  revision: number;
  project: ProjectSummary;
  cards: Card[];
  doneTotal: number;
  doneShown: number;
}

export interface AllBoardResponse {
  revision: number;
  projects: ProjectSummary[];
  cards: Card[];
  doneTotal: number;
  doneShown: number;
}

export interface Attachment {
  file: string;
  caption: string;
  kind: string;
  run: string;
  added: string;
  url: string;
}

export interface TicketDetail extends Card {
  body: string;
  frontmatter: string;
  session: string;
  gitRef: string;
  dependsOnWorkstreams: string[];
  criteriaItems: { text: string; done: boolean }[];
  handoff: { markdown: string; next: string[] } | null;
  review: { markdown: string } | null;
  attachmentFiles: Attachment[];
  // hash is sent back with every edit (STO-3); raw is the whole file for the
  // raw editor. Both are empty on a read-only server.
  hash: string;
  raw: string;
  // runs are the runs linked to the ticket, most recent first (CARD-1).
  runs: Run[];
  // questions are the open questions about the ticket (CARD-6).
  questions: Question[];
}

export interface WorkstreamTicket {
  id: string;
  title: string;
  column: Column | "archived" | "";
  blocked: boolean;
  held: boolean;
  missing: boolean;
}

export interface Workstream {
  slug: string;
  title: string;
  status: "active" | "blocked" | "completed";
  suspended: boolean;
  declaredStatus: string;
  priority: string;
  created: string;
  done: number;
  total: number;
  next: string;
  blockedBy: Reason[];
  dependsOnWorkstreams: string[];
  tags: string[];
  needsRepair: string[];
  warnings: string[];
  tickets: WorkstreamTicket[];
}

const isString = (value: unknown): value is string => typeof value === "string";
const isStringArray = (value: unknown): value is string[] =>
  Array.isArray(value) && value.every(isString);
const isColumn = (value: unknown): value is Column =>
  COLUMNS.some((column) => column.id === value);

function isReason(value: unknown): value is Reason {
  return (
    isRecord(value) &&
    (value.kind === "ticket" ||
      value.kind === "workstream" ||
      value.kind === "order") &&
    isString(value.text) &&
    typeof value.missing === "boolean"
  );
}

export function isCard(value: unknown): value is Card {
  return (
    isRecord(value) &&
    isString(value.project) &&
    isString(value.id) &&
    isString(value.slug) &&
    isColumn(value.column) &&
    isString(value.title) &&
    isString(value.type) &&
    isString(value.priority) &&
    isString(value.workstream) &&
    isStringArray(value.tags) &&
    isString(value.modified) &&
    isRecord(value.criteria) &&
    typeof value.criteria.done === "number" &&
    typeof value.criteria.total === "number" &&
    isString(value.excerpt) &&
    isString(value.handoffNext) &&
    typeof value.blocked === "boolean" &&
    Array.isArray(value.blockedBy) &&
    value.blockedBy.every(isReason) &&
    isStringArray(value.needsRepair) &&
    isStringArray(value.warnings) &&
    (value.live === undefined || isLive(value.live)) &&
    typeof value.needsYou === "boolean" &&
    typeof value.agentWorking === "boolean" &&
    typeof value.openQuestions === "number"
  );
}

function isSummary(value: unknown): value is ProjectSummary {
  return (
    isRecord(value) &&
    isString(value.name) &&
    isString(value.displayName) &&
    isString(value.key) &&
    isRecord(value.counts) &&
    COLUMNS.every(
      (column) =>
        typeof (value.counts as Record<string, unknown>)[column.id] ===
        "number",
    ) &&
    typeof value.needsRepair === "number" &&
    typeof value.blocked === "number" &&
    typeof value.archived === "number" &&
    isString(value.lastModified) &&
    Array.isArray(value.workstreams) &&
    isRunCounts(value.runs)
  );
}

const isProjects = (value: unknown): value is ProjectsResponse =>
  isRecord(value) &&
  typeof value.revision === "number" &&
  isString(value.root) &&
  typeof value.rootMissing === "boolean" &&
  isStringArray(value.v1Projects) &&
  isString(value.migrateCommand) &&
  Array.isArray(value.projects) &&
  value.projects.every(isSummary) &&
  typeof value.archivedProjects === "number" &&
  isRunCounts(value.runs);

const isBoard = (value: unknown): value is BoardResponse =>
  isRecord(value) &&
  typeof value.revision === "number" &&
  isSummary(value.project) &&
  Array.isArray(value.cards) &&
  value.cards.every(isCard) &&
  typeof value.doneTotal === "number";

const isAllBoard = (value: unknown): value is AllBoardResponse =>
  isRecord(value) &&
  typeof value.revision === "number" &&
  Array.isArray(value.projects) &&
  value.projects.every(isSummary) &&
  Array.isArray(value.cards) &&
  value.cards.every(isCard) &&
  typeof value.doneTotal === "number";

const isTicket = (
  value: unknown,
): value is { revision: number; ticket: TicketDetail } =>
  isRecord(value) &&
  isRecord(value.ticket) &&
  isCard(value.ticket) &&
  isString(value.ticket.body) &&
  Array.isArray(value.ticket.criteriaItems) &&
  Array.isArray(value.ticket.questions) &&
  value.ticket.questions.every(isQuestion) &&
  Array.isArray(value.ticket.attachmentFiles) &&
  Array.isArray(value.ticket.runs) &&
  value.ticket.runs.every(isRun);

const isWorkstreams = (
  value: unknown,
): value is { revision: number; workstreams: Workstream[] } =>
  isRecord(value) &&
  Array.isArray(value.workstreams) &&
  value.workstreams.every(
    (item) =>
      isRecord(item) &&
      isString(item.slug) &&
      isString(item.status) &&
      Array.isArray(item.tickets),
  );

export const segment = encodeURIComponent;

export function fetchProjects(
  fetcher: AuthenticatedFetch,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    "/api/projects",
    isProjects,
    "Project list response was invalid",
    signal,
  );
}

export function fetchProjectBoard(
  fetcher: AuthenticatedFetch,
  project: string,
  doneAll: boolean,
  signal?: AbortSignal,
) {
  const query = doneAll ? "?done=all" : "";
  return getJSON(
    fetcher,
    `/api/projects/${segment(project)}/board${query}`,
    isBoard,
    "Board response was invalid",
    signal,
  );
}

export function fetchAllBoard(
  fetcher: AuthenticatedFetch,
  doneAll: boolean,
  signal?: AbortSignal,
) {
  const query = doneAll ? "?done=all" : "";
  return getJSON(
    fetcher,
    `/api/all/board${query}`,
    isAllBoard,
    "Board response was invalid",
    signal,
  );
}

export async function fetchTicket(
  fetcher: AuthenticatedFetch,
  id: string,
  signal?: AbortSignal,
): Promise<TicketDetail> {
  const response = await getJSON(
    fetcher,
    `/api/tickets/${segment(id)}`,
    isTicket,
    "Ticket response was invalid",
    signal,
  );
  return response.ticket;
}

export async function fetchWorkstreams(
  fetcher: AuthenticatedFetch,
  project: string,
  signal?: AbortSignal,
): Promise<Workstream[]> {
  const response = await getJSON(
    fetcher,
    `/api/projects/${segment(project)}/workstreams`,
    isWorkstreams,
    "Workstreams response was invalid",
    signal,
  );
  return response.workstreams;
}

// splitReasons separates real blockers (a dependency, a depended-on
// workstream, a missing reference) from waits on an earlier station of a
// line, which the board shows quietly.
export function splitReasons(reasons: Reason[]): {
  blockers: Reason[];
  waits: Reason[];
} {
  return {
    blockers: reasons.filter((reason) => reason.kind !== "order"),
    waits: reasons.filter((reason) => reason.kind === "order"),
  };
}
