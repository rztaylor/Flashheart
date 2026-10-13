import {
  type FormEvent,
  type KeyboardEvent,
  type ReactNode,
  useId,
  useState,
} from "react";

import type { Comment } from "../../api/board";
import type { NewComment, RequestState } from "../../api/edit";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, TextArea } from "../../components/Field";
import type { IconName } from "../../components/Icon";
import { Pill } from "../../components/Pill";
import {
  focusItem,
  focusList,
  useKeepFocus,
} from "../../components/useKeepFocus";
import { messageOf } from "../../model/errors";
import {
  excerpt,
  requestChoices,
  requestStateOf,
  requestStates,
  type RequestState as Shown,
} from "../../model/requests";
import { sendKeys } from "../editing/InPlace";

// Change requests in review (REV-6, FH-126) on the ticket's full page: the
// list of requests still open or addressed at the top of the page, with
// Resolve and Reopen, the Request changes dialog, and a request's state as
// a pill (also on its comment in Activity).

const requestIcons: Record<Shown, IconName> = {
  "change-requested": "comment",
  addressed: "review",
  resolved: "check",
};

// RequestPill is a comment's state as a change request, a state pill with
// its icon and word; nothing for a plain comment.
export function RequestPill({
  state,
  className,
}: {
  state: string;
  className?: string;
}) {
  const shown = requestStateOf(state);
  if (!shown) return null;
  return (
    <Pill
      tone={requestStates[shown].tone}
      icon={requestIcons[shown]}
      className={className}
    >
      {requestStates[shown].label}
    </Pill>
  );
}

// ChangeRequestList lists the ticket's change requests (REV-6): open ones
// first, then addressed, each with its state, who asked and when and its
// text, then resolved ones on one quiet line each. Where the page can
// save, an addressed request offers Resolve and Reopen, an open one
// Resolve (the person drops it) and a resolved one Reopen. A request that
// changes state moves in the list, and keyboard focus follows it.
export function ChangeRequestList({
  requests,
  render,
  onState,
}: {
  requests: Comment[];
  // render shows a request's text as the page shows comments.
  render(text: string): ReactNode;
  onState?(comment: string, state: RequestState): Promise<void>;
}) {
  const keepFocus = useKeepFocus();
  return (
    <div ref={keepFocus}>
      <ul
        {...focusList("requests")}
        className="divide-y divide-rule overflow-hidden rounded-card border border-rule bg-card"
      >
        {requests.map((request) => (
          <RequestRow
            key={request.id}
            request={request}
            render={render}
            onState={onState}
          />
        ))}
      </ul>
    </div>
  );
}

