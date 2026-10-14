// Writes to the board (board-editing). Every ticket edit sends the hash the
// client read; a stale hash fails with a conflict carrying the current file
// (STO-3, EDIT-7).
import { type Column, isComment, type Status, segment } from "./board";
import {
  ApiError,
  type AuthenticatedFetch,
  getJSON,
  isRecord,
  sendJSON,
} from "./client";

export interface Saved {
  hash: string;
  revision: number;
  warnings: string[];
}

function isSaved(value: unknown): value is Saved {
  return (
    isRecord(value) &&
    typeof value.hash === "string" &&
    typeof value.revision === "number" &&
    Array.isArray(value.warnings)
  );
}

async function save(
  fetcher: AuthenticatedFetch,
  method: "POST" | "PUT" | "PATCH",
  path: string,
  body?: unknown,
): Promise<Saved> {
  const payload = await sendJSON(fetcher, method, path, body);
  if (!isSaved(payload)) throw new Error("Save response was invalid");
  return payload;
}

const ticketPath = (id: string) => `/api/tickets/${segment(id)}`;

// moveTicket moves a ticket to a column. With after it is also placed
// directly after that ticket ("" for the top; EDIT-9); without, it keeps its
// place in the saved order.
export function moveTicket(
  fetcher: AuthenticatedFetch,
  id: string,
  to: Status,
  base: string,
  reason = "",
  after?: string,
) {
  return save(fetcher, "POST", `${ticketPath(id)}/move`, {
    base,
    to,
    reason,
    ...(after === undefined ? {} : { after }),
  });
}

export type FieldValue = string | string[];

export function patchTicket(
  fetcher: AuthenticatedFetch,
  id: string,
  base: string,
  fields: Record<string, FieldValue>,
) {
  return save(fetcher, "PATCH", ticketPath(id), { base, fields });
}

// saveSection replaces one section's text, adding a missing template
// section in its place (FH-66); Handoff and Notes are refused.
export function saveSection(
  fetcher: AuthenticatedFetch,
  id: string,
  base: string,
  section: string,
  content: string,
) {
  return save(fetcher, "PUT", `${ticketPath(id)}/section`, {
    base,
    section,
    content,
  });
}

// saveWorkstreamSection replaces one section of a workstream's text,
// adding a missing template section in its place (FH-139).
export function saveWorkstreamSection(
  fetcher: AuthenticatedFetch,
  project: string,
  slug: string,
  base: string,
  section: string,
  content: string,
) {
  return save(
    fetcher,
    "PUT",
    `/api/projects/${segment(project)}/workstreams/${segment(slug)}/section`,
    { base, section, content },
  );
}

// Renamed is a workstream's name saved: its slug after the save, which the
// name may have changed (FH-167).
export type Renamed = Saved & { slug: string };

// nameWorkstream sets a workstream's name (its title) and renames its slug
// to the one the name makes, as New workstream makes it, rewriting every
// reference (FH-167, FH-158): the server derives the slug. A name whose
// slug another workstream has is refused (409 exists).
export async function nameWorkstream(
  fetcher: AuthenticatedFetch,
  project: string,
  slug: string,
  base: string,
  title: string,
): Promise<Renamed> {
  const payload = await sendJSON(
    fetcher,
    "PATCH",
    `/api/projects/${segment(project)}/workstreams/${segment(slug)}`,
    { base, fields: { title }, slugFromTitle: true },
  );
  if (
    !isRecord(payload) ||
    typeof payload.slug !== "string" ||
    typeof payload.hash !== "string" ||
    typeof payload.revision !== "number"
  )
    throw new Error("Save response was invalid");
  return {
    slug: payload.slug,
    hash: payload.hash,
    revision: payload.revision,
    warnings: [],
  };
}

export function saveRaw(
  fetcher: AuthenticatedFetch,
  id: string,
  base: string,
  content: string,
) {
  return save(fetcher, "PUT", `${ticketPath(id)}/raw`, { base, content });
}

export function setCriterion(
  fetcher: AuthenticatedFetch,
  id: string,
  base: string,
  index: number,
  checked: boolean,
) {
  return save(fetcher, "POST", `${ticketPath(id)}/criteria`, {
    base,
    index,
    checked,
  });
}

// setReviewStep ticks or clears a How to Verify step of a ticket's review
// (REV-3); base is the review's hash.
export function setReviewStep(
  fetcher: AuthenticatedFetch,
  id: string,
  base: string,
  index: number,
  checked: boolean,
) {
  return save(fetcher, "POST", `${ticketPath(id)}/review/steps`, {
    base,
    index,
    checked,
  });
}

// answerQuestion answers an agent's question (RUN-8); the answer reaches
// the session with its next prompt.
export function answerQuestion(
  fetcher: AuthenticatedFetch,
  id: string,
  answer: string,
) {
  return save(fetcher, "POST", `/api/questions/${segment(id)}/answer`, {
    answer,
  });
}

