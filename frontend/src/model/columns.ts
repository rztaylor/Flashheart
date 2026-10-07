// Virtual columns (VIEW-2): computed columns that mirror tickets from their
// real columns while an agent run needs the user or is working on them. They
// stand between In progress and Ready to review.
import type { Card } from "../api/board";
import type { VirtualColumn } from "../api/preferences";

export const VIRTUAL_COLUMNS: {
  id: VirtualColumn;
  title: string;
  empty: string;
  holds(card: Card): boolean;
}[] = [
  {
    id: "needs-you",
    title: "Needs you",
    empty: "Nothing needs you",
    holds: (card) => card.needsYou,
  },
  {
    id: "agent-working",
    title: "Agent working",
    empty: "No agent at work",
    holds: (card) => card.agentWorking,
  },
];

// shownVirtual lists the chosen virtual columns that hold tickets, so a
// board with nothing to flag stays calm.
export function shownVirtual(cards: Card[], chosen: VirtualColumn[]) {
  return VIRTUAL_COLUMNS.filter(
    (column) => chosen.includes(column.id) && cards.some(column.holds),
  );
}

// toggleVirtual turns one virtual column on or off, keeping the fixed order.
export function toggleVirtual(
  chosen: VirtualColumn[],
  id: VirtualColumn,
  on: boolean,
): VirtualColumn[] {
  return VIRTUAL_COLUMNS.map((column) => column.id).filter((column) =>
    column === id ? on : chosen.includes(column),
  );
}

// placeVirtual puts the shown virtual columns after In progress, before Ready
// to review, where the work they mirror sits in the flow.
export function placeVirtual<T extends { id: string }>(
  real: T[],
  mirrors: T[],
): T[] {
  const at = real.findIndex((column) => column.id === "in-progress") + 1;
  return [...real.slice(0, at), ...mirrors, ...real.slice(at)];
}
