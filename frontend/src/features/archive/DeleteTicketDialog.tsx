import { useCallback, useEffect, useState } from "react";

import {
  type DeletePlan,
  deleteArchived,
  fetchDeletePlan,
} from "../../api/archive";
import { ApiError, type AuthenticatedFetch } from "../../api/client";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, TextInput } from "../../components/Field";
import { StateNote } from "../../components/StateNote";
import { confirmsDelete } from "../../model/archive";

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The request failed";

// DeleteTicketDialog deletes an archived ticket for good (EDIT-8): it lists
// everything the delete touches and asks for the ticket's id before the
// delete button works. If anything listed changed meanwhile, the server
// refuses, and the list is reloaded for another look.
export function DeleteTicketDialog({
  fetcher,
  project,
  id,
  onClose,
  onDeleted,
}: {
  fetcher: AuthenticatedFetch;
  project: string;
  id: string;
  onClose(): void;
  onDeleted(): void;
}) {
  const [plan, setPlan] = useState<DeletePlan | null>(null);
  const [error, setError] = useState("");
  const [changed, setChanged] = useState(false);
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    (signal?: AbortSignal) =>
      fetchDeletePlan(fetcher, project, id, signal)
        .then((next) => {
          setPlan(next);
          setError("");
        })
        .catch((reason: unknown) => {
          if (!signal?.aborted) setError(failure(reason));
        }),
    [fetcher, project, id],
  );
  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const ready = !!plan && confirmsDelete(typed, id) && !busy;
  const confirm = async () => {
    if (!plan || !ready) return;
    setBusy(true);
    try {
      await deleteArchived(fetcher, project, id, plan.token, typed.trim());
      onDeleted();
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 409) {
        setChanged(true);
        await load();
      } else {
        setError(failure(reason));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      title={`Delete ${id} permanently?`}
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>Keep it archived</Button>
          <Button variant="primary" disabled={!ready} onClick={confirm}>
            {busy ? "Deleting…" : "Delete permanently"}
          </Button>
        </>
      }
    >
      {plan ? (
        <>
          {plan.title ? <p className="mb-3 font-medium">{plan.title}</p> : null}
          <DeletePreview plan={plan} />
          {changed ? (
            <div className="mt-4">
              <StateNote kind="warning">
                Something here changed since you opened this. The list is up to
                date now; check it before deleting.
              </StateNote>
            </div>
          ) : null}
          <div className="mt-5">
            <FormField
              label={`Type ${id} to confirm`}
              hint="This removes the ticket's folder from the board."
            >
              <TextInput
                value={typed}
                onChange={setTyped}
                autoFocus
                spellCheck={false}
              />
            </FormField>
          </div>
        </>
      ) : error ? null : (
        <p className="text-ink-muted">Checking what the delete touches…</p>
      )}
      {error ? (
        <p role="alert" className="mt-3 text-danger">
          {error}
        </p>
      ) : null}
    </Dialog>
  );
}

// DeletePreview says what a delete removes and rewrites.
export function DeletePreview({ plan }: { plan: DeletePlan }) {
  return (
    <div className="flex flex-col gap-4">
      <p>
        Deleting {plan.id} removes it for good. Board files keep no history, so
        this cannot be undone.
      </p>
      <section aria-label="Files deleted">
        <h3 className="mb-1 heading-cut">
          Its folder ({plan.files.length}{" "}
          {plan.files.length === 1 ? "file" : "files"})
        </h3>
        <ul className="max-h-32 overflow-y-auto rounded-card border border-rule bg-well px-3 py-2 font-mono text-xs">
          {plan.files.map((file) => (
            <li key={file} className="truncate" title={file}>
              {file}
            </li>
          ))}
        </ul>
      </section>
      {plan.tickets.length > 0 ? (
        <section aria-label="Tickets that depend on it">
          <h3 className="mb-1 heading-cut">Tickets that depend on it</h3>
          <ul className="flex flex-col gap-1">
            {plan.tickets.map((ticket) => (
              <li key={`${ticket.project}/${ticket.id}`}>
                <span className="font-semibold tabular-nums">{ticket.id}</span>{" "}
                {ticket.title}{" "}
                <span className="text-ink-muted">({ticket.project})</span>
              </li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-ink-muted">
            {plan.id} comes out of their depends-on, so nothing waits on it.
          </p>
        </section>
      ) : null}
      {plan.workstreams.length > 0 ? (
        <section aria-label="Workstreams that list it">
          <h3 className="mb-1 heading-cut">Workstreams that list it</h3>
          <ul className="flex flex-col gap-1">
            {plan.workstreams.map((workstream) => (
              <li key={workstream.slug}>{workstream.title}</li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-ink-muted">
            {plan.id} comes off their ticket lists.
          </p>
        </section>
      ) : null}
      {plan.tickets.length === 0 && plan.workstreams.length === 0 ? (
        <p className="text-ink-muted">
          No other ticket or workstream refers to {plan.id}.
        </p>
      ) : null}
      <p className="text-xs text-ink-muted">
        Mentions of {plan.id} in ticket text stay as plain text. Its id is
        retired and never given to a new ticket.
      </p>
    </div>
  );
}
