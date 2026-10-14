import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  defaultPreferences,
  type Preferences,
  type ScopePreferences,
} from "../api/preferences";
import type { Route } from "./route";
import { useScopeMemory } from "./useScopeMemory";

const none = { include: [], exclude: [] };
const hiding = (slug: string): ScopePreferences => ({
  view: "board",
  type: none,
  priority: none,
  workstream: { include: [], exclude: [slug] },
  age: none,
  state: "",
});
const withScopes = (scopes: Record<string, ScopePreferences>): Preferences => ({
  ...defaultPreferences,
  scopes,
});

const route: Route = {
  scope: { kind: "project", project: "alpha" },
  view: "board",
};

// FH-173: a copy of the preferences refreshed from the server (an agent
// renamed a workstream the filter hides) restores the open scope's filters,
// so they hide the renamed workstream and the next filter change saves the
// new slug; the page's own changes do not reset them.
describe("useScopeMemory", () => {
  it("restores the open scope's filters when a refresh changed them", () => {
    const updatePreferences = vi.fn();
    const { result, rerender } = renderHook(
      (props: { preferences: Preferences; refreshed: number }) =>
        useScopeMemory({
          route,
          preferencesLoaded: true,
          updatePreferences,
          ...props,
        }),
      {
        initialProps: {
          preferences: withScopes({ alpha: hiding("board-ui") }),
          refreshed: 0,
        },
      },
    );
    expect(result.current.filters.workstream.exclude).toEqual(["board-ui"]);
    act(() =>
      result.current.setFilters({
        ...result.current.filters,
        query: "header",
      }),
    );

    // A refresh that changed another project's scope leaves these alone.
    rerender({
      preferences: withScopes({
        alpha: hiding("board-ui"),
        beta: hiding("offline"),
      }),
      refreshed: 1,
    });
    expect(result.current.filters.query).toBe("header");
    expect(result.current.filters.workstream.exclude).toEqual(["board-ui"]);

    // One that changed this scope's restores them, keeping the search.
    rerender({
      preferences: withScopes({ alpha: hiding("board"), beta: hiding("x") }),
      refreshed: 2,
    });
    expect(result.current.filters.workstream.exclude).toEqual(["board"]);
    expect(result.current.filters.query).toBe("header");
  });

  it("does not restore on the page's own changes", () => {
    const { result, rerender } = renderHook(
      (props: { preferences: Preferences }) =>
        useScopeMemory({
          route,
          preferencesLoaded: true,
          updatePreferences: vi.fn(),
          refreshed: 0,
          ...props,
        }),
      { initialProps: { preferences: withScopes({}) } },
    );
    act(() =>
      result.current.setFilters({
        ...result.current.filters,
        state: "blocked",
      }),
    );
    rerender({ preferences: withScopes({ alpha: hiding("lines") }) });
    expect(result.current.filters.state).toBe("blocked");
    expect(result.current.filters.workstream.exclude).toEqual([]);
  });
});
