import { useCallback } from "react";
import {
  fetchProjectBoard,
  fetchTicket,
  type TicketDetail,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import {
  addComment,
  editComment,
  type FieldValue,
  type NewComment,
  patchTicket,
  type RequestState,
  requestChanges,
  type Saved,
  saveSection,
  setCriterion,
  setRequestState,
  setReviewStep,
} from "../../api/edit";
import { messageOf } from "../../model/errors";
import { sentWords } from "../../model/requests";
import { type Resource, useResource } from "../../state/useResource";
import { saveWithBase } from "../editing/precondition";
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
  // edit saves the full page's in-place edits (FH-66) on base: typed
  // fields (EDIT-6), or one section's text.
  edit?: TicketEdits;
  // comment posts a person's comment (STO-8, FH-125), then reloads and
  // says so; a failed post rejects with the reason.
  comment?(comment: NewComment): Promise<void>;
  // editComment saves a comment's new text on base, the hash of the
  // activity it was read from.
  editComment?(id: string, base: string, body: string): Promise<Saved>;
  // requestChanges asks for changes on a ticket in review (REV-6, FH-126),
  // then reloads and says where they went; a failure rejects with the
  // reason.
  requestChanges?(comments: string[], comment?: NewComment): Promise<void>;
  // setRequestState resolves or reopens a change request, then reloads.
  setRequestState?(comment: string, state: RequestState): Promise<void>;
}

export interface TicketEdits {
  fields(base: string, fields: Record<string, FieldValue>): Promise<Saved>;
  section(base: string, section: string, content: string): Promise<Saved>;
  // tickets are the project's tickets, for the depends-on picker.
  tickets(signal: AbortSignal): Promise<{ id: string; title: string }[]>;
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
  // tick saves one box on the version the panel shows (base), under the
  // same precondition as the Edit tab's saves. A conflict is not offered as
  // a choice: the ticket reloads and the toast says so.
  const tick = (
    base: string,
    send: (base: string) => Promise<Saved>,
    what: string,
  ): Promise<boolean> =>
    saveWithBase(base, send).then((outcome) => {
      resource.reload();
      if (outcome.kind === "saved") return true;
      editing?.notify({
        text:
          outcome.kind === "conflict"
            ? `${id} changed on disk; showing the current ${what}.`
            : `Not saved: ${messageOf(outcome.error, "unknown error")}`,
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
              detail.hash,
              (base) => setCriterion(fetcher, detail.id, base, index, checked),
              "criteria",
            )
        : undefined,
    verify:
      editing && detail && review?.hash
        ? (index, checked) =>
            tick(
              review.hash,
              (base) => setReviewStep(fetcher, detail.id, base, index, checked),
              "review",
            )
        : undefined,
    edit:
      editable && detail
        ? {
            fields: (base, fields) =>
              patchTicket(fetcher, detail.id, base, fields),
            section: (base, section, content) =>
              saveSection(fetcher, detail.id, base, section, content),
            tickets: (signal) =>
              fetchProjectBoard(fetcher, detail.project, false, signal).then(
                (board) => board.cards.map(({ id, title }) => ({ id, title })),
              ),
          }
        : undefined,
    comment:
      editable && detail && editing
        ? (comment) =>
            addComment(fetcher, detail.id, comment).then(() => {
              resource.reload();
              editing.notify({ text: `Commented on ${detail.id}.` });
            })
        : undefined,
    editComment:
      editable && detail
        ? (comment, base, body) =>
            editComment(fetcher, detail.id, comment, base, body)
        : undefined,
    requestChanges:
      editable && detail && editing
        ? (comments, comment) =>
            requestChanges(fetcher, detail.id, comments, comment).then(
              (sent) => {
                resource.reload();
                editing.notify({
                  text: sentWords(detail.id, sent.count, sent.sentToShort),
                  details: sent.warnings,
                });
              },
            )
        : undefined,
    setRequestState:
      editable && detail && editing
        ? (comment, state) =>
            setRequestState(fetcher, detail.id, comment, state).then((sent) => {
              resource.reload();
              editing.notify({
                text:
                  state === "resolved"
                    ? `Resolved the change request on ${detail.id}.`
                    : sentWords(detail.id, 1, sent.sentToShort, "Reopened"),
                details: sent.warnings,
              });
            })
        : undefined,
    answer:
      editing && detail && answers
        ? (question, text) =>
            editing.answer(question, text).finally(() => resource.reload())
        : undefined,
  };
}
