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
import {
  isLive,
  isNeedsReason,
  isQuestion,
  isRun,
  isRunCounts,
  isRunState,
  isTicketSessions,
  type Live,
  type NeedsReason,
  type Question,
  type Run,
  type RunCounts,
  type RunState,
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
  // reason is why the card needs you (FH-185, D39): its linked run's
  // reason when that is about the ticket, else the oldest open question
  // about it from a live session; kind is "" when it does not.
  reason: NeedsReason;
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
  // image is the URL of the project's badge image (FH-151), with a version
  // that changes with its bytes; absent when the project shows its key.
  image?: string;
}

export interface ProjectsResponse {
  revision: number;
  root: string;
  rootMissing: boolean;
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
  // tellSessions are the runs a comment may be sent to now (FH-127): each
  // live session that has claimed the ticket, then its live subagents;
  // none when no live session holds it.
  tellSessions?: TellSession[];
}

// TellSession is a run a comment may be sent to now: a live session
// holding the ticket, or one of its subagents (parent its session,
// agentType its kind).
export interface TellSession {
  run: string;
  short: string;
  agent: string;
  state: RunState;
  parent: string;
  agentType: string;
}

// CommentDelivery is where a comment sent to a session stands (FH-127):
// waiting for the session to take it, delivered to it (and so locked), or
// missed because the session ended first.
export type CommentDelivery = "waiting" | "delivered" | "missed";

// Comment is a person's comment on a ticket (STO-8). edited is "" until it
// is edited; quote is the part of the ticket it quotes ("" for none) and
// quoteFrom where that part is ("Acceptance criteria 2"). A comment sent
// to a session (FH-127) names its run (sentTo, sentToShort) and where its
// delivery stands, delivered when it was handed over ("" until then).
export interface Comment {
  id: string;
  time: string;
  by: string;
  // byShort names the author as the board shows them: a run by its short
  // id (agent-protocol §2), a person as written.
  byShort: string;
  byPerson: boolean;
  state: string;
  edited: string;
  quoteFrom: string;
  quote: string;
  body: string;
  sentTo: string;
  sentToShort: string;
  delivery: CommentDelivery | "";
  delivered: string;
}

// HistoryChange is one thing a write changed: what is created, moved,
// edited, ticked, unticked, archived or restored, or a comment's state
// (change-requested, addressed, resolved, reopened; FH-126); from and to
// are statuses, fields the names edited, criteria and steps 1-based
// numbers, comments the ids of the comments whose state changed or that
// were deleted (FH-140). A question change (asked, answered,
// answered-in-session, session-ended; FH-190) carries its question.
export interface HistoryChange {
  what: string;
  from: string;
  to: string;
  fields: string[];
  criteria: number[];
  steps: number[];
  comments?: string[];
  question?: QuestionChange;
}

// QuestionChange is an agent's question in the ticket's history (FH-190),
// with its text: the one history that keeps it. kind, options and answer
// are "" or empty where the change does not record them.
export interface QuestionChange {
  id: string;
  kind: string;
  text: string;
  options: string[];
  answer: string;
}

