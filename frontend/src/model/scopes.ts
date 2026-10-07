import type { SavedView, ScopePreferences } from "../api/preferences";
import {
  CHOICE_KEYS,
  type Choice,
  emptyFilters,
  type Filters,
  noChoice,
} from "./filters";

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
