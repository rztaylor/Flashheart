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
    id: "AL-99",
    slug: "x",
    column: "backlog",
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
    id: "AL-3",
    slug: "card-panel",
    title: "Card panel",
    workstream: "board-ui",
    tags: ["ui"],
  }),
  card({
    id: "AL-5",
    slug: "column-overflow",
    title: "Column overflow",
    type: "bug",
    priority: "high",
    blocked: true,
    searchText: "A 200-character title widens the column",
  }),
  card({
    id: "AL-6",
    slug: "offline-mode",
    title: "Offline mode",
    type: "spike",
    priority: "low",
    tags: ["later-possibility"],
  }),
  card({
    id: "AL-7",
    slug: "broken",
    title: "Broken",
    type: "docs",
    priority: "",
    needsRepair: ["frontmatter does not parse"],
  }),
];

const ids = (list: Card[]) => list.map((c) => c.id);

describe("applyFilters", () => {
  it("returns everything with empty filters", () => {
    expect(applyFilters(cards, emptyFilters)).toHaveLength(4);
    expect(isFiltered(emptyFilters)).toBe(false);
  });

  it("searches id, title, slug, tags and body text, case-insensitively", () => {
    const search = (query: string) =>
      ids(applyFilters(cards, { ...emptyFilters, query }));
    expect(search("al-5")).toEqual(["AL-5"]);
    expect(search("CARD")).toEqual(["AL-3"]);
    expect(search("offline-mode")).toEqual(["AL-6"]);
    expect(search("widens")).toEqual(["AL-5"]);
    expect(search("ui")).toContain("AL-3");
    expect(search("panel offline")).toEqual([]);
  });

  it("filters by type, priority, workstream and state", () => {
    expect(applyFilters(cards, { ...emptyFilters, type: "bug" })).toHaveLength(
      1,
    );
    expect(
      ids(applyFilters(cards, { ...emptyFilters, priority: "low" })),
    ).toEqual(["AL-6"]);
    expect(
      ids(applyFilters(cards, { ...emptyFilters, workstream: "board-ui" })),
    ).toEqual(["AL-3"]);
    expect(
      applyFilters(cards, { ...emptyFilters, workstream: "(none)" }),
    ).toHaveLength(3);
    expect(
      ids(applyFilters(cards, { ...emptyFilters, state: "blocked" })),
    ).toEqual(["AL-5"]);
    expect(
      applyFilters(cards, { ...emptyFilters, state: "unblocked" }),
    ).toHaveLength(3);
    expect(
      ids(applyFilters(cards, { ...emptyFilters, state: "repair" })),
    ).toEqual(["AL-7"]);
  });

  it("hides later possibilities on request", () => {
    expect(
      ids(applyFilters(cards, { ...emptyFilters, hideLater: true })),
    ).not.toContain("AL-6");
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
