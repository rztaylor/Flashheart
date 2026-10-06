// Writes to the board (board-editing). Every ticket edit sends the hash the
// client read; a stale hash fails with a conflict carrying the current file
// (STO-3, EDIT-7).
import { type Column, segment } from "./board";
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
  to: Column,
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
