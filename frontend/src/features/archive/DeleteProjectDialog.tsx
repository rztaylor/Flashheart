import { useCallback, useEffect, useState } from "react";

import {
  deleteProject,
  fetchProjectDeletePlan,
  type ProjectDeletePlan,
} from "../../api/archive";
import { ApiError, type AuthenticatedFetch } from "../../api/client";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, TextInput } from "../../components/Field";
import { StateNote } from "../../components/StateNote";
import { confirmsDelete } from "../../model/archive";

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The request failed";

// DeleteProjectDialog deletes an archived project for good (PRJ-5): it lists
// what goes and the other projects' tickets whose dependencies are removed,
// and asks for the project's name before the delete button works.
export function DeleteProjectDialog({
  fetcher,
  project,
  onClose,
  onDeleted,
}: {
  fetcher: AuthenticatedFetch;
  project: string;
  onClose(): void;
  onDeleted(displayName: string): void;
}) {
  const [plan, setPlan] = useState<ProjectDeletePlan | null>(null);
  const [error, setError] = useState("");
  const [changed, setChanged] = useState(false);
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    (signal?: AbortSignal) =>
      fetchProjectDeletePlan(fetcher, project, signal)
        .then((next) => {
          setPlan(next);
          setError("");
        })
        .catch((reason: unknown) => {
          if (!signal?.aborted) setError(failure(reason));
        }),
    [fetcher, project],
  );
  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const ready = !!plan && confirmsDelete(typed, project) && !busy;
  const confirm = async () => {
    if (!plan || !ready) return;
    setBusy(true);
    try {
      await deleteProject(fetcher, project, plan.token, typed.trim());
      onDeleted(plan.displayName);
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
      title={`Delete ${plan?.displayName ?? project} permanently?`}
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>Keep it archived</Button>
          <Button variant="danger" disabled={!ready} onClick={confirm}>
            {busy ? "Deleting…" : "Delete permanently"}
          </Button>
        </>
      }
    >
      {plan ? (
        <>
          <ProjectDeletePreview plan={plan} />
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
              label={`Type ${project} to confirm`}
              hint="This removes the project's folder from the board's archive."
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

// ProjectDeletePreview says what deleting a project removes and rewrites.
export function ProjectDeletePreview({ plan }: { plan: ProjectDeletePlan }) {
  return (
    <div className="flex flex-col gap-4">
      <p>
        Deleting {plan.displayName} removes its folder with{" "}
        {plan.tickets === 1 ? "1 ticket" : `${plan.tickets} tickets`}, their
        workstreams, files, reviews and event log. Board files keep no history,
        so this cannot be undone.
      </p>
      {plan.references.length > 0 ? (
        <section aria-label="Tickets in other projects that depend on it">
          <h3 className="mb-1 heading-cut">
            Tickets in other projects that depend on it
          </h3>
          <ul className="flex flex-col gap-1">
            {plan.references.map((ticket) => (
              <li key={`${ticket.project}/${ticket.id}`}>
                <span className="font-semibold tabular-nums">{ticket.id}</span>{" "}
                {ticket.title}{" "}
                <span className="text-ink-muted">
                  ({ticket.project}; depends on {ticket.dependsOn.join(", ")})
                </span>
              </li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-ink-muted">
            Those dependencies are removed, so nothing waits on this project.
          </p>
        </section>
      ) : (
        <p className="text-ink-muted">
          No ticket in another project depends on it.
        </p>
      )}
      <p className="text-xs text-ink-muted">
        Its key {plan.key} stays reserved, so its ticket ids are never given to
        another project.
      </p>
    </div>
  );
}
