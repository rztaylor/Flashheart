// Virtual columns (VIEW-2): computed columns that mirror tickets from their
// real columns while an agent run needs the user. They stand between In
// progress and Ready to review. Agent working is a State filter instead, so
// a working ticket shows once, in its own column (FH-42).
import { type Card, COLUMNS, type Column } from "../api/board";
import type { HideableColumn, VirtualColumn } from "../api/preferences";

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

// shownColumns lists the real columns the Board shows: every one but a
// hidden Backlog (FH-41).
export function shownColumns(hidden: HideableColumn[]) {
  return COLUMNS.filter(
    (column) => !(hidden as readonly string[]).includes(column.id),
  );
}

// setColumnShown shows or hides one hideable column.
export function setColumnShown(
  hidden: HideableColumn[],
  id: HideableColumn,
  shown: boolean,
): HideableColumn[] {
  const rest = hidden.filter((column) => column !== id);
  return shown ? rest : [...rest, id];
}

// neighbourColumn is the shown column a step left (-1) or right (1) of
// from, where Shift with an arrow moves a ticket; none past either end.
export function neighbourColumn(
  hidden: HideableColumn[],
  from: Column,
  step: -1 | 1,
): Column | undefined {
  const shown = shownColumns(hidden);
  const index = shown.findIndex((column) => column.id === from);
  return index < 0 ? undefined : shown[index + step]?.id;
}
