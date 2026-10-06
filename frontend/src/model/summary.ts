import type { RunCounts } from "../api/runs";

// viewSummary is the page header's one quiet line for a view
// (ui-layout.md §1): tickets on the Board and Table, live runs on Agents,
// workstreams and stations served on Workstreams. Empty while loading.
export function viewSummary({
  view,
  board,
  filtered,
  runs,
  workstreams,
}: {
  view: "board" | "agents" | "workstreams" | "table";
  board?: { shown: number; total: number };
  filtered: boolean;
  runs?: RunCounts;
  workstreams: { done: number; total: number }[];
}): string {
  switch (view) {
    case "agents": {
      if (!runs) return "";
      const live = `${runs.live} live ${runs.live === 1 ? "run" : "runs"}`;
      return runs.needsYou > 0
        ? `${live} · ${runs.needsYou} need${runs.needsYou === 1 ? "s" : ""} you`
        : live;
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
