import { describe, expect, it } from "vitest";

import { moveInGrid, neighbourOf } from "./navigation";

// Three columns with 2, 0 and 3 cards.
const sizes = [2, 0, 3];

describe("moveInGrid", () => {
  it("moves within a column and stops at its ends", () => {
    expect(moveInGrid(sizes, { column: 0, row: 0 }, "down")).toEqual({
      column: 0,
      row: 1,
    });
    expect(moveInGrid(sizes, { column: 0, row: 1 }, "down")).toEqual({
      column: 0,
      row: 1,
    });
    expect(moveInGrid(sizes, { column: 0, row: 0 }, "up")).toEqual({
      column: 0,
      row: 0,
    });
  });

  it("skips empty columns and keeps the nearest row", () => {
    expect(moveInGrid(sizes, { column: 0, row: 1 }, "right")).toEqual({
      column: 2,
      row: 1,
    });
    expect(moveInGrid(sizes, { column: 2, row: 2 }, "left")).toEqual({
      column: 0,
      row: 1,
    });
    expect(moveInGrid(sizes, { column: 2, row: 0 }, "right")).toEqual({
      column: 2,
      row: 0,
    });
  });

  it("jumps to the ends of a column", () => {
    expect(moveInGrid(sizes, { column: 2, row: 1 }, "home")).toEqual({
      column: 2,
      row: 0,
    });
    expect(moveInGrid(sizes, { column: 2, row: 0 }, "end")).toEqual({
      column: 2,
      row: 2,
    });
  });
});

// The neighbour rule of ui-layout.md §8 (FH-84).
describe("neighbourOf", () => {
  const order = ["a", "b", "c", "d"];
  const only =
    (...keys: string[]) =>
    (key: string) =>
      keys.includes(key);

  it("is the next item that remains", () => {
    expect(neighbourOf(order, "b", only("a", "c", "d"))).toBe("c");
    expect(neighbourOf(order, "b", only("a", "d"))).toBe("d");
  });

  it("is the previous item that remains when nothing after it does", () => {
    expect(neighbourOf(order, "d", only("a", "b", "c"))).toBe("c");
    expect(neighbourOf(order, "c", only("a"))).toBe("a");
  });

  it("is none when nothing else remains or the item was not listed", () => {
    expect(neighbourOf(order, "b", only())).toBeUndefined();
    expect(neighbourOf(order, "x", only("a"))).toBeUndefined();
  });
});
