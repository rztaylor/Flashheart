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
import { Aside } from "../../components/Aside";
import { Button } from "../../components/Button";
import { Select } from "../../components/Field";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import { Markdown } from "../../components/Markdown";
import { BlockerPill, Pill, StatusPill, Tag } from "../../components/Pill";
import { QuestionCard } from "../../components/QuestionCard";
import { SidePanel } from "../../components/SidePanel";
import { StateNote } from "../../components/StateNote";
import { panelId, Tabs, tabId } from "../../components/Tabs";
import type { Line } from "../../model/lines";
import { ticketBody } from "../../model/markdown";
import type { Step } from "../../model/order";
import { counted, sessionsOf, shortRun } from "../../model/runs";
import { priorityLabel } from "../../model/status";
import { absoluteTime, runningTime } from "../../model/time";
import { useNow } from "../../state/useNow";
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
  // onStep moves the ticket within its column (EDIT-9); absent when the
  // board does not show it.
  onStep?: (step: Step) => void;
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
  onStep,
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
        ? `Runs ${sessionsOf(detail.runs).length}`
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
  // answer sends an answer to an agent's question (CARD-6).
  const answer =
    editing && detail && editable
      ? (id: string, text: string) =>
          editing.answer(id, text).finally(() => resource.reload())
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
            onAnswer={answer}
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
                  onStep={onStep}
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
              // The ARIA tabs pattern makes the panel a tab stop, so a
              // scrolling panel with no controls (Runs) is still reachable.
              // biome-ignore lint/a11y/noNoninteractiveTabindex: see above.
              tabIndex={0}
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

const steps: { value: Step; short: string; label: string }[] = [
  { value: "top", short: "Top", label: "Top of its column" },
  { value: "up", short: "Up", label: "Up one place" },
  { value: "down", short: "Down", label: "Down one place" },
  {
    value: "bottom",
    short: "Bottom",
    label: "Bottom of its column",
  },
];

// PanelActions moves the ticket to any column or to a place in its column
// (the menu alternatives to dragging, EDIT-1, EDIT-9) or archives it
// (EDIT-8).
function PanelActions({
  detail,
  onMove,
  onStep,
  onArchive,
}: {
  detail: TicketDetail;
  onMove(to: Column): void;
  onStep?: (step: Step) => void;
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
      {onStep ? (
        <fieldset className="flex items-center gap-1 text-xs text-ink-muted">
          <legend className="sr-only">Position in its column</legend>
          <span aria-hidden="true" className="mr-1">
            Position
          </span>
          {steps.map((step) => (
            <Button
              key={step.value}
              variant="quiet"
              className="h-8 px-2 py-0 text-xs"
              aria-label={step.label}
              title={step.label}
              onClick={() => onStep(step.value)}
            >
              {step.short}
            </Button>
          ))}
        </fieldset>
      ) : null}
      <Button className="h-8 py-0 text-xs" onClick={onArchive}>
        <Icon name="archive" size={14} />
        Archive
      </Button>
    </div>
  );
}

