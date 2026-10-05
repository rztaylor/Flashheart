import type { SavedView, ScopePreferences } from "../api/preferences";
import { emptyFilters, type Filters } from "./filters";

// Remembered view and filters per project or All projects (VIEW-7, CFG-2).
// The search box is never remembered.

export function filtersFor(
  saved: ScopePreferences | undefined,
  query = "",
): Filters {
  if (!saved) return { ...emptyFilters, query };
  return {
    query,
    type: saved.type,
    priority: saved.priority,
    workstream: saved.workstream,
    state: saved.state || "all",
    hideLater: saved.hideLater,
  };
}

export function rememberScope(
  filters: Filters,
  view: SavedView,
): ScopePreferences {
  return {
    view,
    type: filters.type,
    priority: filters.priority,
    workstream: filters.workstream,
    state: filters.state === "all" ? "" : filters.state,
    hideLater: filters.hideLater,
  };
}

export function sameScope(
  a: ScopePreferences | undefined,
  b: ScopePreferences,
) {
  return (
    a !== undefined &&
    a.view === b.view &&
    a.type === b.type &&
    a.priority === b.priority &&
    a.workstream === b.workstream &&
    a.state === b.state &&
    a.hideLater === b.hideLater
  );
}
