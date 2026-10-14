import { useCallback, useMemo, useState } from "react";

import { type Archived, fetchArchive } from "../../api/archive";
import { STATUSES, statusWord, type TicketRef } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { unarchiveTicket } from "../../api/edit";
import { Aside } from "../../components/Aside";
import { Button } from "../../components/Button";
import { EmptyState } from "../../components/EmptyState";
import { SearchField } from "../../components/Field";
import { Icon } from "../../components/Icon";
import { LoadError } from "../../components/LoadError";
import { StatusPill } from "../../components/Pill";
import { StaleNote } from "../../components/StaleNote";
import { HeadCell, Row, SkeletonTable, Table } from "../../components/Table";
import { TicketLink } from "../../components/TicketLink";
import { PriorityTag, TypeTag } from "../../components/TicketMarks";
import { filterArchived } from "../../model/archive";
import { messageOf } from "../../model/errors";
import { absoluteTime, runningTime } from "../../model/time";
import { useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";
import { ArchiveProjectDialog } from "./ArchiveProjectDialog";
import { DeleteTicketDialog } from "./DeleteTicketDialog";

const statusTitle = (item: Archived) =>
  STATUSES.find((status) => status.id === statusWord(item))?.title ??
  item.column;

// ArchiveSkeleton holds an archive's place while it loads (ui-layout.md
// §8): its table, in the view's gutter unless className places it.
export function ArchiveSkeleton({
  className = "px-4 pt-3 md:px-6",
}: {
  className?: string;
}) {
  return (
    <div data-skeleton="archive" className={className}>
      <SkeletonTable rows={4} />
    </div>
  );
}

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
        text: `Restored ${item.id} to ${statusTitle(item)}.`,
        action: { label: "Open", run: () => onOpen({ id: item.id }) },
      });
    } catch (error) {
      notify({
        text: `${item.id} was not restored: ${messageOf(error, "The request failed")}`,
      });
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
      {archive.status === "loading" ? <ArchiveSkeleton /> : null}
      {archive.status === "error" ? (
        <LoadError
          what="The archive could not be loaded"
          error={archive.error}
          onRetry={archive.reload}
          className="m-4"
        />
      ) : null}
      {archive.status === "ready" && archive.error ? (
        <StaleNote error={archive.error} className="px-4 pt-3 md:px-6" />
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
          <Table
            caption="Archived tickets"
            head={
              <>
                <HeadCell>Ticket</HeadCell>
                <HeadCell>Restores to</HeadCell>
                <HeadCell>Type</HeadCell>
                <HeadCell>Priority</HeadCell>
                <HeadCell>Archived</HeadCell>
                <HeadCell>
                  <span className="sr-only">Actions</span>
                </HeadCell>
              </>
            }
          >
            {shown.map((item) => (
              <Row key={item.id}>
                <td className="max-w-[28rem] px-3 py-2">
                  <span className="flex flex-col">
                    <span className="font-semibold text-ink">
                      {item.title || item.id}
                    </span>
                    <span className="flex items-center gap-2 text-2xs font-semibold tracking-[0.02em] tabular-nums text-ink-muted">
                      <TicketLink id={item.id} className="hover:text-ink" />
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
                  <StatusPill column={item.column} rejected={item.rejected} />
                </td>
                <td className="px-3 py-2">
                  {item.type ? <TypeTag type={item.type} /> : null}
                </td>
                <td className="px-3 py-2">
                  {item.priority ? (
                    <PriorityTag priority={item.priority} />
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
                      variant="danger-quiet"
                      className="h-8 py-0 text-xs"
                      aria-label={`Delete permanently ${item.id}`}
                      onClick={() => setDeleting(item.id)}
                    >
                      Delete permanently
                    </Button>
                  </span>
                </td>
              </Row>
            ))}
          </Table>
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
