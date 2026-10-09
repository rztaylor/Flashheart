import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import {
  catalogue,
  completesWorkstream,
  DEFERRED,
  PLACEMENTS,
  type Placement,
  pickRemark,
  progressDue,
  remarksFor,
  winningAside,
} from "./remarks";

// The reviewed collection keeps these original numbers; the gaps are
// entries the user rejected.
const reviewed = [
  1, 2, 3, 4, 5, 6, 7, 11, 13, 14, 17, 18, 19, 21, 23, 24, 25, 27, 28, 30, 31,
  32, 33, 34, 35, 36, 39, 40, 41, 43, 44, 46, 47, 48, 49, 50, 51, 52, 53, 54,
  55, 57, 58, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 75, 77,
  79, 80, 82, 83, 85, 86, 87, 88, 92, 93, 95, 96, 99, 100,
];

describe("the remarks catalogue", () => {
  it("keeps the whole reviewed collection with its original ids", () => {
    expect(catalogue.map((remark) => remark.id)).toEqual(reviewed);
    for (const remark of catalogue) {
      expect(remark.text.trim()).not.toBe("");
      expect(remark.category).not.toBe("");
    }
  });

  it("labels only the four direct quotes, with their source", () => {
    const quotes = catalogue.filter((remark) => remark.quote);
    expect(quotes.map((remark) => remark.id)).toEqual([1, 2, 3, 4]);
    for (const remark of quotes) expect(remark.quote).toMatch(/\w/);
    // Quote text carries no source note or markup.
    for (const remark of catalogue) expect(remark.text).not.toMatch(/\*\*|—/);
  });

  it("places every entry once or defers it, and invents no entries", () => {
    const placed = Object.values(PLACEMENTS).flatMap(
      (placement) => placement.ids,
    );
    expect(new Set(placed).size).toBe(placed.length);
    expect([...placed, ...DEFERRED].sort((a, b) => a - b)).toEqual(reviewed);
  });

  it("draws each placement only from its own lines", () => {
    expect(remarksFor("completion").map((remark) => remark.id)).toEqual([
      61, 62, 63, 64, 65, 66, 67, 68, 69, 70,
    ]);
    expect(remarksFor("shutdown").every((remark) => remark.id >= 92)).toBe(
      true,
    );
    expect(
      remarksFor("search").every((remark) => remark.id >= 82 && remark.id < 90),
    ).toBe(true);
  });
});

describe("choosing a remark", () => {
  const pool = remarksFor("search");

  it("never repeats the one shown last", () => {
    for (let i = 0; i < 50; i++) {
      const random = () => i / 50;
      const last = pool[Math.floor(random() * pool.length)];
      expect(pickRemark(pool, last?.id, random).id).not.toBe(last?.id);
    }
  });

  it("is decided by its random source", () => {
    expect(pickRemark(pool, undefined, () => 0).id).toBe(pool[0]?.id);
    expect(pickRemark(pool, undefined, () => 0.999).id).toBe(pool.at(-1)?.id);
  });
});

describe("one aside per screen", () => {
  it("shows only the highest priority, ties to the first registered", () => {
    expect(winningAside([])).toBeUndefined();
    expect(
      winningAside([
        { key: "a", priority: 1 },
        { key: "b", priority: 3 },
        { key: "c", priority: 3 },
      ]),
    ).toBe("b");
  });
});

const card = (id: string, patch: Partial<Card>) =>
  ({
    id,
    project: "fh",
    column: "backlog",
    workstream: "",
    ...patch,
  }) as Card;

describe("completion", () => {
  const line = [
    card("FH-1", { workstream: "sync", column: "done" }),
    card("FH-2", { workstream: "sync", column: "review" }),
    card("FH-3", { workstream: "other", column: "backlog" }),
  ];

  it("is earned by the move that finishes a workstream's last ticket", () => {
    const moving = line[1] as Card;
    expect(completesWorkstream(line, moving, "done")).toBe(true);
    expect(completesWorkstream(line, moving, "review")).toBe(false);
  });

  it("is not earned while other tickets remain, or without a workstream", () => {
    const open = [...line, card("FH-4", { workstream: "sync" })];
    expect(completesWorkstream(open, line[1] as Card, "done")).toBe(false);
    expect(
      completesWorkstream(line, card("FH-9", { column: "review" }), "done"),
    ).toBe(false);
    // Moving a ticket that is already done completes nothing.
    expect(completesWorkstream(line, line[0] as Card, "done")).toBe(false);
  });
});

describe("progress", () => {
  it("is remarked on at most once every ten minutes", () => {
    const start = Date.UTC(2026, 9, 6, 10, 0);
    expect(progressDue(undefined, start)).toBe(true);
    expect(progressDue(start, start + 9 * 60_000)).toBe(false);
    expect(progressDue(start, start + 10 * 60_000)).toBe(true);
  });
});

// Every placement named in the type is defined.
const names: Placement[] = [
  "brand",
  "board-empty",
  "search",
  "review-empty",
  "agents-none",
  "needs-you-clear",
  "workstreams-none",
  "plan",
  "handoff",
  "handoff-missing",
  "progress",
  "completion",
  "shutdown",
];
it("defines every placement", () => {
  expect(Object.keys(PLACEMENTS).sort()).toEqual([...names].sort());
});
