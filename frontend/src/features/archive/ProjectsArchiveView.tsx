import { useCallback, useState } from "react";

import { fetchArchivedProjects, restoreProject } from "../../api/archive";
import type { ProjectSummary } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { Button } from "../../components/Button";
import { EmptyState } from "../../components/EmptyState";
import { Icon } from "../../components/Icon";
import { KeyBadge } from "../../components/KeyBadge";
import { absoluteTime, runningTime } from "../../model/time";
import { useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";
import { ArchiveProjectDialog } from "./ArchiveProjectDialog";
import { DeleteProjectDialog } from "./DeleteProjectDialog";

const ticketCount = (project: ProjectSummary) => {
  const total = Object.values(project.counts).reduce((a, b) => a + b, 0);
  return `${total} ${total === 1 ? "ticket" : "tickets"}`;
};

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The request failed";

// ProjectsArchiveView is the archive of the All projects scope (PRJ-5): the
// archived projects with Restore and Delete permanently, and the projects
// on the board, each of which can be archived.
export function ProjectsArchiveView({
  fetcher,
  revision,
  projects,
  editing,
  onArchived,
  onOpenProject,
  onBack,
}: {
  fetcher: AuthenticatedFetch;
  revision: number;
  projects: ProjectSummary[];
  editing: Editing;
  onArchived(name: string, displayName: string): void;
  onOpenProject(name: string): void;
  onBack(): void;
}) {
  const load = useCallback(
    (signal: AbortSignal) => fetchArchivedProjects(fetcher, signal),
    [fetcher],
  );
  const archive = useResource(load, "archived-projects", revision);
  const [archiving, setArchiving] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const { notify } = editing;
  const archived = archive.status === "ready" ? archive.data.projects : [];

  const restore = async (name: string, displayName: string) => {
    try {
      await restoreProject(fetcher, name);
      archive.reload();
      notify({
        text: `Restored project ${displayName}.`,
        action: { label: "Open", run: () => onOpenProject(name) },
      });
    } catch (error) {
      notify({ text: `${displayName} was not restored: ${failure(error)}` });
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-auto">
      <div className="flex items-center border-b border-rule px-4 py-2.5 md:px-6">
        <Button variant="quiet" className="ml-auto" onClick={onBack}>
          <Icon name="board" size={14} />
          Back to the board
        </Button>
      </div>
      <section
        aria-labelledby="archived-projects"
        className="px-4 pt-4 md:px-6"
      >
        <h2 id="archived-projects" className="mb-3 text-lg heading-cut">
          Archived projects
        </h2>
        {archive.status === "error" ? (
          <div className="flex items-center gap-3 text-sm">
            <p role="alert" className="text-danger">
              The archive could not be loaded: {archive.error}
            </p>
            <Button onClick={archive.reload}>
              <Icon name="refresh" size={14} />
              Try again
            </Button>
          </div>
        ) : null}
        {archive.status === "ready" && archived.length === 0 ? (
          <EmptyState title="No archived projects">
            Archive a project below to take it and its tickets off the board. It
            waits here until you restore it or delete it for good.
          </EmptyState>
        ) : null}
        {archived.length > 0 ? (
          <table className="w-full border-separate border-spacing-0 overflow-hidden rounded-panel border border-rule bg-card text-sm shadow-card">
            <caption className="sr-only">Archived projects</caption>
            <thead className="bg-column">
              <tr className="[&>th]:border-b [&>th]:border-rule [&>th]:px-3 [&>th]:py-2.5 [&>th]:text-left [&>th]:heading-cut">
                <th scope="col">Project</th>
                <th scope="col">Repositories</th>
                <th scope="col" className="text-right">
                  Tickets
                </th>
                <th scope="col">Archived</th>
                <th scope="col">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {archived.map((project) => (
                <tr
                  key={project.name}
                  className="hover:bg-well [&>td]:border-b [&>td]:border-rule last:[&>td]:border-b-0"
                >
                  <td className="px-3 py-2">
                    <span className="flex items-center gap-2.5">
                      <KeyBadge>{project.key}</KeyBadge>
                      <span className="flex flex-col">
                        <span className="font-semibold">
                          {project.displayName}
                        </span>
                        <span className="text-2xs text-ink-muted">
                          {project.name}
                        </span>
                      </span>
                    </span>
                  </td>
                  <td className="max-w-[20rem] px-3 py-2 text-xs text-ink-muted">
                    {project.repos.map((repo) => (
                      <span
                        key={repo}
                        className="block truncate text-left font-mono [direction:rtl]"
                        title={repo}
                      >
                        <bdi>{repo}</bdi>
                      </span>
                    ))}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {project.tickets}
                  </td>
                  <td
                    className="px-3 py-2 whitespace-nowrap text-ink-muted"
                    title={absoluteTime(project.archived)}
                  >
                    {runningTime(project.archived)}
                  </td>
                  <td className="px-3 py-2">
                    <span className="flex items-center justify-end gap-1">
                      <Button
                        className="h-8 py-0 text-xs"
                        aria-label={`Restore ${project.displayName}`}
                        onClick={() =>
                          void restore(project.name, project.displayName)
                        }
                      >
                        <Icon name="undo" size={14} />
                        Restore
                      </Button>
                      <Button
                        variant="quiet"
                        className="h-8 py-0 text-xs"
                        aria-label={`Delete permanently ${project.displayName}`}
                        onClick={() => setDeleting(project.name)}
                      >
                        Delete permanently
                      </Button>
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : null}
      </section>
      <section
        aria-labelledby="live-projects"
        className="px-4 pt-8 pb-10 md:px-6"
      >
        <h2 id="live-projects" className="mb-1 text-lg heading-cut">
          Projects on the board
        </h2>
        <p className="mb-3 text-sm text-ink-muted">
          Archiving a project takes it and its tickets off the board; you can
          restore it from here.
        </p>
        <ul className="divide-y divide-rule overflow-hidden rounded-panel border border-rule bg-card shadow-card">
          {projects.map((project) => (
            <li
              key={project.name}
              className="flex items-center gap-3 px-3 py-2 text-sm"
            >
              <KeyBadge>{project.key}</KeyBadge>
              <span className="min-w-0 flex-1 truncate font-semibold">
                {project.displayName}
              </span>
              <span className="text-xs text-ink-muted tabular-nums">
                {ticketCount(project)}
              </span>
              <Button
                className="h-8 py-0 text-xs"
                aria-label={`Archive ${project.displayName}`}
                onClick={() => setArchiving(project.name)}
              >
                <Icon name="archive" size={14} />
                Archive…
              </Button>
            </li>
          ))}
        </ul>
      </section>
      {archiving ? (
        <ArchiveProjectDialog
          fetcher={fetcher}
          project={archiving}
          onClose={() => setArchiving(null)}
          onArchived={(displayName) => {
            const name = archiving;
            setArchiving(null);
            archive.reload();
            onArchived(name, displayName);
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteProjectDialog
          fetcher={fetcher}
          project={deleting}
          onClose={() => setDeleting(null)}
          onDeleted={(displayName) => {
            setDeleting(null);
            archive.reload();
            notify({ text: `Deleted project ${displayName} permanently.` });
          }}
        />
      ) : null}
    </div>
  );
}
