// UI preferences saved through the backend in config.yaml (CFG-2), never in
// browser storage.
import { type AuthenticatedFetch, getJSON, isRecord, sendJSON } from "./client";
import type { ThemePreference } from "./info";

// FilterChoice is one remembered filter dimension (model/filters Choice).
export interface FilterChoice {
  include: string[];
  exclude: string[];
}

export type Density = "compact" | "normal" | "detailed";
export type ColourBy = "type" | "priority" | "age" | "none";
export type SavedView = "" | "board" | "overview" | "workstreams" | "table";
// HideableColumn is a real column the Board can hide (FH-41).
export type HideableColumn = "backlog";
// EditorMode is how a section opens for editing (FH-16): formatted, a live
// preview of the markdown, or the markdown as written.
export type EditorMode = "formatted" | "markdown";
export type SavedState =
  | ""
  | "all"
  | "blocked"
  | "unblocked"
  | "repair"
  | "working"
  | "rejected";
// ScopePreferences is the remembered view and filters of one project, or of
// All projects under the key "all".
export interface ScopePreferences {
  view: SavedView;
  type: FilterChoice;
  priority: FilterChoice;
  workstream: FilterChoice;
  age: FilterChoice;
  state: SavedState;
}

export interface Preferences {
  theme: ThemePreference;
  density: Density;
  colourBy: ColourBy;
  // hiddenColumns lists the real columns hidden from the board (FH-41).
  hiddenColumns: HideableColumn[];
  editor: EditorMode;
  scopes: Record<string, ScopePreferences>;
  // expandedWorkstreams lists per project name the finished workstreams
  // shown expanded on the Workstreams view (FH-68).
  expandedWorkstreams: Record<string, string[]>;
}

export const defaultPreferences: Preferences = {
  theme: "system",
  density: "normal",
  colourBy: "type",
  hiddenColumns: [],
  editor: "formatted",
  scopes: {},
  expandedWorkstreams: {},
};

const oneOf =
  <T extends string>(values: readonly T[]) =>
  (value: unknown): value is T =>
    typeof value === "string" && (values as readonly string[]).includes(value);

const isTheme = oneOf<ThemePreference>(["system", "light", "dark"]);
const isDensity = oneOf<Density>(["compact", "normal", "detailed"]);
const isColourBy = oneOf<ColourBy>(["type", "priority", "age", "none"]);
const isView = oneOf<SavedView>([
  "",
  "board",
  "overview",
  "workstreams",
  "table",
]);
const isHideableColumn = oneOf<HideableColumn>(["backlog"]);
const isEditorMode = oneOf<EditorMode>(["formatted", "markdown"]);
const isState = oneOf<SavedState>([
  "",
  "all",
  "blocked",
  "unblocked",
  "repair",
  "working",
  "rejected",
]);

const isStrings = (value: unknown): value is string[] =>
  Array.isArray(value) && value.every((item) => typeof item === "string");

function isChoice(value: unknown): value is FilterChoice {
  return (
    isRecord(value) && isStrings(value.include) && isStrings(value.exclude)
  );
}

function isScope(value: unknown): value is ScopePreferences {
  return (
    isRecord(value) &&
    isView(value.view) &&
    isChoice(value.type) &&
    isChoice(value.priority) &&
    isChoice(value.workstream) &&
    isChoice(value.age) &&
    isState(value.state)
  );
}

export function isPreferences(value: unknown): value is Preferences {
  return (
    isRecord(value) &&
    isTheme(value.theme) &&
    isDensity(value.density) &&
    isColourBy(value.colourBy) &&
    Array.isArray(value.hiddenColumns) &&
    value.hiddenColumns.every(isHideableColumn) &&
    isEditorMode(value.editor) &&
    isRecord(value.scopes) &&
    Object.values(value.scopes).every(isScope) &&
    isRecord(value.expandedWorkstreams) &&
    Object.values(value.expandedWorkstreams).every(isStrings)
  );
}

export function fetchPreferences(
  fetcher: AuthenticatedFetch,
  signal?: AbortSignal,
) {
  return getJSON(
    fetcher,
    "/api/preferences",
    isPreferences,
    "Preferences response was invalid",
    signal,
  );
}

// PreferencesPatch is a save of what changed (FH-173): the top-level
// preferences changed, and the scopes and expanded workstreams of each
// project changed, null for a project's entry removed. What it leaves out
// stays as saved, so a copy older than the file (an agent renamed a
// workstream since it loaded) does not put back what it did not change.
export type PreferencesPatch = Partial<
  Omit<Preferences, "scopes" | "expandedWorkstreams">
> & {
  scopes?: Record<string, ScopePreferences | null>;
  expandedWorkstreams?: Record<string, string[] | null>;
};

const same = (a: unknown, b: unknown) =>
  JSON.stringify(a) === JSON.stringify(b);

// changedEntries is each project of next whose entry differs from saved,
// and null for each project next no longer has; undefined when none.
function changedEntries<T>(
  saved: Record<string, T>,
  next: Record<string, T>,
): Record<string, T | null> | undefined {
  const out: Record<string, T | null> = {};
  for (const [project, value] of Object.entries(next))
    if (!same(saved[project], value)) out[project] = value;
  for (const project of Object.keys(saved))
    if (!(project in next)) out[project] = null;
  return Object.keys(out).length > 0 ? out : undefined;
}

// preferencesPatch is what changed from saved, the server's copy, to next:
// all of next when the server's copy is unknown, and undefined when
// nothing changed.
export function preferencesPatch(
  saved: Preferences | undefined,
  next: Preferences,
): PreferencesPatch | undefined {
  if (!saved) return next;
  const patch: PreferencesPatch = {};
  const { scopes, expandedWorkstreams, ...rest } = next;
  for (const key of Object.keys(rest) as (keyof typeof rest)[])
    if (!same(saved[key], rest[key]))
      Object.assign(patch, { [key]: rest[key] });
  const changedScopes = changedEntries(saved.scopes, scopes);
  if (changedScopes) patch.scopes = changedScopes;
  const changedExpanded = changedEntries(
    saved.expandedWorkstreams,
    expandedWorkstreams,
  );
  if (changedExpanded) patch.expandedWorkstreams = changedExpanded;
  return Object.keys(patch).length > 0 ? patch : undefined;
}

// savePreferences saves a patch of the preferences. keepalive sends a save
// the page is unloading on (FH-48).
export async function savePreferences(
  fetcher: AuthenticatedFetch,
  patch: PreferencesPatch,
  options: { keepalive?: boolean } = {},
) {
  await sendJSON(fetcher, "PATCH", "/api/preferences", patch, options);
}