// History is one write that changed the ticket: who, when and what, never
// the ticket's text except a question's (board-format §Activity).
export interface History {
  id: string;
  time: string;
  by: string;
  byShort: string;
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

// WorkstreamDetail is a workstream's page (FH-139): its line, its body,
// and, where the board can save, its file and the hash a section save
// sends ("" when it cannot).
export interface WorkstreamDetail extends Workstream {
  body: string;
  hash: string;
  raw: string;
}

const isColumn = (value: unknown): value is Column =>
  COLUMNS.some((column) => column.id === value);
const optionalString = isOptional(isString);

const isTicketRef = (
  value: unknown,
): value is { project: string; id: string } =>
  isRecord(value) && isString(value.project) && isString(value.id);

function isReason(value: unknown): value is Reason {
  return (
    isRecord(value) &&
    (value.kind === "ticket" || value.kind === "workstream") &&
    isString(value.text) &&
    isOptional(isTicketRef)(value.ticket) &&
    optionalString(value.workstream) &&
    optionalString(value.column) &&
    isOptional(isNumber)(value.pending) &&
    isBoolean(value.missing) &&
    optionalString(value.via)
  );
}

export function isCard(value: unknown): value is Card {
  return (
    isRecord(value) &&
    isString(value.project) &&
    isString(value.id) &&
    isString(value.slug) &&
    isColumn(value.column) &&
    isBoolean(value.rejected) &&
    isString(value.title) &&
    isString(value.type) &&
    isString(value.priority) &&
    isString(value.workstream) &&
    isStringArray(value.tags) &&
    isString(value.created) &&
    isString(value.updated) &&
    isString(value.modified) &&
    isString(value.branch) &&
    isString(value.pr) &&
    isStringArray(value.dependsOn) &&
    isRecord(value.criteria) &&
    isNumber(value.criteria.done) &&
    isNumber(value.criteria.total) &&
    isString(value.excerpt) &&
    isString(value.handoffNext) &&
    isNumber(value.attachments) &&
    isBoolean(value.hasReview) &&
    isBoolean(value.blocked) &&
    isArrayOf(isReason)(value.blockedBy) &&
    isStringArray(value.needsRepair) &&
    isStringArray(value.warnings) &&
    optionalString(value.searchText) &&
    isOptional(isLive)(value.live) &&
    isBoolean(value.needsYou) &&
    isBoolean(value.agentWorking) &&
    isNeedsReason(value.reason) &&
    isNumber(value.openQuestions) &&
    isChangeRequests(value.changeRequests) &&
    isOptional(isTicketSessions)(value.sessions)
  );
}

const isWorkstreamStatus = (
  value: unknown,
): value is WorkstreamBrief["status"] =>
  value === "active" || value === "blocked" || value === "completed";

const isWorkstreamBrief = (value: unknown): value is WorkstreamBrief =>
  isRecord(value) &&
  isString(value.slug) &&
  isString(value.title) &&
  isString(value.created) &&
  isWorkstreamStatus(value.status) &&
  isNumber(value.done) &&
  isNumber(value.total);

function isSummary(value: unknown): value is ProjectSummary {
  return (
    isRecord(value) &&
    isString(value.name) &&
    isString(value.displayName) &&
    isString(value.key) &&
    isBoolean(value.keyDerived) &&
    isStringArray(value.repos) &&
    isRecord(value.counts) &&
    COLUMNS.every(
      (column) =>
        typeof (value.counts as Record<string, unknown>)[column.id] ===
        "number",
    ) &&
    isNumber(value.needsRepair) &&
    isNumber(value.blocked) &&
    isStringArray(value.warnings) &&
    isNumber(value.archived) &&
    isNumber(value.rejected) &&
    isString(value.lastModified) &&
    isArrayOf(isWorkstreamBrief)(value.workstreams) &&
    isRunCounts(value.runs) &&
    isOptional(isString)(value.image)
  );
}

export const isProjects = (value: unknown): value is ProjectsResponse =>
  isRecord(value) &&
  isNumber(value.revision) &&
  isString(value.root) &&
  isBoolean(value.rootMissing) &&
  isArrayOf(isSummary)(value.projects) &&
  isNumber(value.archivedProjects) &&
  isRunCounts(value.runs);

export const isBoard = (value: unknown): value is BoardResponse =>
  isRecord(value) &&
  isNumber(value.revision) &&
  isSummary(value.project) &&
  isArrayOf(isCard)(value.cards) &&
  isNumber(value.doneTotal) &&
  isNumber(value.doneShown);

export const isAllBoard = (value: unknown): value is AllBoardResponse =>
  isRecord(value) &&
  isNumber(value.revision) &&
  isArrayOf(isSummary)(value.projects) &&
  isArrayOf(isCard)(value.cards) &&
  isNumber(value.doneTotal) &&
  isNumber(value.doneShown);

const isCheckItem = (
  value: unknown,
): value is { text: string; done: boolean } =>
  isRecord(value) && isString(value.text) && isBoolean(value.done);

const isReview = (value: unknown): value is Review =>
  isRecord(value) &&
  isString(value.markdown) &&
  isString(value.hash) &&
  isString(value.before) &&
  isString(value.intro) &&
  isString(value.outro) &&
  isString(value.after) &&
  isArrayOf(isCheckItem)(value.steps);

const isHandoff = (
  value: unknown,
): value is NonNullable<TicketDetail["handoff"]> =>
  isRecord(value) && isString(value.markdown) && isStringArray(value.next);

const isAttachment = (value: unknown): value is Attachment =>
  isRecord(value) &&
  isString(value.file) &&
  isString(value.caption) &&
  isString(value.kind) &&
  isString(value.run) &&
  isString(value.added) &&
  isString(value.url);

const isNumberArray = isArrayOf(isNumber);

export const isComment = (value: unknown): value is Comment =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.time) &&
  isString(value.by) &&
  isString(value.byShort) &&
  isBoolean(value.byPerson) &&
  isString(value.state) &&
  isString(value.edited) &&
  isString(value.sentTo) &&
  isString(value.sentToShort) &&
  (value.delivery === "" ||
    value.delivery === "waiting" ||
    value.delivery === "delivered" ||
    value.delivery === "missed") &&
  isString(value.delivered) &&
  isString(value.quoteFrom) &&
  isString(value.quote) &&
  isString(value.body);

