import { useEffect, useState } from "react";

import type { Preferences } from "../api/preferences";
import { emptyFilters, type Filters } from "../model/filters";
import type { Scope } from "../model/scopes";
import type { Route, View } from "./route";
import {
  rememberIn,
  restoredFilters,
  savedView,
  scopeName,
  viewFor,
} from "./scopeMemory";

// useScopeMemory holds the filters and remembers them, with the view, per
// project and for All projects (VIEW-7, CFG-2): a scope's own filters come
// back when it is chosen, once the preferences have loaded.
export function useScopeMemory({
  route,
  preferences,
  preferencesLoaded,
  updatePreferences,
}: {
  route: Route;
  preferences: Preferences;
  preferencesLoaded: boolean;
  updatePreferences(change: (current: Preferences) => Preferences): void;
}) {
  const [filters, setFiltersState] = useState<Filters>(emptyFilters);
  const name = scopeName(route.scope);
  // biome-ignore lint/correctness/useExhaustiveDependencies: restore only when the scope changes or preferences first load.
  useEffect(() => {
    if (preferencesLoaded)
      setFiltersState((current) => restoredFilters(preferences, name, current));
  }, [name, preferencesLoaded]);

  const remember = (next: Filters, view: View) => {
    const saved = savedView(view);
    if (saved !== undefined)
      updatePreferences((current) => rememberIn(current, name, next, saved));
  };
  return {
    filters,
    // setFilters changes the filters and remembers them for this scope.
    setFilters(next: Filters) {
      setFiltersState(next);
      remember(next, route.view);
    },
    // carryFilters changes the filters without remembering them, before a
    // move to a scope that restores its own.
    carryFilters: setFiltersState,
    // rememberView remembers the filters with a newly chosen view.
    rememberView(view: View) {
      remember(filters, view);
    },
    // viewFor is the view a chosen scope opens on.
    viewFor: (scope: Scope) => viewFor(preferences, scope, route.view),
  };
}