function RequestRow({
  request,
  render,
  onState,
}: {
  request: Comment;
  render(text: string): ReactNode;
  onState?(comment: string, state: RequestState): Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const errorId = useId();
  const change = async (state: RequestState) => {
    if (!onState || busy) return;
    setBusy(true);
    setError("");
    try {
      await onState(request.id, state);
    } catch (failure) {
      setError(messageOf(failure, "Not saved"));
    } finally {
      setBusy(false);
    }
  };
  const date = new Date(request.time);
  const short = excerpt(request.body, 40);
  const resolved = request.state === "resolved";
  // While a change saves, its buttons stay focusable (aria-disabled), so
  // focus is not lost; the row then moves and focus follows it.
  const action = (
    label: string,
    state: RequestState,
    variant: "secondary" | "quiet",
  ) => (
    <Button
      variant={variant}
      className="h-8 py-0 text-xs aria-disabled:cursor-wait aria-disabled:opacity-50"
      aria-label={`${label} “${short}”`}
      aria-describedby={error ? errorId : undefined}
      aria-disabled={busy || undefined}
      onClick={() => void change(state)}
    >
      {label}
    </Button>
  );
  const when = Number.isNaN(date.getTime()) ? null : (
    <time dateTime={request.time}>
      {date.toLocaleString([], {
        day: "numeric",
        month: "short",
        hour: "2-digit",
        minute: "2-digit",
      })}
    </time>
  );
  if (resolved)
    return (
      <li
        {...focusItem(request.id)}
        className="flex flex-wrap items-center gap-x-2 gap-y-1 px-4 py-2 text-xs text-ink-muted"
      >
        <RequestPill state={request.state} />
        <span className="min-w-0 flex-1 truncate" title={request.body}>
          {excerpt(request.body)}
        </span>
        <span>{request.by}</span>
        {onState ? action("Reopen", "change-requested", "quiet") : null}
        {error ? (
          <p id={errorId} role="alert" className="w-full text-sm text-danger">
            {error}
          </p>
        ) : null}
      </li>
    );
  return (
    <li {...focusItem(request.id)} className="flex flex-col gap-2 px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-muted">
        <RequestPill state={request.state} />
        <span className="font-semibold text-ink">{request.by}</span>
        {when}
        <span className="sm:ml-auto">
          {request.state === "addressed"
            ? "The agent addressed it: resolve or reopen it"
            : "Waiting for the agent"}
        </span>
      </div>
      <div className="min-w-0 text-sm">{render(request.body)}</div>
      {onState ? (
        <div className="flex flex-wrap items-center gap-2">
          {action("Resolve", "resolved", "secondary")}
          {request.state === "addressed"
            ? action("Reopen", "change-requested", "quiet")
            : null}
          {error ? (
            <p id={errorId} role="alert" className="text-sm text-danger">
              {error}
            </p>
          ) : null}
        </div>
      ) : null}
    </li>
  );
}

// RequestChangesDialog asks for changes on a ticket in review (REV-6):
// the person picks people's comments to send back (open requests come
// checked) and may write a new one, then Request changes moves the ticket
// back to In progress. It needs at least one; Ctrl or Cmd and Enter sends;
// a failed request keeps the dialog open with the reason.
export function RequestChangesDialog({
  ticket,
  comments,
  onSubmit,
  onClose,
}: {
  ticket: string;
  comments: Comment[];
  onSubmit(ids: string[], comment?: NewComment): Promise<void>;
  onClose(): void;
}) {
  const choices = requestChoices(comments);
  const [chosen, setChosen] = useState<Set<string>>(
    () =>
      new Set(
        choices
          .filter((choice) => choice.checked)
          .map((choice) => choice.comment.id),
      ),
  );
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const formId = useId();
  const ready = chosen.size > 0 || text.trim() !== "";
  const send = async () => {
    if (busy) return;
    if (!ready) {
      setError("Choose a comment or write a change to request.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await onSubmit(
        choices
          .map((choice) => choice.comment.id)
          .filter((id) => chosen.has(id))
          .reverse(),
        text.trim() ? { body: text } : undefined,
      );
      onClose();
    } catch (failure) {
      setError(messageOf(failure, "Changes not requested"));
      setBusy(false);
    }
  };
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void send();
    }
  };
  const onSubmitForm = (event: FormEvent) => {
    event.preventDefault();
    void send();
  };
  const toggle = (id: string, on: boolean) =>
    setChosen((current) => {
      const next = new Set(current);
      if (on) next.add(id);
      else next.delete(id);
      return next;
    });
  return (
    <Dialog
      title={`Request changes on ${ticket}`}
      onClose={onClose}
      actions={
        <>
          <Button variant="quiet" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            form={formId}
            variant="primary"
            aria-disabled={busy || undefined}
            className="aria-disabled:cursor-wait aria-disabled:opacity-50"
          >
            {busy ? "Requesting…" : "Request changes"}
          </Button>
        </>
      }
    >
      <form
        id={formId}
        onSubmit={onSubmitForm}
        onKeyDown={onKeyDown}
        className="flex flex-col gap-4"
      >
        <p className="text-ink-muted">
          {ticket} goes back to In progress on its branch. The session holding
          it gets the changes with its next prompt; with none, the next session
          that picks it up sees them.
        </p>
        {choices.length > 0 ? (
          <fieldset className="flex flex-col gap-1">
            <legend className="mb-1 text-xs font-medium text-ink">
              Comments to send back
            </legend>
            <ul className="flex max-h-56 flex-col gap-1 overflow-y-auto">
              {choices.map(({ comment }) => (
                <li key={comment.id}>
                  <label className="flex cursor-pointer items-start gap-2 rounded-control px-2 py-1.5 hover:bg-well">
                    <input
                      type="checkbox"
                      checked={chosen.has(comment.id)}
                      onChange={(event) =>
                        toggle(comment.id, event.target.checked)
                      }
                      className="mt-0.5 size-4 shrink-0 accent-[var(--fh-select)]"
                    />
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <span className="text-ink">{excerpt(comment.body)}</span>
                      <span className="flex flex-wrap items-center gap-1.5 text-xs text-ink-muted">
                        {comment.by}
                        <RequestPill state={comment.state} />
                      </span>
                    </span>
                  </label>
                </li>
              ))}
            </ul>
          </fieldset>
        ) : null}
        <FormField
          label="New change request"
          hint={`Optional. ${sendKeys} requests the changes.`}
        >
          <TextArea
            value={text}
            onChange={setText}
            rows={4}
            placeholder="What should change…"
            autoFocus={choices.length === 0}
          />
        </FormField>
        {error ? (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        ) : null}
      </form>
    </Dialog>
  );
}
