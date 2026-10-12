// Keyboard movement across board columns (NFR-3): arrow keys move between
// cards; empty columns are skipped. neighbourOf is where focus goes when the
// focused item leaves its list (FH-84).

export interface GridPosition {
  column: number;
  row: number;
}

export type GridMove = "up" | "down" | "left" | "right" | "home" | "end";

export function moveInGrid(
  sizes: number[],
  from: GridPosition,
  move: GridMove,
): GridPosition {
  const size = sizes[from.column] ?? 0;
  switch (move) {
    case "up":
      return { column: from.column, row: Math.max(0, from.row - 1) };
    case "down":
      return { column: from.column, row: Math.min(size - 1, from.row + 1) };
    case "home":
      return { column: from.column, row: 0 };
    case "end":
      return { column: from.column, row: Math.max(0, size - 1) };
    case "left":
    case "right": {
      const step = move === "left" ? -1 : 1;
      for (
        let column = from.column + step;
        column >= 0 && column < sizes.length;
        column += step
      ) {
        const count = sizes[column] ?? 0;
        if (count > 0) return { column, row: Math.min(from.row, count - 1) };
      }
      return from;
    }
  }
}

// neighbourOf is the item that takes focus when key leaves its list (FH-84,
// ui-layout.md §8): the first item after it, in the list's order before the
// change, that remains, else the nearest one before it that remains; none
// when nothing remains or key was not listed.
export function neighbourOf(
  order: string[],
  key: string,
  remains: (key: string) => boolean,
): string | undefined {
  const at = order.indexOf(key);
  if (at < 0) return undefined;
  const after = order.slice(at + 1).find(remains);
  if (after !== undefined) return after;
  return order.slice(0, at).reverse().find(remains);
}
