import { type ReactNode, useCallback, useEffect, useId, useState } from "react";
import {
  COLUMNS,
  type Column,
  fetchTicket,
  type Reason,
  splitReasons,
  type TicketDetail,
  type TicketRef,
  type WorkstreamBrief,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { conflictOf, type Saved, setCriterion } from "../../api/edit";
import { Button } from "../../components/Button";
import { Select } from "../../components/Field";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import { Markdown } from "../../components/Markdown";
import { SidePanel } from "../../components/SidePanel";
import { StateNote } from "../../components/StateNote";
import { panelId, Tabs, tabId } from "../../components/Tabs";
import type { Line } from "../../model/lines";
import { ticketBody } from "../../model/markdown";
import { absoluteTime, runningTime } from "../../model/time";
import { useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";
import { EditTab } from "./EditTab";
import { RunsTab } from "./RunsTab";

const panelBodyClass =
  "relative min-h-0 flex-1 overflow-y-auto px-6 pt-5 pb-10";

interface CardPanelProps {
  ticket: TicketRef;
  fetcher: AuthenticatedFetch;
  lines: Map<string, Map<string, Line>>;
  workstreamTitle(project: string, slug: string): string;
  // Project keys, so ticket ids in markdown become links (KEY-3).
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  onClose(): void;
  // revision reloads the ticket when the board changes (LIFE-3).
  revision: number;
  // editing enables moves, archiving and the Edit tab; absent when the
  // server is read-only.
  editing?: Editing;
  workstreamsOf(project: string): WorkstreamBrief[];
}

type PanelTab = "ticket" | "edit" | "runs" | "review";

// CardPanel shows one ticket beside the board (CARD-1): the Ticket tab with
// blocked-by (CARD-4), handoff with a warning when a live run has edited
// since it (CARD-5), tickable criteria (CARD-3) and the rendered markdown;
// the Edit tab (EDIT-6); the Runs tab (agent runs linked to the ticket); and
// a Review tab when a review file exists (REV-3). Its header moves and archives the ticket (EDIT-1, EDIT-8).
export function CardPanel({
  ticket,
  fetcher,
  lines,
  workstreamTitle,
  keys,
  onOpen,
  onClose,
  revision,
  editing,
  workstreamsOf,
}: CardPanelProps) {
  const load = useCallback(
    (signal: AbortSignal) => fetchTicket(fetcher, ticket.id, signal),
    [fetcher, ticket.id],
  );
  const resource = useResource(load, ticket.id, revision);
  const [tab, setTab] = useState<PanelTab>("ticket");
  const tabsId = useId();
  const detail = resource.status === "ready" ? resource.data : undefined;
  const editable = !!editing && !!detail?.hash;
  const items: { id: PanelTab; label: string }[] = [
    { id: "ticket", label: "Ticket" },
    ...(editable ? [{ id: "edit" as const, label: "Edit" }] : []),
    {
      id: "runs",
      label: detail?.runs.length
        ? `Runs ${detail.runs.filter((run) => !run.parent).length || detail.runs.length}`
        : "Runs",
    },
    ...(detail?.review ? [{ id: "review" as const, label: "Review" }] : []),
  ];
  const activeTab = items.some((item) => item.id === tab) ? tab : "ticket";

  const saved = (result: Saved | undefined, what: string) => {
    resource.reload();
    if (result && editing)
      editing.notify({
        text: `${what} ${ticket.id}.`,
        details: result.warnings,
      });
  };
  const toggle =
    editable && detail && editing
      ? (index: number, checked: boolean): Promise<boolean> =>
          setCriterion(fetcher, detail.id, detail.hash, index, checked)
            .then(() => {
              resource.reload();
              return true;
            })
            .catch((error: unknown) => {
              resource.reload();
              editing.notify({
                text: conflictOf(error)
                  ? `${detail.id} changed on disk; showing the current criteria.`
                  : `Not saved: ${error instanceof Error ? error.message : "unknown error"}`,
              });
              return false;
            })
      : undefined;
  const body = (current: TicketDetail) => {
    switch (activeTab) {
      case "review":
        return <ReviewTab detail={current} keys={keys} onOpen={onOpen} />;
      case "runs":
        return <RunsTab detail={current} />;
      case "edit":
        return (
          <EditTab
            detail={current}
            fetcher={fetcher}
            workstreams={workstreamsOf(current.project)}
            onSaved={saved}
          />
        );
      default:
        return (
          <TicketTab
            detail={current}
            keys={keys}
            onOpen={onOpen}
            onToggle={toggle}
          />
        );
    }
  };

  return (
    <SidePanel label={`Ticket ${ticket.id}`} onClose={onClose}>
      {resource.status === "loading" ? <PanelSkeleton /> : null}
      {resource.status === "error" ? (
        <div className="p-6">
          <p role="alert" className="text-sm text-danger">
            This ticket could not be loaded: {resource.error}
          </p>
        </div>
      ) : null}
      {detail ? (
        <>
          <PanelHeader
            detail={detail}
            line={lines.get(detail.project)?.get(detail.workstream)}
            workstreamTitle={workstreamTitle(detail.project, detail.workstream)}
            actions={
              editing && editable ? (
                <PanelActions
                  detail={detail}
                  onMove={(to) => void editing.move(detail, to)}
                  onArchive={() => {
                    void editing.archive(detail);
                    onClose();
                  }}
                />
              ) : null
            }
          />
          {items.length > 1 ? (
            <div className="px-6">
              <Tabs
                idPrefix={tabsId}
                label="Ticket views"
                selected={activeTab}
                onSelect={(id) => setTab(id as PanelTab)}
                items={items}
              />
            </div>
          ) : (
            <div aria-hidden="true" className="mx-6 border-b border-rule" />
          )}
          {items.length > 1 ? (
            <div
              role="tabpanel"
              id={panelId(tabsId, activeTab)}
              aria-labelledby={tabId(tabsId, activeTab)}
              className={panelBodyClass}
            >
              {body(detail)}
            </div>
          ) : (
            <div className={panelBodyClass}>{body(detail)}</div>
          )}
        </>
      ) : null}
    </SidePanel>
  );
}

// PanelActions moves the ticket to any column (the menu alternative to
// dragging, EDIT-1) or archives it (EDIT-8).
function PanelActions({
  detail,
  onMove,
  onArchive,
}: {
  detail: TicketDetail;
  onMove(to: Column): void;
  onArchive(): void;
}) {
  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      {/* biome-ignore lint/a11y/noLabelWithoutControl: the Select inside is the control. */}
      <label className="flex items-center gap-2 text-xs text-ink-muted">
        Move to
        <Select
          value={detail.column}
          onChange={(value) => onMove(value as Column)}
        >
          {COLUMNS.map((column) => (
            <option key={column.id} value={column.id}>
              {column.title}
            </option>
          ))}
        </Select>
      </label>
      <Button className="h-8 py-0 text-xs" onClick={onArchive}>
        <Icon name="archive" size={14} />
        Archive
      </Button>
    </div>
  );
}

function PanelHeader({
  detail,
  line,
  workstreamTitle,
  actions,
}: {
  detail: TicketDetail;
  line?: Line;
  workstreamTitle: string;
  actions?: ReactNode;
}) {
  const column =
    COLUMNS.find((item) => item.id === detail.column)?.title ?? detail.column;
  const facts = [
    detail.type,
    detail.priority ? `${detail.priority} priority` : "",
    detail.created ? `created ${detail.created}` : "",
  ].filter(Boolean);
  return (
    <header className="px-6 pt-5 pb-4 pr-14">
      <h2 className="text-xl leading-tight font-semibold tracking-[-0.01em]">
        {detail.title}
      </h2>
      <p className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-muted">
        <span className="font-semibold tracking-[0.02em] tabular-nums text-ink">
          {detail.id}
        </span>
        <span className="font-semibold text-ink">{column}</span>
        {detail.workstream ? (
          <span className="flex items-center gap-1.5">
            <LineBullet line={line} size="sm" />
            {workstreamTitle}
          </span>
        ) : null}
        {facts.map((fact) => (
          <span key={fact}>{fact}</span>
        ))}
        {detail.branch ? (
          <span className="flex items-center gap-1 font-mono">
            <Icon name="branch" size={12} />
            {detail.branch}
          </span>
        ) : null}
        <span title={absoluteTime(detail.modified)}>
          {changedLabel(detail.modified)}
        </span>
      </p>
      {actions}
    </header>
  );
}

// ReasonList explains why a ticket cannot start (CARD-4). Real blockers are
// marked with the diamond; waits on an earlier station of the line are not.
function ReasonList({
  id,
  title,
  marked,
  reasons,
  onOpen,
}: {
  id: string;
  title: string;
  marked?: boolean;
  reasons: Reason[];
  onOpen(ticket: TicketRef): void;
}) {
  if (reasons.length === 0) return null;
  return (
    <section aria-labelledby={id}>
      <h3
        id={id}
        className={`mb-2 flex items-center gap-1.5 border-t-2 pt-1.5 text-sm station-sign ${marked ? "border-rule-strong text-ink" : "border-rule text-ink-muted"}`}
      >
        {marked ? <Icon name="diamond" size={14} /> : null}
        {title}
      </h3>
      <ul className="flex flex-col gap-1.5">
        {reasons.map((reason) => (
          <li
            key={reason.text}
            className="flex items-start justify-between gap-3 text-sm"
          >
            <span
              className={marked ? "font-medium text-ink" : "text-ink-muted"}
            >
              {reason.text}
            </span>
            {reason.ticket && !reason.missing ? (
              <button
                type="button"
                onClick={() =>
                  reason.ticket && onOpen({ id: reason.ticket.id })
                }
                aria-label={`Open ${reason.ticket.id}`}
                className="shrink-0 text-xs text-ink-muted underline decoration-ink-faint hover:text-ink"
              >
                Open
              </button>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
}

// Criterion is a tickable acceptance criterion (CARD-3). It shows the new
// state at once and follows the file when it reloads.
function Criterion({
  text,
  done,
  onToggle,
}: {
  text: string;
  done: boolean;
  // onToggle resolves false when the change was not saved.
  onToggle(checked: boolean): Promise<boolean>;
}) {
  const [checked, setChecked] = useState(done);
  useEffect(() => setChecked(done), [done]);
  return (
    <label className="flex cursor-pointer items-start gap-2 text-sm">
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => {
          const next = event.target.checked;
          setChecked(next);
          void onToggle(next).then((ok) => {
            if (!ok) setChecked(done);
          });
        }}
        className="mt-[0.2em] size-3.5 shrink-0 accent-[var(--fh-ink)]"
      />
      <span className={checked ? "text-ink-muted" : "text-ink"}>{text}</span>
    </label>
  );
}

function TicketTab({
  detail,
  keys,
  onOpen,
  onToggle,
}: {
  detail: TicketDetail;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  // onToggle ticks or unticks a criterion (CARD-3); absent when read-only.
  onToggle?(index: number, checked: boolean): Promise<boolean>;
}) {
  const done = detail.criteriaItems.filter((item) => item.done).length;
  // Criteria are addressed by position in the file, and their text may
  // repeat, so the key is position and text.
  const criteria = detail.criteriaItems.map((item, index) => ({
    ...item,
    index,
    key: `${index}:${item.text}`,
  }));
  const { blockers, waits } = splitReasons(detail.blockedBy);
  return (
    <div className="flex flex-col gap-6">
      {detail.needsRepair.length > 0 ? (
        <section
          aria-labelledby="repair-heading"
          className="relative overflow-hidden rounded-card border border-dashed border-ink-muted p-3 pt-4"
        >
          <span
            aria-hidden="true"
            className="hatched-strong absolute inset-x-0 top-0 h-1.5"
          />
          <h3 id="repair-heading" className="mb-1.5 text-sm station-sign">
            Needs repair
          </h3>
          {detail.needsRepair.map((problem) => (
            <StateNote key={problem} kind="repair">
              {problem}
            </StateNote>
          ))}
          <p className="mt-2 text-xs text-ink-muted">
            Flashheart never rewrites a broken file. Fix it in your editor; the
            board updates when it parses.
          </p>
          {detail.frontmatter ? (
            <pre className="mt-2 overflow-x-auto rounded-card bg-card p-2 font-mono text-xs">
              {detail.frontmatter}
            </pre>
          ) : null}
        </section>
      ) : null}

      <ReasonList
        id="blocked-heading"
        title="Blocked by"
        marked
        reasons={blockers}
        onOpen={onOpen}
      />
      <ReasonList
        id="waits-heading"
        title="Waits for"
        reasons={waits}
        onOpen={onOpen}
      />

      {detail.handoff ? (
        <section
          aria-labelledby="handoff-heading"
          className="rounded-panel bg-well p-4"
        >
          <h3 id="handoff-heading" className="mb-2 text-sm station-sign">
            Handoff
          </h3>
          {staleHandoff(detail) ? (
            <div className="mb-3">
              <StateNote kind="warning">{staleHandoff(detail)}</StateNote>
            </div>
          ) : null}
          {detail.handoff.next.length > 0 ? (
            <div className="mb-3">
              <p className="text-xs text-ink-muted">Next</p>
              <ul className="mt-1 flex flex-col gap-1">
                {detail.handoff.next.map((item) => (
                  <li
                    key={item}
                    className="flex items-start gap-1.5 text-md font-medium"
                  >
                    <Icon name="next" size={14} className="mt-[0.3em]" />
                    {item}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <details className="group text-sm">
            <summary className="flex cursor-pointer list-none items-center gap-1 text-xs text-ink-muted hover:text-ink [&::-webkit-details-marker]:hidden">
              <Icon
                name="chevronDown"
                size={12}
                className="-rotate-90 transition-transform group-open:rotate-0"
              />
              Full handoff
            </summary>
            <div className="mt-2">
              <Markdown
                context={{ project: detail.project, ticket: detail.id }}
                keys={keys}
                onOpenTicket={onOpen}
              >
                {detail.handoff.markdown}
              </Markdown>
            </div>
          </details>
        </section>
      ) : null}

      {detail.criteriaItems.length > 0 ? (
        <section aria-labelledby="criteria-heading">
          <h3
            id="criteria-heading"
            className="mb-2 flex items-baseline justify-between border-t-2 border-rule-strong pt-1.5 text-sm station-sign"
          >
            Acceptance criteria
            <span className="text-xs font-normal text-ink-muted">
              {done} of {detail.criteriaItems.length}
            </span>
          </h3>
          <ul className="flex flex-col gap-1">
            {criteria.map((item) =>
              onToggle ? (
                <li key={item.key}>
                  <Criterion
                    text={item.text}
                    done={item.done}
                    onToggle={(checked) => onToggle(item.index, checked)}
                  />
                </li>
              ) : (
                <li key={item.key} className="flex items-start gap-2 text-sm">
                  <span
                    aria-hidden="true"
                    className={`mt-[0.2em] grid size-3.5 shrink-0 place-items-center rounded-[2px] border ${item.done ? "border-ink bg-ink text-ground" : "border-ink-muted"}`}
                  >
                    {item.done ? <Icon name="criteria" size={10} /> : null}
                  </span>
                  <span className={item.done ? "text-ink-muted" : "text-ink"}>
                    <span className="sr-only">
                      {item.done ? "Done: " : "Not done: "}
                    </span>
                    {item.text}
                  </span>
                </li>
              ),
            )}
          </ul>
        </section>
      ) : null}

      {detail.warnings.length > 0 ? (
        <section aria-label="Format warnings" className="flex flex-col gap-1">
          {detail.warnings.map((warning) => (
            <StateNote key={warning} kind="warning">
              {warning}
            </StateNote>
          ))}
        </section>
      ) : null}

      <section aria-label="Ticket text">
        <Markdown
          context={{ project: detail.project, ticket: detail.id }}
          keys={keys}
          onOpenTicket={onOpen}
        >
          {ticketBody(detail.body)}
        </Markdown>
      </section>
    </div>
  );
}

function ReviewTab({
  detail,
  keys,
  onOpen,
}: {
  detail: TicketDetail;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
}) {
  return (
    <div className="flex flex-col gap-6">
      {detail.attachmentFiles.length > 0 ? (
        <section aria-labelledby="attachments-heading">
          <h3
            id="attachments-heading"
            className="mb-2 border-t-2 border-rule-strong pt-1.5 text-sm station-sign"
          >
            Attachments
          </h3>
          <ul className="grid grid-cols-2 gap-3">
            {detail.attachmentFiles.map((attachment) => (
              <li key={attachment.file}>
                <a
                  href={attachment.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="group block"
                >
                  {attachment.kind === "screenshot" ||
                  /\.(png|jpe?g|gif|webp)$/i.test(attachment.file) ? (
                    <img
                      src={attachment.url}
                      alt={attachment.caption || attachment.file}
                      loading="lazy"
                      className="aspect-video w-full rounded-card border border-rule bg-well object-cover group-hover:border-ink-muted"
                    />
                  ) : (
                    <span className="flex aspect-video items-center justify-center rounded-card border border-rule bg-well text-ink-muted">
                      <Icon name="attachment" size={20} />
                    </span>
                  )}
                  <span className="mt-1 block text-xs text-ink">
                    {attachment.caption || attachment.file}
                  </span>
                  <span className="block truncate font-mono text-2xs text-ink-faint">
                    {attachment.file}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      {detail.review ? (
        <Markdown
          context={{ project: detail.project, ticket: detail.id }}
          keys={keys}
          onOpenTicket={onOpen}
        >
          {detail.review.markdown}
        </Markdown>
      ) : null}
    </div>
  );
}

function PanelSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col gap-3 p-6">
      <div className="h-3 w-24 animate-pulse rounded-card bg-well" />
      <div className="h-6 w-3/4 animate-pulse rounded-card bg-well" />
      <div className="h-3 w-1/2 animate-pulse rounded-card bg-well" />
      <div className="mt-6 h-24 animate-pulse rounded-card bg-well" />
    </div>
  );
}

function changedLabel(modified: string): string {
  const age = runningTime(modified);
  if (!age) return "";
  return age === "now" ? "changed just now" : `changed ${age} ago`;
}

// staleHandoff warns when the run working on the ticket has edited files
// since its last checkpoint, so the handoff may be behind (CARD-5).
function staleHandoff(detail: TicketDetail): string {
  const run = detail.runs.find((item) => !item.parent && item.dirty);
  if (!run) return "";
  const edits = `${run.edits} ${run.edits === 1 ? "edit" : "edits"}`;
  return run.state === "ended"
    ? `Run ${run.short} ended after ${edits} without updating this handoff.`
    : `Run ${run.short} has made ${edits} since this handoff.`;
}
