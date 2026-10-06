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
  isString(value.type) &&
  isString(value.priority) &&
  isString(value.workstream) &&
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
  isString(value.title) &&
  isString(value.token) &&
  Array.isArray(value.files) &&
  value.files.every(isString) &&
  Array.isArray(value.tickets) &&
  value.tickets.every(
    (item) =>
      isRecord(item) &&
      isString(item.id) &&
      isString(item.project) &&
      isString(item.title),
  ) &&
  Array.isArray(value.workstreams) &&
  value.workstreams.every(
    (item) => isRecord(item) && isString(item.slug) && isString(item.title),
  );

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

// Projects (PRJ-5): what is live before archiving one, the archived list,
// restore, and the permanent delete with its preview.

export interface ArchiveCheck {
  name: string;
  displayName: string;
  tickets: number;
  counts: Record<string, number>;
  liveRuns: number;
  claimed: number;
  openQuestions: number;
}

export interface ArchivedProject {
  name: string;
  displayName: string;
  key: string;
  repos: string[];
  tickets: number;
  archived: string;
}

export interface ProjectDeletePlan {
  name: string;
  displayName: string;
  key: string;
  tickets: number;
  references: {
    project: string;
    id: string;
    title: string;
    dependsOn: string[];
  }[];
  token: string;
}

const isArchiveCheck = (value: unknown): value is ArchiveCheck =>
  isRecord(value) &&
  isString(value.name) &&
  isString(value.displayName) &&
  typeof value.tickets === "number" &&
  isRecord(value.counts) &&
  typeof value.liveRuns === "number" &&
  typeof value.claimed === "number" &&
  typeof value.openQuestions === "number";

const isArchivedProject = (value: unknown): value is ArchivedProject =>
  isRecord(value) &&
  isString(value.name) &&
  isString(value.displayName) &&
  isString(value.key) &&
  Array.isArray(value.repos) &&
  typeof value.tickets === "number" &&
  isString(value.archived);

const isArchivedProjects = (
  value: unknown,
): value is { revision: number; projects: ArchivedProject[] } =>
  isRecord(value) &&
  typeof value.revision === "number" &&
  Array.isArray(value.projects) &&
  value.projects.every(isArchivedProject);

const isProjectDeletePlan = (value: unknown): value is ProjectDeletePlan =>
  isRecord(value) &&
  isString(value.name) &&
  isString(value.displayName) &&
  isString(value.key) &&
  typeof value.tickets === "number" &&
  isString(value.token) &&
  Array.isArray(value.references) &&
  value.references.every(
    (item) =>
      isRecord(item) &&
      isString(item.id) &&
      isString(item.project) &&
      isString(item.title) &&
      Array.isArray(item.dependsOn),
  );

const archivedProjectPath = (project: string) =>
  `/api/archived-projects/${segment(project)}`;

export function fetchArchiveCheck(
  fetcher: AuthenticatedFetch,
  project: string,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    `/api/projects/${segment(project)}/archive-check`,
    isArchiveCheck,
    "Archive check was invalid",
    signal,
  );
}

export async function archiveProject(
  fetcher: AuthenticatedFetch,
  project: string,
) {
  await sendJSON(
    fetcher,
    "POST",
    `/api/projects/${segment(project)}/archive-project`,
  );
}

export function fetchArchivedProjects(
  fetcher: AuthenticatedFetch,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    "/api/archived-projects",
    isArchivedProjects,
    "Archived projects response was invalid",
    signal,
  );
}

export async function restoreProject(
  fetcher: AuthenticatedFetch,
  project: string,
) {
  await sendJSON(fetcher, "POST", `${archivedProjectPath(project)}/restore`);
}

export function fetchProjectDeletePlan(
  fetcher: AuthenticatedFetch,
  project: string,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    archivedProjectPath(project),
    isProjectDeletePlan,
    "Delete preview was invalid",
    signal,
  );
}

export async function deleteProject(
  fetcher: AuthenticatedFetch,
  project: string,
  token: string,
  confirm: string,
) {
  await sendJSON(fetcher, "POST", `${archivedProjectPath(project)}/delete`, {
    token,
    confirm,
  });
}
