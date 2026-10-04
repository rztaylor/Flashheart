import { describe, expect, it } from "vitest";

import { moveInGrid } from "./navigation";

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
