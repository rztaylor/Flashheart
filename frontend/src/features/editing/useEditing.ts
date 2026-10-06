import { useCallback, useRef, useState } from "react";

import { COLUMNS, type Column } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import {
  answerQuestion,
  archiveTicket,
  blockedReasons,
  moveTicket,
  unarchiveTicket,
} from "../../api/edit";
import type { ToastMessage } from "../../components/Toast";

export interface Movable {
  id: string;
  title: string;
  column: Column;
}

// Placement puts a ticket directly after another in its column ("" for the
// top; EDIT-9). undoAfter is where it was, for Undo.
export interface Placement {
  after?: string;
  undoAfter?: string;
}

export interface BlockedMove {
  ticket: Movable;
  to: Column;
  reasons: string[];
  placement: Placement;
}

// Pending is a move shown before the board data has it: its column, its
// placement, and whether the save has returned.
export interface Pending {
  column: Column;
  after?: string;
  saved: boolean;
}

const columnTitle = (column: Column) =>
  COLUMNS.find((item) => item.id === column)?.title ?? column;

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The change could not be saved";

// useEditing coordinates moves and archiving from any view (EDIT-1, EDIT-2,
// EDIT-3, EDIT-8): it shows the new column and place at once (EDIT-9), asks
// for a reason before starting a blocked ticket, and announces each result
// with Undo, which puts the ticket back in its old column and place.
// remark, when given, picks a marginal remark for a successful move's toast
// (FH-22); it is asked only after the save succeeds.
export function useEditing(
  fetcher: AuthenticatedFetch,
  onChanged: () => void,
  remark?: (ticket: Movable, to: Column) => ToastMessage["aside"],
) {
  const [toast, setToast] = useState<ToastMessage | null>(null);
  const [blocked, setBlocked] = useState<BlockedMove | null>(null);
  const [pending, setPending] = useState<Record<string, Pending>>({});
  const counter = useRef(0);

  const notify = useCallback((message: Omit<ToastMessage, "id">) => {
    counter.current += 1;
    setToast({ id: counter.current, ...message });
  }, []);
  const dismiss = useCallback(() => setToast(null), []);
  const settle = useCallback((id: string) => {
    setPending((current) => {
      const { [id]: _, ...rest } = current;
      return rest;
    });
  }, []);

  const move = useCallback(
    async (
      ticket: Movable,
      to: Column,
      reason = "",
      placement: Placement = {},
    ) => {
      const { after, undoAfter } = placement;
      if (ticket.column === to && after === undefined) return;
      setPending((current) => ({
        ...current,
        [ticket.id]: { column: to, after, saved: false },
      }));
      try {
        const saved = await moveTicket(
          fetcher,
          ticket.id,
          to,
          "",
          reason,
          after,
        );
        setPending((current) => {
          const entry = current[ticket.id];
          return entry
            ? { ...current, [ticket.id]: { ...entry, saved: true } }
            : current;
        });
        notify({
          text:
            ticket.column === to
              ? `Moved ${ticket.id} in ${columnTitle(to)}.`
              : `Moved ${ticket.id} to ${columnTitle(to)}.`,
          details: saved.warnings,
          aside: remark?.(ticket, to),
          action: {
            label: "Undo",
            run: () =>
              void move(
                { ...ticket, column: to },
                ticket.column,
                "",
                after === undefined
                  ? {}
                  : { after: undoAfter ?? "", undoAfter: after },
              ),
          },
        });
        onChanged();
      } catch (error) {
        settle(ticket.id);
        const reasons = blockedReasons(error);
        if (reasons) {
          setBlocked({ ticket, to, reasons, placement });
          return;
        }
        notify({ text: `${ticket.id} was not moved: ${failure(error)}` });
      }
    },
    [fetcher, notify, onChanged, settle, remark],
  );

  const confirmBlocked = useCallback(
    (reason: string) => {
      if (!blocked) return;
      setBlocked(null);
      void move(blocked.ticket, blocked.to, reason, blocked.placement);
    },
    [blocked, move],
  );

  const archive = useCallback(
    async (ticket: Movable) => {
      try {
        await archiveTicket(fetcher, ticket.id);
        notify({
          text: `Archived ${ticket.id}.`,
          action: {
            label: "Undo",
            run: () =>
              void unarchiveTicket(fetcher, ticket.id)
                .then(onChanged)
                .catch((error: unknown) =>
                  notify({
                    text: `${ticket.id} was not restored: ${failure(error)}`,
                  }),
                ),
          },
        });
        onChanged();
      } catch (error) {
        notify({ text: `${ticket.id} was not archived: ${failure(error)}` });
      }
    },
    [fetcher, notify, onChanged],
  );

  // answer sends an answer to an agent's question (CARD-6) from any view.
  // It resolves to an error message to show beside the question, or
  // undefined once sent; warnings (the ticket's notes not updated) are
  // announced.
  const answer = useCallback(
    async (id: string, text: string): Promise<string | undefined> => {
      try {
        const saved = await answerQuestion(fetcher, id, text);
        if (saved.warnings.length > 0)
          notify({ text: "Answer sent.", details: saved.warnings });
        return undefined;
      } catch (error) {
        return `Not sent: ${failure(error)}`;
      } finally {
        onChanged();
      }
    },
    [fetcher, notify, onChanged],
  );

  return {
    toast,
    dismiss,
    notify,
    answer,
    blocked,
    confirmBlocked,
    cancelBlocked: useCallback(() => setBlocked(null), []),
    pending,
    // reconcile drops optimistic moves the board data now agrees with (a
    // placement once its save has returned), or whose tickets are gone.
    reconcile: useCallback((cards: { id: string; column: Column }[]) => {
      setPending((current) => {
        const ids = Object.keys(current);
        if (ids.length === 0) return current;
        const columns = new Map(cards.map((card) => [card.id, card.column]));
        const next = Object.fromEntries(
          Object.entries(current).filter(
            ([id, entry]) =>
              columns.has(id) &&
              (columns.get(id) !== entry.column ||
                (entry.after !== undefined && !entry.saved)),
          ),
        );
        return Object.keys(next).length === ids.length ? current : next;
      });
    }, []),
    move,
    archive,
  };
}

export type Editing = ReturnType<typeof useEditing>;
