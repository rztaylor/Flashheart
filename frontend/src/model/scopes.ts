import type { SavedView, ScopePreferences } from "../api/preferences";
import {
  CHOICE_KEYS,
  type Choice,
  emptyFilters,
  type Filters,
  noChoice,
} from "./filters";

// Remembered view and filters per project or All projects (VIEW-7, CFG-2).
// The search box and the band's Needs you toggle (FH-44) are never
// remembered; they carry over from scope to scope.

export function filtersFor(
  saved: ScopePreferences | undefined,
  query = "",
  needsYou = false,
): Filters {
  if (!saved) return { ...emptyFilters, query, needsYou };
  return {
    query,
    needsYou,
    type: saved.type,
    priority: saved.priority,
    workstream: saved.workstream,
    age: saved.age,
    state: saved.state || "all",
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
    age: filters.age,
    state: filters.state === "all" ? "" : filters.state,
  };
}

const sameList = (a: string[], b: string[]) =>
  a.length === b.length && a.every((value, index) => value === b[index]);

const sameChoice = (a: Choice = noChoice, b: Choice = noChoice) =>
  sameList(a.include, b.include) && sameList(a.exclude, b.exclude);

export function sameScope(
  a: ScopePreferences | undefined,
  b: ScopePreferences,
) {
  return (
    a !== undefined &&
    a.view === b.view &&
    a.state === b.state &&
    CHOICE_KEYS.every((key) => sameChoice(a[key], b[key]))
  );
}
