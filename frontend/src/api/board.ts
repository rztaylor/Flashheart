import { type AuthenticatedFetch, getJSON, isRecord } from "./client";
import {
  isLive,
  isQuestion,
  isRun,
  isRunCounts,
  isTicketSessions,
  type Live,
  type Question,
  type Run,
  type RunCounts,
  type TicketSessions,
} from "./runs";

export type Column = "backlog" | "up-next" | "in-progress" | "review" | "done";

export const COLUMNS: { id: Column; title: string }[] = [
  { id: "backlog", title: "Backlog" },
  { id: "up-next", title: "Up next" },
  { id: "in-progress", title: "In progress" },
  { id: "review", title: "Ready to review" },
  { id: "done", title: "Done" },
];

// Status is a ticket's status value: its column, or rejected, a ticket
// closed without being accepted, which sits in Done (FH-58).
export type Status = Column | "rejected";

// STATUSES are the statuses a ticket can be moved to, in workflow order.
export const STATUSES: { id: Status; title: string }[] = [
  ...COLUMNS,
  { id: "rejected", title: "Rejected" },
];

// statusWord is a ticket's status: rejected, or else its column.
export const statusWord = (ticket: {
  column: Column;
  rejected?: boolean;
}): Status => (ticket.rejected ? "rejected" : ticket.column);

// columnOf is the column a status puts a ticket in.
export const columnOf = (status: Status): Column =>
  status === "rejected" ? "done" : status;

// TicketRef names a ticket by id (FH-42); ids are unique across the root.
export interface TicketRef {
  id: string;
}