const isHistoryChange = (value: unknown): value is HistoryChange =>
  isRecord(value) &&
  isString(value.what) &&
  isString(value.from) &&
  isString(value.to) &&
  isStringArray(value.fields) &&
  isNumberArray(value.criteria) &&
  isNumberArray(value.steps) &&
  isStringArray(value.comments) &&
  isOptional(isQuestionChange)(value.question);

const isQuestionChange = (value: unknown): value is QuestionChange =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.kind) &&
  isString(value.text) &&
  isStringArray(value.options) &&
  isString(value.answer);

const isHistory = (value: unknown): value is History =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.time) &&
  isString(value.by) &&
  isString(value.byShort) &&
  isBoolean(value.byPerson) &&
  isArrayOf(isHistoryChange)(value.changes);

const isTellSession = (value: unknown): value is TellSession =>
  isRecord(value) &&
  isString(value.run) &&
  isString(value.short) &&
  isString(value.agent) &&
  isRunState(value.state) &&
  isString(value.parent) &&
  isString(value.agentType);

const isTicketDetail = (value: unknown): value is TicketDetail =>
  isCard(value) &&
  isRecord(value) &&
  isString(value.body) &&
  isString(value.frontmatter) &&
  isString(value.session) &&
  isString(value.gitRef) &&
  isStringArray(value.dependsOnWorkstreams) &&
  isArrayOf(isCheckItem)(value.criteriaItems) &&
  (value.handoff === null || isHandoff(value.handoff)) &&
  (value.review === null || isReview(value.review)) &&
  isArrayOf(isAttachment)(value.attachmentFiles) &&
  isString(value.hash) &&
  isString(value.raw) &&
  isArrayOf(isRun)(value.runs) &&
  isArrayOf(isQuestion)(value.questions) &&
  isArrayOf(isComment)(value.comments) &&
  isArrayOf(isHistory)(value.history) &&
  isString(value.activityHash) &&
  isArrayOf(isTellSession)(value.tellSessions);

// TicketResponse is GET /api/tickets/{id}.
export interface TicketResponse {
  revision: number;
  ticket: TicketDetail;
}

export const isTicketResponse = (value: unknown): value is TicketResponse =>
  isRecord(value) && isNumber(value.revision) && isTicketDetail(value.ticket);

const isWorkstreamTicket = (value: unknown): value is WorkstreamTicket =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.title) &&
  (value.column === "" ||
    value.column === "archived" ||
    isColumn(value.column)) &&
  isBoolean(value.rejected) &&
  isBoolean(value.blocked) &&
  isBoolean(value.held) &&
  isBoolean(value.missing) &&
  isStringArray(value.dependsOn) &&
  isStringArray(value.outside);

const isWorkstreamLine = (value: unknown): value is Workstream =>
  isRecord(value) &&
  isString(value.slug) &&
  isString(value.title) &&
  isWorkstreamStatus(value.status) &&
  isBoolean(value.suspended) &&
  isString(value.declaredStatus) &&
  isString(value.priority) &&
  isString(value.created) &&
  isNumber(value.done) &&
  isNumber(value.total) &&
  isString(value.next) &&
  isArrayOf(isReason)(value.blockedBy) &&
  isStringArray(value.dependsOnWorkstreams) &&
  isStringArray(value.tags) &&
  isStringArray(value.needsRepair) &&
  isStringArray(value.warnings) &&
  isArrayOf(isWorkstreamTicket)(value.tickets);

export const isWorkstreams = (
  value: unknown,
): value is { revision: number; workstreams: Workstream[] } =>
  isRecord(value) &&
  isNumber(value.revision) &&
  isArrayOf(isWorkstreamLine)(value.workstreams);

export const isWorkstreamResponse = (
  value: unknown,
): value is { revision: number; workstream: WorkstreamDetail } =>
  isRecord(value) &&
  isNumber(value.revision) &&
  isWorkstreamLine(value.workstream) &&
  isRecord(value.workstream) &&
  isString(value.workstream.body) &&
  isString(value.workstream.hash) &&
  isString(value.workstream.raw);

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
    isTicketResponse,
    "Ticket response was invalid",
    signal,
  );
  return response.ticket;
}

export async function fetchWorkstream(
  fetcher: AuthenticatedFetch,
  project: string,
  slug: string,
  signal?: AbortSignal,
): Promise<WorkstreamDetail> {
  const response = await getJSON(
    fetcher,
    `/api/projects/${segment(project)}/workstreams/${segment(slug)}`,
    isWorkstreamResponse,
    "Workstream response was invalid",
    signal,
  );
  return response.workstream;
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
