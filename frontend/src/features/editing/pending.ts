import type { Column } from "../../api/board";

// Pending is a move shown before the board data has it: its column, its
// placement (EDIT-9), whether its save has returned, and its sequence, so
// only the save that made an entry settles it.
export interface Pending {
  column: Column;
  after?: string;
  saved: boolean;
  seq: number;
}

export type PendingMoves = Record<string, Pending>;

// markSaved records that move seq of a ticket has been saved, unless a
// later move has replaced it.
export function markSaved(
  pending: PendingMoves,
  id: string,
  seq: number,
): PendingMoves {
  const entry = pending[id];
  return entry?.seq === seq
    ? { ...pending, [id]: { ...entry, saved: true } }
    : pending;
}

// settle drops move seq of a ticket (it failed), unless a later move has
// replaced it.
export function settle(
  pending: PendingMoves,
  id: string,
  seq: number,
): PendingMoves {
  if (pending[id]?.seq !== seq) return pending;
  const { [id]: _, ...rest } = pending;
  return rest;
}

// serial runs tasks one after another in the order they are given, so two
// quick moves reach the server in the order they were made.
export function serial() {
  let tail: Promise<unknown> = Promise.resolve();
  return <T>(task: () => Promise<T>): Promise<T> => {
    const result = tail.then(task, task);
    tail = result.catch(() => undefined);
    return result;
  };
}
