// The Board's real columns: which it shows, and where Shift with an arrow
// moves a ticket. Needs you and Agent working are filters, not columns, so
// a ticket shows once, in its own column (FH-42, FH-44).
import { COLUMNS, type Column } from "../api/board";
import type { HideableColumn } from "../api/preferences";

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
