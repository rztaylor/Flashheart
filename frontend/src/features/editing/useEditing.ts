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

export interface BlockedMove {
  ticket: Movable;
  to: Column;
  reasons: string[];
}

const columnTitle = (column: Column) =>
  COLUMNS.find((item) => item.id === column)?.title ?? column;

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The change could not be saved";

// useEditing coordinates moves and archiving from any view (EDIT-1, EDIT-2,
// EDIT-3, EDIT-8): it shows the new column at once, asks for a reason before
// starting a blocked ticket, and announces each result with Undo.
export function useEditing(fetcher: AuthenticatedFetch, onChanged: () => void) {
  const [toast, setToast] = useState<ToastMessage | null>(null);
  const [blocked, setBlocked] = useState<BlockedMove | null>(null);
  const [pending, setPending] = useState<Record<string, Column>>({});
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
    async (ticket: Movable, to: Column, reason = "") => {
      if (ticket.column === to) return;
      setPending((current) => ({ ...current, [ticket.id]: to }));
      try {
        const saved = await moveTicket(fetcher, ticket.id, to, "", reason);
        notify({
          text: `Moved ${ticket.id} to ${columnTitle(to)}.`,
          details: saved.warnings,
          action: {
            label: "Undo",
            run: () => void move({ ...ticket, column: to }, ticket.column),
          },
        });
        onChanged();
      } catch (error) {
        settle(ticket.id);
        const reasons = blockedReasons(error);
        if (reasons) {
          setBlocked({ ticket, to, reasons });
          return;
        }
        notify({ text: `${ticket.id} was not moved: ${failure(error)}` });
      }
    },
    [fetcher, notify, onChanged, settle],
  );

  const confirmBlocked = useCallback(
    (reason: string) => {
      if (!blocked) return;
      setBlocked(null);
      void move(blocked.ticket, blocked.to, reason);
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
    // reconcile drops optimistic columns the board data now agrees with, or
    // whose tickets are gone.
    reconcile: useCallback((cards: { id: string; column: Column }[]) => {
      setPending((current) => {
        const ids = Object.keys(current);
        if (ids.length === 0) return current;
        const columns = new Map(cards.map((card) => [card.id, card.column]));
        const next = Object.fromEntries(
          Object.entries(current).filter(
            ([id, column]) => columns.has(id) && columns.get(id) !== column,
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
