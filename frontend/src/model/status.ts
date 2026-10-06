import { COLUMNS } from "../api/board";

// StateTone is a status pill's colour role (ui-layout.md §7). Backlog and
// Up next are neutral; only work in motion, under review or finished takes
// a hue, and the pill always says the column's word.
export type StateTone = "neutral" | "progress" | "review" | "done" | "blocked";

const tones: Record<string, StateTone> = {
  "in-progress": "progress",
  review: "review",
  done: "done",
};

// statusOf names a ticket's column, or an archived or missing station.
export function statusOf(column: string): { tone: StateTone; label: string } {
  if (column === "archived") return { tone: "neutral", label: "Archived" };
  const known = COLUMNS.find((item) => item.id === column);
  if (!known) return { tone: "neutral", label: "Missing" };
  return { tone: tones[column] ?? "neutral", label: known.title };
}
