// Keyboard movement across board columns (NFR-3): arrow keys move between
// cards; empty columns are skipped.

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
