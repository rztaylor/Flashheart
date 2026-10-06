// Archived tickets (EDIT-8): the archive list, the preview of what a
// permanent delete touches, and the delete itself. A delete carries the
// preview's token, so it is refused when anything listed has changed, and
// the id as the user typed it.
import { COLUMNS, type Column, segment } from "./board";
import { type AuthenticatedFetch, getJSON, isRecord, sendJSON } from "./client";

export interface Archived {
  id: string;
  title: string;
  type: string;
  priority: string;
  workstream: string;
  // column is where Restore returns the ticket.
  column: Column;
  archived: string;
  attachments: number;
  hasReview: boolean;
}

export interface ArchiveList {
  revision: number;
  tickets: Archived[];
}

export interface DeletePlan {
  id: string;
  title: string;
  files: string[];
  tickets: { project: string; id: string; title: string }[];
  workstreams: { slug: string; title: string }[];
  token: string;
}

const isString = (value: unknown): value is string => typeof value === "string";

const isArchived = (value: unknown): value is Archived =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.title) &&
  COLUMNS.some((column) => column.id === value.column) &&
  isString(value.archived) &&
  typeof value.attachments === "number" &&
  typeof value.hasReview === "boolean";

const isArchiveList = (value: unknown): value is ArchiveList =>
  isRecord(value) &&
  typeof value.revision === "number" &&
  Array.isArray(value.tickets) &&
  value.tickets.every(isArchived);

const isDeletePlan = (value: unknown): value is DeletePlan =>
  isRecord(value) &&
  isString(value.id) &&
  isString(value.token) &&
  Array.isArray(value.files) &&
  value.files.every(isString) &&
  Array.isArray(value.tickets) &&
  value.tickets.every((item) => isRecord(item) && isString(item.id)) &&
  Array.isArray(value.workstreams) &&
  value.workstreams.every((item) => isRecord(item) && isString(item.slug));

const archivePath = (project: string) =>
  `/api/projects/${segment(project)}/archive`;

export function fetchArchive(
  fetcher: AuthenticatedFetch,
  project: string,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    archivePath(project),
    isArchiveList,
    "Archive response was invalid",
    signal,
  );
}

export function fetchDeletePlan(
  fetcher: AuthenticatedFetch,
  project: string,
  id: string,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    `${archivePath(project)}/${segment(id)}`,
    isDeletePlan,
    "Delete preview was invalid",
    signal,
  );
}

export async function deleteArchived(
  fetcher: AuthenticatedFetch,
  project: string,
  id: string,
  token: string,
  confirm: string,
) {
  await sendJSON(
    fetcher,
    "POST",
    `${archivePath(project)}/${segment(id)}/delete`,
    { token, confirm },
  );
}
