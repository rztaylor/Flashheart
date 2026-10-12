import { useCallback, useRef, useState } from "react";

import {
  type Column,
  columnOf,
  STATUSES,
  type Status,
  statusWord,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import {
  answerQuestion,
  archiveTicket,
  blockedReasons,
  moveTicket,
  unarchiveTicket,
} from "../../api/edit";
import type { ToastMessage } from "../../components/Toast";
import { messageOf } from "../../model/errors";
import { markSaved, type PendingMoves, serial, settle } from "./pending";

export interface Movable {
  id: string;
  title: string;
  column: Column;
  rejected?: boolean;
}

// Placement puts a ticket directly after another in its column ("" for the
// top; EDIT-9). undoAfter is where it was, for Undo.
export interface Placement {
  after?: string;
  undoAfter?: string;
}

export interface BlockedMove {
  ticket: Movable;
  to: Status;
  reasons: string[];
  placement: Placement;
}

export type { Pending } from "./pending";

const statusTitle = (status: Status) =>
  STATUSES.find((item) => item.id === status)?.title ?? status;

// notSaved is a failed save's words when the error has none.
const notSaved = "The change could not be saved";

// useEditing coordinates moves and archiving from any view (EDIT-1, EDIT-2,
// EDIT-3, EDIT-8), rejecting included (a move to the rejected status, which
// shows in Done; FH-58): it shows the new column and place at once (EDIT-9), asks
// for a reason before starting a blocked ticket, and announces each result
// with Undo, which puts the ticket back in its old column and place.
// remark, when given, picks a marginal remark for a successful move's toast
// (FH-22); it is asked only after the save succeeds.
export function useEditing(
  fetcher: AuthenticatedFetch,
  onChanged: () => void,
  remark?: (ticket: Movable, to: Status) => ToastMessage["aside"],
) {
  const [toast, setToast] = useState<ToastMessage | null>(null);
  const [blocked, setBlocked] = useState<BlockedMove | null>(null);
  const [pending, setPending] = useState<PendingMoves>({});
  const counter = useRef(0);
  // Moves are saved one at a time, in the order they were made; seq tells
  // a move's entry from a later one for the same ticket.
  const queue = useRef(serial());
  const seq = useRef(0);

  const notify = useCallback((message: Omit<ToastMessage, "id">) => {
    counter.current += 1;
    setToast({ id: counter.current, ...message });
  }, []);
  const dismiss = useCallback(() => setToast(null), []);

  const move = useCallback(
    async (
      ticket: Movable,
      to: Status,
      reason = "",
      placement: Placement = {},
    ) => {
      const { after, undoAfter } = placement;
      const from = statusWord(ticket);
      if (from === to && after === undefined) return;
      seq.current += 1;
      const mine = seq.current;
      setPending((current) => ({
        ...current,
        [ticket.id]: { column: columnOf(to), after, saved: false, seq: mine },
      }));
      try {
        const saved = await queue.current(() =>
          moveTicket(fetcher, ticket.id, to, "", reason, after),
        );
        setPending((current) => markSaved(current, ticket.id, mine));
        notify({
          text:
            to === "rejected"
              ? `Rejected ${ticket.id}.`
              : from === to
                ? `Moved ${ticket.id} in ${statusTitle(to)}.`
                : `Moved ${ticket.id} to ${statusTitle(to)}.`,
          details: saved.warnings,
          aside: remark?.(ticket, to),
          action: {
            label: "Undo",
            run: () =>
              void move(
                {
                  ...ticket,
                  column: columnOf(to),
                  rejected: to === "rejected",
                },
                from,
                "",
                after === undefined
                  ? {}
                  : { after: undoAfter ?? "", undoAfter: after },
              ),
          },
        });
        onChanged();
      } catch (error) {
        setPending((current) => settle(current, ticket.id, mine));
        const reasons = blockedReasons(error);
        if (reasons) {
          setBlocked({ ticket, to, reasons, placement });
          return;
        }
        notify({
          text: `${ticket.id} was not moved: ${messageOf(error, notSaved)}`,
        });
      }
    },
    [fetcher, notify, onChanged, remark],
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
                    text: `${ticket.id} was not restored: ${messageOf(error, notSaved)}`,
                  }),
                ),
          },
        });
        onChanged();
      } catch (error) {
        notify({
          text: `${ticket.id} was not archived: ${messageOf(error, notSaved)}`,
        });
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
        return `Not sent: ${messageOf(error, notSaved)}`;
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
