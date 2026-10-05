import { describe, expect, it } from "vitest";

import { emptyFilters } from "./filters";
import { filtersFor, rememberScope, sameScope } from "./scopes";

describe("remembered scopes", () => {
  it("round-trips filters without the search text", () => {
    const filters = {
      ...emptyFilters,
      query: "handoff",
      type: "bug",
      state: "blocked" as const,
      hideLater: true,
    };
    const saved = rememberScope(filters, "table");
    expect(saved).toEqual({
      view: "table",
      type: "bug",
      priority: "",
      workstream: "",
      state: "blocked",
      hideLater: true,
    });
    expect(filtersFor(saved, "kept")).toEqual({ ...filters, query: "kept" });
    expect(filtersFor(undefined)).toEqual(emptyFilters);
    expect(sameScope(saved, rememberScope(filters, "table"))).toBe(true);
    expect(sameScope(saved, rememberScope(filters, "board"))).toBe(false);
  });
});