// PanelHeader: the id and project, the title, a pill row (column, blocker,
// priority, type), a meta row and the actions (ui-layout.md §3).
export function PanelHeader({
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
  const priority = detail.priority
    ? `${priorityLabel(detail.priority)} priority`
    : "";
  return (
    <header className="px-6 pt-5 pb-4 pr-16">
      <p className="flex items-center gap-2 text-sm text-ink-muted">
        <span className="font-semibold tracking-[0.02em] tabular-nums text-ink">
          {detail.id}
        </span>
        <span>{detail.project}</span>
      </p>
      <h2 className="mt-1 text-2xl leading-tight display-cut">
        {detail.title}
      </h2>
      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <StatusPill column={detail.column} />
        {/* Only a real blocker, never a wait on the line's own order. */}
        {splitReasons(detail.blockedBy).blockers.length > 0 ? (
          <Pill tone="blocked">Blocked</Pill>
        ) : null}
        {priority ? (
          <Tag strong={detail.priority === "high"}>{priority}</Tag>
        ) : null}
        {detail.type ? <Tag>{detail.type}</Tag> : null}
      </div>
      <p className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-muted">
        {detail.workstream ? (
          <span className="flex items-center gap-1.5 font-medium text-ink">
            <LineBullet line={line} size="sm" />
            {workstreamTitle}
          </span>
        ) : null}
        {detail.created ? <span>created {detail.created}</span> : null}
        {detail.branch ? (
          <span className="flex min-w-0 items-center gap-1 font-mono">
            <Icon name="branch" size={12} />
            <span className="truncate">{detail.branch}</span>
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
// blocked pills; waits on an earlier station of the line are quiet words.
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
        className={`mb-2 flex items-center gap-1.5 text-md heading-cut ${marked ? "text-ink" : "text-ink-muted"}`}
      >
        {marked ? (
          <Icon
            name="diamond"
            size={15}
            className="text-(--fh-state-blocked-ink)"
          />
        ) : null}
        {title}
      </h3>
      <ul className="flex flex-col gap-1.5">
        {reasons.map((reason) => (
          <li
            key={reason.text}
            className="flex items-start justify-between gap-3 text-sm"
          >
            {marked ? (
              <BlockerPill>{reason.text}</BlockerPill>
            ) : (
              <span className="text-ink-muted">{reason.text}</span>
            )}
            {reason.ticket && !reason.missing ? (
              <button
                type="button"
                onClick={() =>
                  reason.ticket && onOpen({ id: reason.ticket.id })
                }
                aria-label={`Open ${reason.ticket.id}`}
                className="mt-1 shrink-0 text-xs font-medium text-ink-muted underline decoration-ink-faint hover:text-ink"
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
    <label className="flex cursor-pointer items-start gap-2.5 px-3 py-2 text-sm">
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
        className="mt-[0.15em] size-4 shrink-0 accent-[var(--fh-select)]"
      />
      <span className={checked ? "text-ink-muted" : "text-ink"}>{text}</span>
    </label>
  );
}

// TicketTab, in the order of ui-layout.md §3: needs repair, questions for
// the user, the handoff, blockers and waits, criteria, format warnings and
// the ticket's own markdown.
export function TicketTab({
  detail,
  keys,
  onOpen,
  onToggle,
  onAnswer,
}: {
  detail: TicketDetail;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  // onToggle ticks or unticks a criterion (CARD-3); absent when read-only.
  onToggle?(index: number, checked: boolean): Promise<boolean>;
  // onAnswer answers an agent's question (CARD-6); absent when read-only.
  onAnswer?(question: string, answer: string): Promise<string | undefined>;
}) {
  const now = useNow();
  const done = detail.criteriaItems.filter((item) => item.done).length;
  // Criteria are addressed by position in the file, and their text may
  // repeat, so the key is position and text.
  const criteria = detail.criteriaItems.map((item, index) => ({
    ...item,
    index,
    key: `${index}:${item.text}`,
  }));
  const { blockers, waits } = splitReasons(detail.blockedBy);
  const stale = staleHandoff(detail);
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
          <h3 id="repair-heading" className="mb-1.5 text-md heading-cut">
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
            <pre className="mt-2 overflow-x-auto rounded-card bg-well p-2 font-mono text-xs">
              {detail.frontmatter}
            </pre>
          ) : null}
        </section>
      ) : null}

      {detail.questions.length > 0 ? (
        <section
          aria-labelledby="questions-heading"
          className="flex flex-col gap-3 rounded-card border border-callout-question-edge/30 bg-callout-question p-4"
        >
          <h3
            id="questions-heading"
            className="flex items-center gap-2 text-md heading-cut"
          >
            <span className="grid size-5 place-items-center rounded-full bg-attention text-on-attention">
              <Icon name="warning" size={12} />
            </span>
            {detail.questions.length === 1
              ? "Question for you"
              : `${detail.questions.length} questions for you`}
          </h3>
          {detail.questions.map((question) => (
            <QuestionCard
              key={question.id}
              question={question}
              asker={shortRun(question.run)}
              now={now}
              onAnswer={
                onAnswer ? (text) => onAnswer(question.id, text) : undefined
              }
            />
          ))}
        </section>
      ) : null}

      {detail.handoff ? (
        <section
          aria-labelledby="handoff-heading"
          className="rounded-card border border-callout-handoff-edge/30 bg-callout-handoff p-4"
        >
          <h3
            id="handoff-heading"
            className="mb-2 flex items-center gap-2 text-md heading-cut"
          >
            <Icon name="next" size={16} className="text-callout-handoff-edge" />
            Handoff
          </h3>
          {stale ? (
            <div className="mb-3">
              <StateNote kind="warning">{stale}</StateNote>
            </div>
          ) : null}
          {detail.handoff.next.length > 0 ? (
            <div className="mb-3">
              <p className="text-xs text-ink-muted">Next</p>
              <ul className="mt-1 flex flex-col gap-1">
                {detail.handoff.next.map((item) => (
                  <li
                    key={item}
                    className="text-lg leading-snug font-semibold text-ink"
                  >
                    {item}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <details className="group text-sm">
            <summary className="flex cursor-pointer list-none items-center gap-1 text-xs font-medium text-ink-muted hover:text-ink [&::-webkit-details-marker]:hidden">
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
          {!stale && detail.handoff.next.length > 0 ? (
            <Aside placement="handoff" className="mt-3" />
          ) : null}
        </section>
      ) : detail.column === "in-progress" ? (
        <Aside placement="handoff-missing" />
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

      {detail.criteriaItems.length > 0 ? (
        <section aria-labelledby="criteria-heading">
          <h3
            id="criteria-heading"
            className="mb-2 flex items-baseline justify-between text-md heading-cut"
          >
            Acceptance criteria
            <span className="text-sm font-normal text-ink-muted">
              {done} of {detail.criteriaItems.length}
            </span>
          </h3>
          <ul className="divide-y divide-rule overflow-hidden rounded-card border border-rule bg-card">
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
                <li
                  key={item.key}
                  className="flex items-start gap-2.5 px-3 py-2 text-sm"
                >
                  <span
                    aria-hidden="true"
                    className={`mt-[0.15em] grid size-4 shrink-0 place-items-center rounded-mark border ${item.done ? "border-select bg-select text-card" : "border-ink-muted"}`}
                  >
                    {item.done ? <Icon name="check" size={11} /> : null}
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
          <h3 id="attachments-heading" className="mb-2 text-md heading-cut">
            Attachments
          </h3>
          <ul className="grid grid-cols-2 gap-3">
            {detail.attachmentFiles.map((attachment) => (
              <li key={attachment.file}>
                <a
                  href={attachment.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="group block rounded-card"
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
                  <span className="mt-1.5 block text-xs font-medium text-ink">
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
  const edits = counted(run.edits, "edit");
  return run.state === "ended"
    ? `Run ${run.short} ended after ${edits} without updating this handoff.`
    : `Run ${run.short} has made ${edits} since this handoff.`;
}
