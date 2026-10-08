import { describe, expect, it } from "vitest";

import { emptyFilters } from "./filters";
import { filtersFor, rememberScope, sameScope } from "./scopes";

describe("remembered scopes", () => {
  it("round-trips filters without the search text", () => {
    const filters = {
      ...emptyFilters,
      query: "handoff",
      type: { include: ["bug", "spike"], exclude: [] },
      workstream: { include: [], exclude: ["board-ui"] },
      state: "blocked" as const,
    };
    const saved = rememberScope(filters, "table");
    expect(saved).toEqual({
      view: "table",
      type: { include: ["bug", "spike"], exclude: [] },
      priority: { include: [], exclude: [] },
      workstream: { include: [], exclude: ["board-ui"] },
      age: { include: [], exclude: [] },
      state: "blocked",
    });
    expect(filtersFor(saved, "kept")).toEqual({ ...filters, query: "kept" });
    expect(filtersFor(undefined)).toEqual(emptyFilters);
    expect(sameScope(saved, rememberScope(filters, "table"))).toBe(true);
    expect(sameScope(saved, rememberScope(filters, "board"))).toBe(false);
    expect(
      sameScope(
        saved,
        rememberScope(
          { ...filters, type: { include: ["bug"], exclude: [] } },
          "table",
        ),
      ),
    ).toBe(false);
  });
});