// NewComment is a person's comment as posted: its text and, optionally,
// the part of the ticket it quotes and where that is from (STO-8).
export interface NewComment {
  body: string;
  quote?: string;
  quoteFrom?: string;
  // sendTo also sends the comment to that run now (FH-127): a live
  // session holding the ticket, or one of its subagents.
  sendTo?: string;
}

// Posted reports a new comment as a save, with the run it was sent to and
// its short id (agent-protocol §2; "" when it was not, or could not be
// sent: warnings say why).
export type Posted = Saved & { sentTo: string; sentToShort: string };

// commentSave sends a comment write and reads the activity's new hash as
// a save's (Saved), so a comment edit goes through the same draft and
// conflict handling as a ticket edit.
async function commentSave(
  fetcher: AuthenticatedFetch,
  method: "POST" | "PATCH",
  path: string,
  body: unknown,
): Promise<Posted> {
  const payload = await sendJSON(fetcher, method, path, body);
  if (
    !isRecord(payload) ||
    !isComment(payload.comment) ||
    typeof payload.activityHash !== "string" ||
    typeof payload.revision !== "number"
  )
    throw new Error("Comment response was invalid");
  return {
    hash: payload.activityHash,
    revision: payload.revision,
    warnings: Array.isArray(payload.warnings)
      ? payload.warnings.filter(
          (item): item is string => typeof item === "string",
        )
      : [],
    sentTo: typeof payload.sentTo === "string" ? payload.sentTo : "",
    sentToShort:
      typeof payload.sentToShort === "string" ? payload.sentToShort : "",
  };
}

// addComment appends a comment to the ticket's activity (D47); comments
// are appended as the file is, with no precondition.
export function addComment(
  fetcher: AuthenticatedFetch,
  id: string,
  comment: NewComment,
) {
  return commentSave(fetcher, "POST", `${ticketPath(id)}/comments`, {
    body: comment.body,
    quote: comment.quote ?? "",
    quoteFrom: comment.quoteFrom ?? "",
    ...(comment.sendTo ? { sendTo: comment.sendTo } : {}),
  });
}

// editComment replaces a comment's text on base, the hash of the activity
// it was read from; a stale base is a conflict carrying the current file.
export function editComment(
  fetcher: AuthenticatedFetch,
  id: string,
  comment: string,
  base: string,
  body: string,
) {
  return commentSave(
    fetcher,
    "PATCH",
    `${ticketPath(id)}/comments/${segment(comment)}`,
    { base, body },
  );
}

// deleteComment deletes a comment the person posted by mistake (FH-140) on
// base, the hash of the activity it was read from. A stale base is a
// conflict; a comment its session has read (`delivered`) or a change
// request not resolved (`open_request`) is refused with the reason.
export async function deleteComment(
  fetcher: AuthenticatedFetch,
  id: string,
  comment: string,
  base: string,
): Promise<Saved> {
  const payload = await sendJSON(
    fetcher,
    "DELETE",
    `${ticketPath(id)}/comments/${segment(comment)}`,
    { base },
  );
  if (
    !isRecord(payload) ||
    typeof payload.activityHash !== "string" ||
    typeof payload.revision !== "number"
  )
    throw new Error("Delete response was invalid");
  return {
    hash: payload.activityHash,
    revision: payload.revision,
    warnings: [],
  };
}

// Sent reports a change request write (REV-6, FH-126): the board revision,
// how many requests it covered, the short id of the run they were sent to
// (agent-protocol §2; "" when no live session holds the ticket, so the next
// one finds them on it) and any warning.
export interface Sent {
  revision: number;
  count: number;
  sentToShort: string;
  warnings: string[];
}

function isSent(payload: unknown, count: number): Sent {
  if (
    !isRecord(payload) ||
    typeof payload.revision !== "number" ||
    (payload.sentToShort !== undefined &&
      typeof payload.sentToShort !== "string")
  )
    throw new Error("Change request response was invalid");
  const warnings = Array.isArray(payload.warnings)
    ? payload.warnings.filter(
        (item): item is string => typeof item === "string",
      )
    : [];
  return {
    revision: payload.revision,
    count,
    sentToShort:
      typeof payload.sentToShort === "string" ? payload.sentToShort : "",
    warnings,
  };
}

// requestChanges asks for changes on a ticket in review (REV-6): the
// comments named become change requests, comment (when given) is added as
// one, and the ticket goes back to In progress on its branch.
export async function requestChanges(
  fetcher: AuthenticatedFetch,
  id: string,
  comments: string[],
  comment?: NewComment,
): Promise<Sent> {
  const payload = await sendJSON(
    fetcher,
    "POST",
    `${ticketPath(id)}/request-changes`,
    {
      comments,
      ...(comment
        ? {
            comment: {
              body: comment.body,
              quote: comment.quote ?? "",
              quoteFrom: comment.quoteFrom ?? "",
            },
          }
        : {}),
    },
  );
  const count =
    isRecord(payload) && Array.isArray(payload.comments)
      ? payload.comments.length
      : 0;
  return isSent(payload, count);
}