export const TICKET_ID = /^[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*$/;

export interface Reason {
  kind: "ticket" | "workstream";
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
  // rejected marks a ticket in Done whose status is rejected (FH-58).
  rejected?: boolean;
  title: string;
  type: string;
  priority: string;
  workstream: string;
  tags: string[];
  created: string;
  updated: string;
  modified: string;
  branch: string;
  // pr is the ticket's pull request as written (FH-114): free text, linked
  // only when it is an http(s) URL.
  pr: string;
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
  // live is the linked run that most needs attention (VIEW-8); needsYou
  // answers the band's Needs you filter (VIEW-2, FH-44), and agentWorking
  // answers the Agent working State filter (FH-42).
  live?: Live;
  needsYou: boolean;
  agentWorking: boolean;
  // openQuestions counts open questions about the ticket (CARD-6).
  openQuestions: number;
  // changeRequests counts its change requests still open (waiting for an
  // agent) or addressed (waiting for the person; REV-6, FH-126); absent
  // from an older server.
  changeRequests?: ChangeRequests;
  // sessions summarises the runs working for the ticket (RUN-9); absent
  // when none is linked and the ticket is not in progress.
  sessions?: TicketSessions;
}

// ChangeRequests counts a ticket's change requests (REV-6): open waits
// for an agent to make it, addressed for the person to resolve or reopen.
export interface ChangeRequests {
  open: number;
  addressed: number;
}

const isChangeRequests = (value: unknown): value is ChangeRequests =>
  isRecord(value) &&
  typeof value.open === "number" &&
  typeof value.addressed === "number";

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
  warnings: string[];
  lastModified: string;
  workstreams: WorkstreamBrief[];
  // archived counts the project's archived tickets (EDIT-8).
  archived: number;
  // rejected counts the tickets in Done whose status is rejected (FH-58).
  rejected: number;
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

// Review is a ticket's review file (REV-3), split around its How to Verify
// steps; with no steps only markdown matters. hash guards ticking a step
// and is empty on a read-only server.
export interface Review {
  markdown: string;
  hash: string;
  before: string;
  intro: string;
  steps: { text: string; done: boolean }[];
  outro: string;
  after: string;
}

export interface TicketDetail extends Card {
  body: string;
  frontmatter: string;
  session: string;
  gitRef: string;
  dependsOnWorkstreams: string[];
  criteriaItems: { text: string; done: boolean }[];
  handoff: { markdown: string; next: string[] } | null;
  review: Review | null;
  attachmentFiles: Attachment[];
  // hash is sent back with every edit (STO-3); raw is the whole file for the
  // raw editor. Both are empty on a read-only server.
  hash: string;
  raw: string;
  // runs are the runs linked to the ticket, most recent first (CARD-1).
  runs: Run[];
  // questions are the open questions about the ticket (CARD-6).
  questions: Question[];
  // comments and history are the ticket's activity file, oldest first
  // (STO-8, D47); activityHash guards a comment edit and is empty on a
  // read-only server or while the ticket has no activity file.
  comments: Comment[];
  history: History[];
  activityHash: string;
}

// Comment is a person's comment on a ticket (STO-8). edited is "" until it
// is edited; quote is the part of the ticket it quotes ("" for none) and
// quoteFrom where that part is ("Acceptance criteria 2").
export interface Comment {
  id: string;
  time: string;
  by: string;
  byPerson: boolean;
  state: string;
  edited: string;
  quoteFrom: string;
  quote: string;
  body: string;
}

// HistoryChange is one thing a write changed: what is created, moved,
// edited, ticked, unticked, archived or restored, or a comment's state
// (change-requested, addressed, resolved, reopened; FH-126); from and to
// are statuses, fields the names edited, criteria and steps 1-based
// numbers, comments the ids of the comments whose state changed.
export interface HistoryChange {
  what: string;
  from: string;
  to: string;
  fields: string[];
  criteria: number[];
  steps: number[];
  comments?: string[];
}

// History is one write that changed the ticket: who, when and what, never
// the ticket's text (board-format §Activity).
export interface History {
  id: string;
  time: string;
  by: string;
  byPerson: boolean;
  changes: HistoryChange[];
}

export interface WorkstreamTicket {
  id: string;
  title: string;
  column: Column | "archived" | "";
  rejected?: boolean;
  blocked: boolean;
  held: boolean;
  missing: boolean;
  // dependsOn: tickets on this workstream it depends on (the graph's edges);
  // outside: its unfinished dependencies on tickets elsewhere.
  dependsOn: string[];
  outside: string[];
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
    (value.kind === "ticket" || value.kind === "workstream") &&
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
    typeof value.openQuestions === "number" &&
    (value.changeRequests === undefined ||
      isChangeRequests(value.changeRequests)) &&
    (value.sessions === undefined || isTicketSessions(value.sessions))
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
    typeof value.rejected === "number" &&
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

const isReview = (value: unknown): value is Review =>
  isRecord(value) &&
  isString(value.markdown) &&
  isString(value.hash) &&
  isString(value.before) &&
  isString(value.intro) &&
  isString(value.outro) &&
  isString(value.after) &&
  Array.isArray(value.steps) &&
  value.steps.every(
    (step) =>
      isRecord(step) && isString(step.text) && typeof step.done === "boolean",
  );

const isNumberArray = (value: unknown): value is number[] =>
  Array.isArray(value) && value.every((item) => typeof item === "number");

export const isComment = (value: unknown): value is Comment =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.time) &&
  isString(value.by) &&
  typeof value.byPerson === "boolean" &&
  isString(value.state) &&
  isString(value.edited) &&
  isString(value.quoteFrom) &&
  isString(value.quote) &&
  isString(value.body);

const isHistory = (value: unknown): value is History =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.time) &&
  isString(value.by) &&
  typeof value.byPerson === "boolean" &&
  Array.isArray(value.changes) &&
  value.changes.every(
    (change) =>
      isRecord(change) &&
      isString(change.what) &&
      isString(change.from) &&
      isString(change.to) &&
      isStringArray(change.fields) &&
      isNumberArray(change.criteria) &&
      isNumberArray(change.steps) &&
      (change.comments === undefined || isStringArray(change.comments)),
  );

const isTicket = (
  value: unknown,
): value is { revision: number; ticket: TicketDetail } =>
  isRecord(value) &&
  isRecord(value.ticket) &&
  isCard(value.ticket) &&
  Array.isArray(value.ticket.comments) &&
  value.ticket.comments.every(isComment) &&
  Array.isArray(value.ticket.history) &&
  value.ticket.history.every(isHistory) &&
  isString(value.ticket.activityHash) &&
  isString(value.ticket.body) &&
  Array.isArray(value.ticket.criteriaItems) &&
  Array.isArray(value.ticket.questions) &&
  value.ticket.questions.every(isQuestion) &&
  Array.isArray(value.ticket.attachmentFiles) &&
  (value.ticket.review === null || isReview(value.ticket.review)) &&
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
