// UI preferences saved through the backend in config.yaml (CFG-2), never in
// browser storage.
import { type AuthenticatedFetch, getJSON, isRecord, sendJSON } from "./client";
import type { ThemePreference } from "./info";

export type Density = "compact" | "normal" | "detailed";
export type ColourBy = "type" | "priority" | "age" | "none";
export type SavedView = "" | "board" | "agents" | "workstreams" | "table";
export type VirtualColumn = "needs-you" | "agent-working";
export type SavedState = "" | "all" | "blocked" | "unblocked" | "repair";

// ScopePreferences is the remembered view and filters of one project, or of
// All projects under the key "all".
export interface ScopePreferences {
  view: SavedView;
  type: string;
  priority: string;
  workstream: string;
  state: SavedState;
  hideLater: boolean;
}

export interface Preferences {
  theme: ThemePreference;
  density: Density;
  colourBy: ColourBy;
  // virtualColumns lists the virtual columns shown on the board (VIEW-2).
  virtualColumns: VirtualColumn[];
  scopes: Record<string, ScopePreferences>;
}

export const defaultPreferences: Preferences = {
  theme: "system",
  density: "normal",
  colourBy: "type",
  virtualColumns: ["needs-you"],
  scopes: {},
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
  "agents",
  "workstreams",
  "table",
]);
const isVirtualColumn = oneOf<VirtualColumn>(["needs-you", "agent-working"]);
const isState = oneOf<SavedState>([
  "",
  "all",
  "blocked",
  "unblocked",
  "repair",
]);

function isScope(value: unknown): value is ScopePreferences {
  return (
    isRecord(value) &&
    isView(value.view) &&
    typeof value.type === "string" &&
    typeof value.priority === "string" &&
    typeof value.workstream === "string" &&
    isState(value.state) &&
    typeof value.hideLater === "boolean"
  );
}

export function isPreferences(value: unknown): value is Preferences {
  return (
    isRecord(value) &&
    isTheme(value.theme) &&
    isDensity(value.density) &&
    isColourBy(value.colourBy) &&
    Array.isArray(value.virtualColumns) &&
    value.virtualColumns.every(isVirtualColumn) &&
    isRecord(value.scopes) &&
    Object.values(value.scopes).every(isScope)
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

export async function savePreferences(
  fetcher: AuthenticatedFetch,
  preferences: Preferences,
) {
  await sendJSON(fetcher, "PUT", "/api/preferences", preferences);
}
