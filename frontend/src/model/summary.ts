import type { Column } from "../api/board";

// viewSummary is the page header's one quiet line for a view
// (ui-layout.md §1): tickets on the Board and Table, what is in progress
// and ready to review on the Overview, workstreams and stations served on
// Workstreams, archived tickets on the archive. Empty while loading.
export function viewSummary({
  view,
  board,
  filtered,
  counts,
  workstreams,
  archived,
}: {
  view: "board" | "overview" | "workstreams" | "table" | "archive" | "ticket";
  board?: { shown: number; total: number };
  filtered: boolean;
  // counts are the tickets per column of each project in scope.
  counts?: Record<Column, number>[];
  workstreams: { done: number; total: number }[];
  archived?: { count: number; of: "tickets" | "projects" };
}): string {
  switch (view) {
    // A ticket's full page has its own header.
    case "ticket":
      return "";
    case "archive": {
      if (!archived) return "";
      const one = archived.of === "tickets" ? "ticket" : "project";
      return `${archived.count} archived ${archived.count === 1 ? one : archived.of}`;
    }
    case "overview": {
      if (!counts) return "";
      const sum = (column: Column) =>
        counts.reduce((total, project) => total + project[column], 0);
      return `${sum("in-progress")} in progress · ${sum("review")} ready to review`;
    }
    case "workstreams": {
      const done = workstreams.reduce((sum, item) => sum + item.done, 0);
      const total = workstreams.reduce((sum, item) => sum + item.total, 0);
      return `${workstreams.length} ${workstreams.length === 1 ? "workstream" : "workstreams"} · ${done} of ${total} stations served`;
    }
    default:
      if (!board) return "";
      return filtered
        ? `${board.shown} of ${board.total} tickets`
        : `${board.total} ${board.total === 1 ? "ticket" : "tickets"}`;
  }
}
