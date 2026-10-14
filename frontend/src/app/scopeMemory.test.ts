import { describe, expect, it } from "vitest";

import { defaultPreferences, type Preferences } from "../api/preferences";
import { emptyFilters } from "../model/filters";
import {
  rememberIn,
  renameWorkstreamIn,
  restoredFilters,
  savedView,
  scopeName,
  viewFor,
} from "./scopeMemory";

const bugs = {
  ...emptyFilters,
  type: { include: ["bug"], exclude: [] },
};

describe("scope memory (VIEW-7, CFG-2)", () => {
  it("names All projects and each project apart", () => {
    expect(scopeName({ kind: "all" })).toBe("all");
    expect(scopeName({ kind: "project", project: "flashheart" })).toBe(
      "flashheart",
    );
  });

  it("never remembers the archive or a ticket's full page as the view", () => {
    expect(savedView("board")).toBe("board");
    expect(savedView("overview")).toBe("overview");
    expect(savedView("workstreams")).toBe("workstreams");
    expect(savedView("table")).toBe("table");
    expect(savedView("archive")).toBe(undefined);
    expect(savedView("ticket")).toBe(undefined);
  });

  it("remembers a scope's filters and view beside the other scopes'", () => {
    const before: Preferences = {
      ...defaultPreferences,
      scopes: {
        all: {
          view: "table",
          type: { include: [], exclude: [] },
          priority: { include: [], exclude: [] },
          workstream: { include: [], exclude: [] },
          age: { include: [], exclude: [] },
          state: "",
        },
      },
    };
    const after = rememberIn(before, "flashheart", bugs, "board");
    expect(after.scopes.all).toBe(before.scopes.all);
    expect(after.scopes.flashheart).toMatchObject({
      view: "board",
      type: { include: ["bug"], exclude: [] },
    });
    expect(before.scopes.flashheart).toBe(undefined);
  });

  it("leaves the preferences as they are when nothing changed, so nothing is saved", () => {
    const saved = rememberIn(defaultPreferences, "flashheart", bugs, "board");
    expect(rememberIn(saved, "flashheart", bugs, "board")).toBe(saved);
    // The search text is never remembered, so typing changes nothing.
    expect(
      rememberIn(saved, "flashheart", { ...bugs, query: "handoff" }, "board"),
    ).toBe(saved);
    expect(rememberIn(saved, "flashheart", bugs, "table")).not.toBe(saved);
  });

  it("restores a scope's filters, carrying the search and Needs you over", () => {
    const saved = rememberIn(defaultPreferences, "flashheart", bugs, "board");
    const current = { ...emptyFilters, query: "lock", needsYou: true };
    expect(restoredFilters(saved, "flashheart", current)).toEqual({
      ...bugs,
      query: "lock",
      needsYou: true,
    });
    expect(
      restoredFilters(saved, "elsewhere", { ...bugs, query: "x" }),
    ).toEqual({ ...emptyFilters, query: "x" });
  });

  it("opens a chosen scope on its remembered view, else the current one", () => {
    const saved = rememberIn(defaultPreferences, "flashheart", bugs, "table");
    expect(
      viewFor(saved, { kind: "project", project: "flashheart" }, "board"),
    ).toBe("table");
    expect(viewFor(saved, { kind: "all" }, "overview")).toBe("overview");
    const entry = saved.scopes.flashheart;
    if (!entry) throw new Error("the scope was not remembered");
    const unset = {
      ...saved,
      scopes: { flashheart: { ...entry, view: "" as const } },
    };
    expect(
      viewFor(unset, { kind: "project", project: "flashheart" }, "workstreams"),
    ).toBe("workstreams");
  });
});

// A renamed workstream keeps its project's saved filter and expanded
// choice (FH-170), as the server rewrote them: the page's copy follows,
// so its next save does not put the old slug back.
describe("renaming a workstream in the preferences", () => {
  const preferences: Preferences = {
    ...defaultPreferences,
    scopes: {
      alpha: {
        view: "board",
        type: { include: [], exclude: [] },
        priority: { include: [], exclude: [] },
        workstream: { include: ["board-ui", "board"], exclude: ["x"] },
        age: { include: [], exclude: [] },
        state: "",
      },
      all: {
        view: "",
        type: { include: [], exclude: [] },
        priority: { include: [], exclude: [] },
        workstream: { include: [], exclude: ["board-ui"] },
        age: { include: [], exclude: [] },
        state: "",
      },
    },
    expandedWorkstreams: { alpha: ["a", "board-ui"], beta: ["board-ui"] },
  };

  it("rewrites the project's choices only, once", () => {
    const next = renameWorkstreamIn(preferences, "alpha", "board-ui", "board");
    expect(next.scopes.alpha?.workstream).toEqual({
      include: ["board"],
      exclude: ["x"],
    });
    expect(next.scopes.all?.workstream.exclude).toEqual(["board-ui"]);
    expect(next.expandedWorkstreams).toEqual({
      alpha: ["a", "board"],
      beta: ["board-ui"],
    });
    expect(preferences.expandedWorkstreams.alpha).toEqual(["a", "board-ui"]);
  });

  it("returns the preferences themselves when nothing names the slug", () => {
    expect(renameWorkstreamIn(preferences, "alpha", "offline", "away")).toBe(
      preferences,
    );
    expect(renameWorkstreamIn(preferences, "gamma", "board-ui", "b")).toBe(
      preferences,
    );
  });
});