// RequestState is a state the person moves a change request to: resolved,
// or change-requested to reopen it.
export type RequestState = "resolved" | "change-requested";

// setRequestState resolves or reopens a change request (REV-6); the change
// merges with the activity as it is, as a tick does.
export async function setRequestState(
  fetcher: AuthenticatedFetch,
  id: string,
  comment: string,
  state: RequestState,
): Promise<Sent> {
  const payload = await sendJSON(
    fetcher,
    "PATCH",
    `${ticketPath(id)}/comments/${segment(comment)}`,
    { state },
  );
  return isSent(payload, 1);
}

export function archiveTicket(fetcher: AuthenticatedFetch, id: string) {
  return save(fetcher, "POST", `${ticketPath(id)}/archive`);
}

export function unarchiveTicket(fetcher: AuthenticatedFetch, id: string) {
  return save(fetcher, "POST", `${ticketPath(id)}/unarchive`);
}

export interface NewTicket {
  title: string;
  type: string;
  priority: string;
  status: Column;
  workstream: string;
  description: string;
  criteria: string[];
  dependsOn: string[];
  tags: string[];
  key: string;
}

export interface Created {
  id: string;
  hash: string;
  key: string;
  revision: number;
}

export async function createTicket(
  fetcher: AuthenticatedFetch,
  project: string,
  ticket: NewTicket,
): Promise<Created> {
  const payload = await sendJSON(
    fetcher,
    "POST",
    `/api/projects/${segment(project)}/tickets`,
    ticket,
  );
  if (!isRecord(payload) || typeof payload.id !== "string")
    throw new Error("Create response was invalid");
  return payload as unknown as Created;
}

export interface NewWorkstream {
  title: string;
  goal: string;
  priority: string;
  dependsOnWorkstreams: string[];
}

export interface CreatedWorkstream {
  slug: string;
  hash: string;
  revision: number;
}

// createWorkstream writes a workstream from the board-format template; the
// server makes its slug from the title (FH-17).
export async function createWorkstream(
  fetcher: AuthenticatedFetch,
  project: string,
  workstream: NewWorkstream,
): Promise<CreatedWorkstream> {
  const payload = await sendJSON(
    fetcher,
    "POST",
    `/api/projects/${segment(project)}/workstreams`,
    workstream,
  );
  if (!isRecord(payload) || typeof payload.slug !== "string")
    throw new Error("Create response was invalid");
  return payload as unknown as CreatedWorkstream;
}

export function reorderWorkstream(
  fetcher: AuthenticatedFetch,
  project: string,
  slug: string,
  tickets: string[],
) {
  return save(
    fetcher,
    "PUT",
    `/api/projects/${segment(project)}/workstreams/${segment(slug)}/order`,
    { tickets },
  );
}

export function setProjectKey(
  fetcher: AuthenticatedFetch,
  project: string,
  key: string,
) {
  return save(fetcher, "PUT", `/api/projects/${segment(project)}/key`, {
    key,
  });
}

// Conflict is the current file returned with a stale-hash 409.
export interface Conflict {
  hash: string;
  content: string;
}

export function conflictOf(error: unknown): Conflict | undefined {
  if (!(error instanceof ApiError) || error.code !== "conflict") return;
  const current = error.payload.current;
  if (
    isRecord(current) &&
    typeof current.hash === "string" &&
    typeof current.content === "string"
  )
    return { hash: current.hash, content: current.content };
  return { hash: "", content: "" };
}

// blockedReasons returns the reasons a move into In progress needs
// confirming (EDIT-2), or undefined for any other error.
export function blockedReasons(error: unknown): string[] | undefined {
  if (!(error instanceof ApiError) || error.code !== "confirm_blocked") return;
  const body = isRecord(error.payload.error) ? error.payload.error : {};
  return Array.isArray(body.blockedBy)
    ? body.blockedBy.filter((item): item is string => typeof item === "string")
    : [];
}

export function keysInUse(error: unknown): string[] | undefined {
  if (!(error instanceof ApiError) || error.code !== "key_taken") return;
  const body = isRecord(error.payload.error) ? error.payload.error : {};
  return Array.isArray(body.inUse)
    ? body.inUse.filter((item): item is string => typeof item === "string")
    : [];
}

// Live updates (LIFE-3): resolves with the revision once it moves past
// since, or the same revision when the server's wait ends.
export async function waitForChanges(
  fetcher: AuthenticatedFetch,
  since: number,
  signal?: AbortSignal,
): Promise<number> {
  const response = await getJSON(
    fetcher,
    `/api/changes?since=${since}`,
    (value): value is { revision: number } =>
      isRecord(value) && typeof value.revision === "number",
    "Changes response was invalid",
    signal,
  );
  return response.revision;
}
