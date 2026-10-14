import type { Preferences, SavedView } from "../api/preferences";
import type { Filters } from "../model/filters";
import {
  filtersFor,
  rememberScope,
  type Scope,
  sameScope,
} from "../model/scopes";
import type { View } from "./route";

// Scope memory: the view and filters remembered per project and for All
// projects (VIEW-7, CFG-2), saved with the other preferences. The search
// and the band's Needs you toggle carry over from scope to scope instead
// (model/scopes).

// scopeName is a scope's key in the remembered scopes.
export const scopeName = (scope: Scope): string =>
  scope.kind === "all" ? "all" : scope.project;

// savedView is the view to remember, or undefined for the archive, a
// ticket's full page and a workstream's page, which are side trips.
export function savedView(view: View): SavedView | undefined {
  return view === "archive" || view === "ticket" || view === "workstream"
    ? undefined
    : view;
}

// rememberIn records a scope's filters and view, returning the preferences
// unchanged when the scope already holds them, so nothing is saved.
export function rememberIn(
  preferences: Preferences,
  name: string,
  filters: Filters,
  view: SavedView,
): Preferences {
  const entry = rememberScope(filters, view);
  return sameScope(preferences.scopes[name], entry)
    ? preferences
    : { ...preferences, scopes: { ...preferences.scopes, [name]: entry } };
}

// restoredFilters are a scope's remembered filters with the current search
// and Needs you carried over.
export function restoredFilters(
  preferences: Preferences,
  name: string,
  current: Filters,
): Filters {
  return filtersFor(preferences.scopes[name], current.query, current.needsYou);
}

// viewFor is the view a chosen scope opens on: its remembered view, or else
// the current one.
export function viewFor(
  preferences: Preferences,
  scope: Scope,
  current: View,
): View {
  return preferences.scopes[scopeName(scope)]?.view || current;
}
