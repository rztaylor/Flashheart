import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import {
  applyFilters,
  chipWorkstreams,
  choiceCount,
  choiceState,
  emptyFilters,
  filterOptions,
  fittingCount,
  isFiltered,
  noChoice,
  toggleChoice,
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
    needsYou: false,
    agentWorking: false,
    openQuestions: 0,
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
const only = (...include: string[]) => ({ include, exclude: [] });
const not = (...exclude: string[]) => ({ include: [], exclude });

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
    expect(
      applyFilters(cards, { ...emptyFilters, type: only("bug") }),
    ).toHaveLength(1);
    expect(
      ids(applyFilters(cards, { ...emptyFilters, priority: only("low") })),
    ).toEqual(["AL-6"]);
    expect(
      ids(
        applyFilters(cards, { ...emptyFilters, workstream: only("board-ui") }),
      ),
    ).toEqual(["AL-3"]);
    expect(
      applyFilters(cards, { ...emptyFilters, workstream: only("(none)") }),
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

  it("shows only tickets that need you, with any other filter (FH-44)", () => {
    const waiting = [
      card({ id: "AL-1", needsYou: true, blocked: true }),
      card({ id: "AL-2", needsYou: true }),
      card({ id: "AL-3" }),
    ];
    const only = { ...emptyFilters, needsYou: true };
    expect(ids(applyFilters(waiting, only))).toEqual(["AL-1", "AL-2"]);
    expect(ids(applyFilters(waiting, { ...only, state: "blocked" }))).toEqual([
      "AL-1",
    ]);
    expect(isFiltered(only)).toBe(true);
  });

  it("shows only tickets with an agent at work (FH-42)", () => {
    const working = [
      card({ id: "AL-1", agentWorking: true }),
      card({ id: "AL-2" }),
    ];
    expect(
      ids(applyFilters(working, { ...emptyFilters, state: "working" })),
    ).toEqual(["AL-1"]);
  });

  it("keeps a card matching any included value of a dimension", () => {
    expect(
      ids(applyFilters(cards, { ...emptyFilters, type: only("bug", "spike") })),
    ).toEqual(["AL-5", "AL-6"]);
  });

  it("drops cards with an excluded value", () => {
    expect(
      ids(applyFilters(cards, { ...emptyFilters, type: not("bug", "docs") })),
    ).toEqual(["AL-3", "AL-6"]);
    expect(
      ids(
        applyFilters(cards, { ...emptyFilters, workstream: not("board-ui") }),
      ),
    ).toEqual(["AL-5", "AL-6", "AL-7"]);
    expect(isFiltered({ ...emptyFilters, priority: not("low") })).toBe(true);
  });

  it("combines dimensions with and", () => {
    expect(
      ids(
        applyFilters(cards, {
          ...emptyFilters,
          type: only("bug", "spike"),
          priority: not("low"),
        }),
      ),
    ).toEqual(["AL-5"]);
  });

  it("filters by the age of the last change", () => {
    const now = new Date("2026-10-07T12:00:00Z");
    const aged = [
      card({ id: "AL-1", modified: "2026-10-07T09:00:00Z" }),
      card({ id: "AL-2", modified: "2026-10-03T09:00:00Z" }),
      card({ id: "AL-3", modified: "2026-09-01T09:00:00Z" }),
    ];
    expect(
      ids(applyFilters(aged, { ...emptyFilters, age: only("today") }, now)),
    ).toEqual(["AL-1"]);
    expect(
      ids(applyFilters(aged, { ...emptyFilters, age: not("older") }, now)),
    ).toEqual(["AL-1", "AL-2"]);
  });
});

describe("toggleChoice", () => {
  it("includes on a click and restores on the next", () => {
    const once = toggleChoice(noChoice, "bug", false);
    expect(once).toEqual(only("bug"));
    expect(toggleChoice(once, "spike", false)).toEqual(only("bug", "spike"));
    expect(toggleChoice(once, "bug", false)).toEqual(noChoice);
  });

  it("excludes on a modified click and restores on any next click", () => {
    const once = toggleChoice(noChoice, "bug", true);
    expect(once).toEqual(not("bug"));
    expect(toggleChoice(once, "bug", false)).toEqual(noChoice);
    expect(toggleChoice(toggleChoice(once, "bug", true), "x", true)).toEqual(
      not("x"),
    );
  });

  it("names a value's state", () => {
    expect(choiceState(only("bug"), "bug")).toBe("included");
    expect(choiceState(not("bug"), "bug")).toBe("excluded");
    expect(choiceState(only("bug"), "spike")).toBe("idle");
    expect(choiceCount({ include: ["a"], exclude: ["b", "c"] })).toBe(3);
  });
});

describe("chipWorkstreams", () => {
  const brief = (slug: string, done: number, total: number) => ({
    slug,
    title: slug,
    created: "",
    status: "active" as const,
    done,
    total,
  });
  it("orders by open tickets, most first, and leaves out finished lines", () => {
    const lines = [
      brief("quiet", 0, 2),
      brief("busy", 1, 4),
      brief("finished", 3, 3),
      brief("empty", 0, 0),
    ];
    const open = [
      card({ workstream: "busy", column: "in-progress" }),
      card({ workstream: "busy", column: "review" }),
      card({ workstream: "busy", column: "backlog" }),
      card({ workstream: "quiet", column: "backlog" }),
      card({ workstream: "quiet", column: "up-next" }),
    ];
    expect(chipWorkstreams(lines, open).map((line) => line.slug)).toEqual([
      "busy",
      "quiet",
      "empty",
    ]);
  });

  it("keeps a finished line while it is filtered", () => {
    expect(
      chipWorkstreams([brief("finished", 3, 3)], [], ["finished"]).map(
        (line) => line.slug,
      ),
    ).toEqual(["finished"]);
  });
});

describe("fittingCount", () => {
  it("counts the chips that fit on one line", () => {
    expect(fittingCount([100, 100, 100], 320, 8)).toBe(3);
    expect(fittingCount([100, 100, 100], 300, 8)).toBe(2);
    expect(fittingCount([100, 100, 100], 99, 8)).toBe(0);
    expect(fittingCount([], 300, 8)).toBe(0);
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
