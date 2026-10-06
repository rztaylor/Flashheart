import { useCallback, useMemo, useState } from "react";

import { type Archived, fetchArchive } from "../../api/archive";
import { COLUMNS, type TicketRef } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { unarchiveTicket } from "../../api/edit";
import { Aside } from "../../components/Aside";
import { Button } from "../../components/Button";
import { EmptyState } from "../../components/EmptyState";
import { SearchField } from "../../components/Field";
import { Icon } from "../../components/Icon";
import { StatusPill, Tag } from "../../components/Pill";
import { filterArchived } from "../../model/archive";
import { priorityLabel } from "../../model/status";
import { absoluteTime, runningTime } from "../../model/time";
import { useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";
import { ArchiveProjectDialog } from "./ArchiveProjectDialog";
import { DeleteTicketDialog } from "./DeleteTicketDialog";

const columnTitle = (column: string) =>
  COLUMNS.find((item) => item.id === column)?.title ?? column;

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The request failed";

// ArchiveView lists a project's archived tickets (EDIT-8), searchable, with
// Restore (back to the column it left) and Delete permanently, which only
// archived tickets offer. Archive project… archives the whole project
// (PRJ-5).
export function ArchiveView({
  fetcher,
  project,
  revision,
  editing,
  onOpen,
  onBack,
  onProjectArchived,
}: {
  fetcher: AuthenticatedFetch;
  project: string;
  revision: number;
  editing: Editing;
  onOpen(ticket: TicketRef): void;
  onBack(): void;
  // onProjectArchived follows archiving the whole project (PRJ-5).
  onProjectArchived(displayName: string): void;
}) {
  const load = useCallback(
    (signal: AbortSignal) => fetchArchive(fetcher, project, signal),
    [fetcher, project],
  );
  const archive = useResource(load, project, revision);
  const [query, setQuery] = useState("");
  const [deleting, setDeleting] = useState<string | null>(null);
  const [archivingProject, setArchivingProject] = useState(false);
  const items = archive.status === "ready" ? archive.data.tickets : [];
  const shown = useMemo(() => filterArchived(items, query), [items, query]);
  const { notify } = editing;

  const restore = async (item: Archived) => {
    try {
      await unarchiveTicket(fetcher, item.id);
      archive.reload();
      notify({
        text: `Restored ${item.id} to ${columnTitle(item.column)}.`,
        action: { label: "Open", run: () => onOpen({ id: item.id }) },
      });
    } catch (error) {
      notify({ text: `${item.id} was not restored: ${failure(error)}` });
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex flex-wrap items-center gap-3 border-b border-rule px-4 py-2.5 md:px-6">
        <div className="flex w-full max-w-sm">
          <SearchField
            tone="plain"
            label="Search archived tickets"
            placeholder="Search id, title, type"
            value={query}
            onChange={setQuery}
          />
        </div>
        <Button
          variant="quiet"
          className="ml-auto"
          onClick={() => setArchivingProject(true)}
        >
          <Icon name="archive" size={14} />
          Archive project…
        </Button>
        <Button variant="quiet" onClick={onBack}>
          <Icon name="board" size={14} />
          Back to the board
        </Button>
      </div>
      {archive.status === "loading" ? (
        <p className="px-6 py-4 text-sm text-ink-muted">Loading the archive…</p>
      ) : null}
      {archive.status === "error" ? (
        <div className="m-4 flex items-center gap-3 text-sm">
          <p role="alert" className="text-danger">
            The archive could not be loaded: {archive.error}
          </p>
          <Button onClick={archive.reload}>
            <Icon name="refresh" size={14} />
            Try again
          </Button>
        </div>
      ) : null}
      {archive.status === "ready" && items.length === 0 ? (
        <EmptyState title="Nothing archived">
          Archive a ticket from its panel to clear it off the board. It waits
          here until you restore it or delete it for good.
        </EmptyState>
      ) : null}
      {archive.status === "ready" && items.length > 0 && shown.length === 0 ? (
        <EmptyState title="No archived tickets match">
          Clear the search to see all {items.length}.
          <Aside placement="search" className="mt-3" />
        </EmptyState>
      ) : null}
      {shown.length > 0 ? (
        <div className="min-h-0 flex-1 overflow-auto px-4 pt-3 pb-10 md:px-6">
          <table className="w-full border-separate border-spacing-0 overflow-hidden rounded-panel border border-rule bg-card text-sm shadow-card">
            <caption className="sr-only">Archived tickets</caption>
            <thead className="sticky top-0 z-10 bg-column">
              <tr className="[&>th]:border-b [&>th]:border-rule [&>th]:px-3 [&>th]:py-2.5 [&>th]:text-left [&>th]:heading-cut">
                <th scope="col">Ticket</th>
                <th scope="col">Restores to</th>
                <th scope="col">Type</th>
                <th scope="col">Priority</th>
                <th scope="col">Archived</th>
                <th scope="col">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {shown.map((item) => (
                <tr
                  key={item.id}
                  className="hover:bg-well [&>td]:border-b [&>td]:border-rule last:[&>td]:border-b-0"
                >
                  <td className="max-w-[28rem] px-3 py-2">
                    <span className="flex flex-col">
                      <span className="font-semibold text-ink">
                        {item.title || item.id}
                      </span>
                      <span className="flex items-center gap-2 text-2xs font-semibold tracking-[0.02em] tabular-nums text-ink-muted">
                        {item.id}
                        {item.hasReview ? (
                          <span className="flex items-center gap-0.5 font-normal">
                            <Icon name="review" size={11} />
                            review
                          </span>
                        ) : null}
                        {item.attachments > 0 ? (
                          <span className="flex items-center gap-0.5 font-normal">
                            <Icon name="attachment" size={11} />
                            {item.attachments}{" "}
                            {item.attachments === 1 ? "file" : "files"}
                          </span>
                        ) : null}
                      </span>
                    </span>
                  </td>
                  <td className="px-3 py-2 whitespace-nowrap">
                    <StatusPill column={item.column} />
                  </td>
                  <td className="px-3 py-2">
                    {item.type ? <Tag>{item.type}</Tag> : null}
                  </td>
                  <td className="px-3 py-2">
                    {item.priority ? (
                      <Tag strong={item.priority === "high"}>
                        {priorityLabel(item.priority)}
                      </Tag>
                    ) : null}
                  </td>
                  <td
                    className="px-3 py-2 whitespace-nowrap text-ink-muted"
                    title={absoluteTime(item.archived)}
                  >
                    {runningTime(item.archived)}
                  </td>
                  <td className="px-3 py-2">
                    <span className="flex items-center justify-end gap-1">
                      <Button
                        className="h-8 py-0 text-xs"
                        aria-label={`Restore ${item.id}`}
                        onClick={() => void restore(item)}
                      >
                        <Icon name="undo" size={14} />
                        Restore
                      </Button>
                      <Button
                        variant="quiet"
                        className="h-8 py-0 text-xs"
                        aria-label={`Delete permanently ${item.id}`}
                        onClick={() => setDeleting(item.id)}
                      >
                        Delete permanently
                      </Button>
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
      {archivingProject ? (
        <ArchiveProjectDialog
          fetcher={fetcher}
          project={project}
          onClose={() => setArchivingProject(false)}
          onArchived={(displayName) => {
            setArchivingProject(false);
            onProjectArchived(displayName);
          }}
        />
      ) : null}
      {deleting ? (
        <DeleteTicketDialog
          fetcher={fetcher}
          project={project}
          id={deleting}
          onClose={() => setDeleting(null)}
          onDeleted={() => {
            notify({ text: `Deleted ${deleting} permanently.` });
            setDeleting(null);
            archive.reload();
          }}
        />
      ) : null}
    </div>
  );
}
