import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import {
  applyFilters,
  emptyFilters,
  filterOptions,
  isFiltered,
} from "./filters";

function card(overrides: Partial<Card>): Card {
  return {
    project: "alpha",
    slug: "feat--x",
    column: "todo",
    title: "X",
    type: "feature",
    priority: "medium",
    workstream: "",
    tags: [],
    created: "2026-10-01",
    updated: "",
    modified: "",
    branch: "",
    dependsOn: [],
    criteria: { done: 0, total: 0 },
    excerpt: "",
    handoffNext: "",
    attachments: 0,
    hasReview: false,
    blocked: false,
    blockedBy: [],
    needsRepair: [],
    warnings: [],
    ...overrides,
  };
}

const cards = [
  card({
    slug: "feat--card-panel",
    title: "Card panel",
    workstream: "board-ui",
    tags: ["ui"],
  }),
  card({
    slug: "bug--overflow",
    title: "Column overflow",
    type: "bug",
    priority: "high",
    blocked: true,
    searchText: "A 200-character title widens the column",
  }),
  card({
    slug: "spike--offline",
    title: "Offline mode",
    type: "spike",
    priority: "low",
    tags: ["later-possibility"],
  }),
  card({
    slug: "docs--broken",
    title: "Broken",
    type: "docs",
    priority: "",
    needsRepair: ["frontmatter does not parse"],
  }),
];

describe("applyFilters", () => {
  it("returns everything with empty filters", () => {
    expect(applyFilters(cards, emptyFilters)).toHaveLength(4);
    expect(isFiltered(emptyFilters)).toBe(false);
  });

  it("searches title, slug, tags and body text, case-insensitively", () => {
    const slugs = (query: string) =>
      applyFilters(cards, { ...emptyFilters, query }).map((c) => c.slug);
    expect(slugs("CARD")).toEqual(["feat--card-panel"]);
    expect(slugs("bug--")).toEqual(["bug--overflow"]);
    expect(slugs("widens")).toEqual(["bug--overflow"]);
    expect(slugs("ui")).toContain("feat--card-panel");
    expect(slugs("panel offline")).toEqual([]);
  });

  it("filters by type, priority, workstream and state", () => {
    expect(applyFilters(cards, { ...emptyFilters, type: "bug" })).toHaveLength(
      1,
    );
    expect(
      applyFilters(cards, { ...emptyFilters, priority: "low" })[0]?.slug,
    ).toBe("spike--offline");
    expect(
      applyFilters(cards, { ...emptyFilters, workstream: "board-ui" })[0]?.slug,
    ).toBe("feat--card-panel");
    expect(
      applyFilters(cards, { ...emptyFilters, workstream: "(none)" }),
    ).toHaveLength(3);
    expect(
      applyFilters(cards, { ...emptyFilters, state: "blocked" }).map(
        (c) => c.slug,
      ),
    ).toEqual(["bug--overflow"]);
    expect(
      applyFilters(cards, { ...emptyFilters, state: "unblocked" }),
    ).toHaveLength(3);
    expect(
      applyFilters(cards, { ...emptyFilters, state: "repair" }).map(
        (c) => c.slug,
      ),
    ).toEqual(["docs--broken"]);
  });

  it("hides later possibilities on request", () => {
    const visible = applyFilters(cards, { ...emptyFilters, hideLater: true });
    expect(visible.map((c) => c.slug)).not.toContain("spike--offline");
    expect(isFiltered({ ...emptyFilters, hideLater: true })).toBe(true);
  });
});

describe("filterOptions", () => {
  it("lists the values present, sorted", () => {
    const options = filterOptions(cards);
    expect(options.types).toEqual(["bug", "docs", "feature", "spike"]);
    expect(options.priorities).toEqual(["high", "medium", "low"]);
    expect(options.workstreams).toEqual(["board-ui"]);
  });
});
