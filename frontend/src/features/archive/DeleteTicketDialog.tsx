import { useCallback } from "react";

import {
  type DeletePlan,
  deleteArchived,
  fetchDeletePlan,
} from "../../api/archive";
import type { AuthenticatedFetch } from "../../api/client";
import { TypedDeleteDialog } from "./TypedDeleteDialog";

// DeleteTicketDialog deletes an archived ticket for good (EDIT-8), after
// listing everything the delete touches and the typed id.
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
  const load = useCallback(
    (signal?: AbortSignal) => fetchDeletePlan(fetcher, project, id, signal),
    [fetcher, project, id],
  );
  return (
    <TypedDeleteDialog<DeletePlan>
      title={() => `Delete ${id} permanently?`}
      word={id}
      hint="This removes the ticket's folder from the board."
      load={load}
      remove={(plan, typed) =>
        deleteArchived(fetcher, project, id, plan.token, typed)
      }
      onClose={onClose}
      onDeleted={onDeleted}
    >
      {(plan) => (
        <>
          {plan.title ? <p className="mb-3 font-medium">{plan.title}</p> : null}
          <DeletePreview plan={plan} />
        </>
      )}
    </TypedDeleteDialog>
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
