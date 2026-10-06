import { useCallback } from "react";

import {
  deleteProject,
  fetchProjectDeletePlan,
  type ProjectDeletePlan,
} from "../../api/archive";
import type { AuthenticatedFetch } from "../../api/client";
import { TypedDeleteDialog } from "./TypedDeleteDialog";

// DeleteProjectDialog deletes an archived project for good (PRJ-5), after
// listing what goes and the other projects' tickets whose dependencies are
// removed, and the typed project name.
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
  const load = useCallback(
    (signal?: AbortSignal) => fetchProjectDeletePlan(fetcher, project, signal),
    [fetcher, project],
  );
  return (
    <TypedDeleteDialog<ProjectDeletePlan>
      title={(plan) => `Delete ${plan?.displayName ?? project} permanently?`}
      word={project}
      hint="This removes the project's folder from the board's archive."
      load={load}
      remove={(plan, typed) =>
        deleteProject(fetcher, project, plan.token, typed)
      }
      onClose={onClose}
      onDeleted={(plan) => onDeleted(plan.displayName)}
    >
      {(plan) => <ProjectDeletePreview plan={plan} />}
    </TypedDeleteDialog>
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
