import { describe, expect, it } from "vitest";

import type { Card } from "../api/board";
import {
  afterForIndex,
  afterForStep,
  canReorder,
  displayOrder,
  placeCard,
  predecessor,
} from "./order";

const card = (id: string, patch: Partial<Card> = {}) =>
  ({
    id,
    project: "fh",
    column: "backlog",
    priority: "medium",
    created: "2026-10-01",
    title: id,
    ...patch,
  }) as Card;

const ids = (cards: Card[]) => cards.map((item) => item.id);

describe("manual order", () => {
  const a = card("FH-1");
  const b = card("FH-2");
  const c = card("FH-3");
  const other = card("NG-1", { project: "ng" });

  it("follows the nearest card of the same project above the drop", () => {
    const shown = [a, b, other, c];
    expect(afterForIndex(shown, c, 0)).toBe("");
    expect(afterForIndex(shown, c, 1)).toBe("FH-1");
    expect(afterForIndex(shown, c, 2)).toBe("FH-2");
    // Below another project's card, it still follows its own project's card.
    expect(afterForIndex(shown, c, 3)).toBe("FH-2");
    expect(afterForIndex(shown, a, 3)).toBe("FH-3");
    expect(afterForIndex([other], a, 1)).toBe("");
  });

  it("steps up, down, to the top and to the bottom among visible cards", () => {
    const shown = [a, other, b, c];
    expect(afterForStep(shown, b, "up")).toBe("");
    expect(afterForStep(shown, c, "up")).toBe("FH-1");
    expect(afterForStep(shown, a, "down")).toBe("FH-2");
    expect(afterForStep(shown, a, "bottom")).toBe("FH-3");
    expect(afterForStep(shown, c, "top")).toBe("");
    // Already there: nothing to do.
    expect(afterForStep(shown, a, "up")).toBeNull();
    expect(afterForStep(shown, a, "top")).toBeNull();
    expect(afterForStep(shown, c, "down")).toBeNull();
    expect(afterForStep(shown, c, "bottom")).toBeNull();
  });

  it("names the card before, for Undo", () => {
    const all = [a, card("FH-9", { column: "up-next" }), other, b];
    expect(predecessor(all, a)).toBe("");
    expect(predecessor(all, b)).toBe("FH-1");
    expect(predecessor(all, other)).toBe("");
  });

  it("places a card for display at once, without touching other columns", () => {
    const next = card("FH-5", { column: "up-next" });
    const all = [a, b, c, next];
    expect(ids(placeCard(all, "FH-3", "backlog", ""))).toEqual([
      "FH-3",
      "FH-1",
      "FH-2",
      "FH-5",
    ]);
    expect(ids(placeCard(all, "FH-1", "backlog", "FH-2"))).toEqual([
      "FH-2",
      "FH-1",
      "FH-3",
      "FH-5",
    ]);
    const moved = placeCard(all, "FH-5", "backlog", "FH-1");
    expect(ids(moved)).toEqual(["FH-1", "FH-5", "FH-2", "FH-3"]);
    expect(moved[1]?.column).toBe("backlog");
    // Without a position the card only changes column.
    expect(placeCard(all, "FH-1", "done", undefined)[0]?.column).toBe("done");
    expect(all[0]?.column).toBe("backlog");
  });
});

describe("display sorts", () => {
  const cards = [
    card("FH-1", { priority: "low", title: "b" }),
    card("FH-2", { priority: "high", title: "c" }),
    card("FH-3", { priority: "medium", title: "a" }),
  ];

  it("show the manual order when none is chosen", () => {
    expect(displayOrder(cards, null)).toBe(cards);
    expect(canReorder(null)).toBe(true);
  });

  it("rearrange a copy and never the manual order", () => {
    const before = ids(cards);
    expect(
      ids(displayOrder(cards, { by: "priority", descending: false })),
    ).toEqual(["FH-2", "FH-3", "FH-1"]);
    expect(ids(displayOrder(cards, { by: "title", descending: true }))).toEqual(
      ["FH-2", "FH-1", "FH-3"],
    );
    expect(ids(cards)).toEqual(before);
    expect(canReorder({ by: "title", descending: false })).toBe(false);
  });
});
