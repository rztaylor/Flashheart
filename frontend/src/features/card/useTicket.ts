import { useCallback } from "react";
import { fetchTicket, type TicketDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import {
  conflictOf,
  type Saved,
  setCriterion,
  setReviewStep,
} from "../../api/edit";
import { messageOf } from "../../model/errors";
import { type Resource, useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";

// TicketState is one loaded ticket and the saves its sections make. A save
// that is not allowed (read-only server, no hash, answers off) is absent.
export interface TicketState {
  // resource is the loaded ticket; reload loads it again (Try again).
  resource: Resource<TicketDetail> & { reload(): void };
  detail?: TicketDetail;
  // editable: the ticket can be moved, archived and edited.
  editable: boolean;
  // saved reloads after an Edit tab save and says what was saved.
  saved(result: Saved | undefined, what: string): void;
  // toggle ticks or unticks an acceptance criterion (CARD-3).
  toggle?(index: number, checked: boolean): Promise<boolean>;
  // verify ticks a review's How to Verify step (REV-3).
  verify?(index: number, checked: boolean): Promise<boolean>;
  // answer sends an answer to an agent's question (CARD-6).
  answer?(question: string, text: string): Promise<string | undefined>;
}

// useTicket loads a ticket, reloading it when the board revision moves
// (LIFE-3), and wires its saves: each reloads the ticket, and a failed tick
// says why and resolves false so the box goes back.
export function useTicket({
  fetcher,
  id,
  revision,
  editing,
  answers,
}: {
  fetcher: AuthenticatedFetch;
  id: string;
  revision: number;
  editing?: Editing;
  answers: boolean;
}): TicketState {
  const load = useCallback(
    (signal: AbortSignal) => fetchTicket(fetcher, id, signal),
    [fetcher, id],
  );
  const resource = useResource(load, id, revision);
  const detail = resource.status === "ready" ? resource.data : undefined;
  const editable = !!editing && !!detail?.hash;

  const saved = (result: Saved | undefined, what: string) => {
    resource.reload();
    if (result && editing)
      editing.notify({
        text: `${what} ${id}.`,
        details: result.warnings,
      });
  };
  const tick = (save: Promise<Saved>, what: string): Promise<boolean> =>
    save
      .then(() => {
        resource.reload();
        return true;
      })
      .catch((error: unknown) => {
        resource.reload();
        editing?.notify({
          text: conflictOf(error)
            ? `${id} changed on disk; showing the current ${what}.`
            : `Not saved: ${messageOf(error, "unknown error")}`,
        });
        return false;
      });
  const review = detail?.review;
  return {
    resource,
    detail,
    editable,
    saved,
    toggle:
      editable && detail && editing
        ? (index, checked) =>
            tick(
              setCriterion(fetcher, detail.id, detail.hash, index, checked),
              "criteria",
            )
        : undefined,
    verify:
      editing && detail && review?.hash
        ? (index, checked) =>
            tick(
              setReviewStep(fetcher, detail.id, review.hash, index, checked),
              "review",
            )
        : undefined,
    answer:
      editing && detail && answers
        ? (question, text) =>
            editing.answer(question, text).finally(() => resource.reload())
        : undefined,
  };
}
