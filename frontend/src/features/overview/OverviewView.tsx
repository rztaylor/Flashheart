import { type ReactNode, useCallback, useId, useMemo, useState } from "react";

import {
  type Card,
  fetchAllBoard,
  fetchProjectBoard,
  type ProjectSummary,
  type TicketRef,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { fetchRuns, type Run } from "../../api/runs";
import { Aside } from "../../components/Aside";
import { Button } from "../../components/Button";
import { Icon, type IconName } from "../../components/Icon";
import { LoadError } from "../../components/LoadError";
import { attentionClass } from "../../components/NeedsYou";
import { stateToneClass, stateToneIcon } from "../../components/Pill";
import { QuestionCard } from "../../components/QuestionCard";
import { RunStateLabel, RunStateMark } from "../../components/RunState";
import { StaleNote } from "../../components/StaleNote";
import { TicketLink } from "../../components/TicketLink";
import {
  focusHeading,
  focusItem,
  focusList,
  useKeepFocus,
} from "../../components/useKeepFocus";
import {
  collapsible,
  type DecisionItem,
  type Overview,
  overview,
  type ProgressItem,
  pipelineStages,
  type ReviewItem,
  type RiskItem,
  SECTIONS,
  type Section,
  type SectionId,
  sectionOpen,
} from "../../model/overview";
import { agentName, agentNote, shortID } from "../../model/runs";
import { absoluteTime, durationWords } from "../../model/time";
import { useNow } from "../../state/useNow";
import { useResource } from "../../state/useResource";
import { PipelineHeadline } from "./PipelineHeadline";

// column fills Main with the gutter Board, Workstreams and Table share.
const column = "flex flex-col gap-4 px-4 pt-1 md:px-6";

type Answer = (question: string, answer: string) => Promise<string | undefined>;

interface OverviewViewProps {
  fetcher: AuthenticatedFetch;
  // project limits the view to one project; "" shows every project.
  project: string;
  projects: ProjectSummary[];
  revision: number;
  // boardHref is the Board for the same scope, where the pipeline leads.
  boardHref: string;
  onOpen(ticket: TicketRef): void;
  // onReview opens a ticket on its Review tab.
  onReview(ticket: TicketRef): void;
  // onAnswer answers an agent's question (CARD-6); absent when read-only.
  onAnswer?: Answer;
  // onCreateTicket opens New ticket for a project; absent when read-only.
  onCreateTicket?(project: string): void;
}

// OverviewView is the project manager's view (VIEW-3, D32): what needs a
// decision, what is ready for review, what is at risk and what is in
// progress, about tickets, with agent sessions as evidence, under the
// scope's pipeline (FH-63). It loads the scope's tickets and runs;
// run detail stays on the card's Runs tab.
export function OverviewView({
  fetcher,
  project,
  projects,
  revision,
  boardHref,
  onOpen,
  onReview,
  onAnswer,
  onCreateTicket,
}: OverviewViewProps) {
  const now = useNow();
  const load = useCallback(
    async (signal: AbortSignal) => {
      const [board, runs] = await Promise.all([
        project
          ? fetchProjectBoard(fetcher, project, false, signal)
          : fetchAllBoard(fetcher, false, signal),
        fetchRuns(fetcher, project, false, signal),
      ]);
      return { cards: board.cards, runs: runs.runs };
    },
    [fetcher, project],
  );
  const resource = useResource(load, project, revision);
  const data = useMemo(
    () =>
      resource.data
        ? overview(resource.data.cards, resource.data.runs)
        : undefined,
    [resource.data],
  );
  const projectNames = useMemo(
    () => new Map(projects.map((item) => [item.name, item.displayName])),
    [projects],
  );
  // A focused row keeps focus when a reload moves it to another section,
  // and hands it on when it goes (FH-84).
  const keepFocus = useKeepFocus();

  if (resource.status === "loading") return <OverviewSkeleton />;
  if (resource.status === "error" || !data) {
    return (
      <LoadError
        what="The overview could not be loaded"
        error={resource.error ?? ""}
        onRetry={resource.reload}
        className="m-4"
      />
    );
  }
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div ref={keepFocus} className={`${column} pb-12`}>
        <PipelineHeadline
          stages={pipelineStages(projects, project)}
          boardHref={boardHref}
        />
        {resource.error ? <StaleNote error={resource.error} /> : null}
        <OverviewSections
          data={data}
          now={now}
          projectNames={projectNames}
          onOpen={onOpen}
          onReview={onReview}
          onAnswer={onAnswer}
          onCreateTicket={onCreateTicket}
          nobodyNeedsYou={projects.every((item) => item.runs.needsYou === 0)}
          noLiveRuns={resource.data.runs.every((run) => run.state === "ended")}
        />
      </div>
    </div>
  );
}

interface SectionsProps {
  data: Overview;
  now: Date;
  projectNames: Map<string, string>;
  onOpen(ticket: TicketRef): void;
  onReview(ticket: TicketRef): void;
  onAnswer?: Answer;
  onCreateTicket?(project: string): void;
  // nobodyNeedsYou and noLiveRuns earn the quiet sections their remark.
  nobodyNeedsYou?: boolean;
  noLiveRuns?: boolean;
}

// OverviewSections draws the sections in their fixed order, one calm column
// of rounded section cards.
export function OverviewSections({
  data,
  now,
  projectNames,
  onOpen,
  onReview,
  onAnswer,
  onCreateTicket,
  nobodyNeedsYou,
  noLiveRuns,
}: SectionsProps) {
  const rows: Record<SectionId, ReactNode[]> = {
    decision: data.decision.map((item) => (
      <DecisionRow
        key={item.key}
        item={item}
        now={now}
        projectNames={projectNames}
        onOpen={onOpen}
        onAnswer={onAnswer}
      />
    )),
    review: data.review.map((item) => (
      <ReviewRow
        key={item.card.id}
        item={item}
        now={now}
        onOpen={onOpen}
        onReview={onReview}
      />
    )),
    risk: data.risk.map((item) => (
      <RiskRow key={item.card.id} item={item} now={now} onOpen={onOpen} />
    )),
    progress: data.progress.map((item) => (
      <ProgressRow key={item.card.id} item={item} now={now} onOpen={onOpen} />
    )),
    unticketed: data.unticketed.map((run) => (
      <UnticketedRow
        key={run.id}
        run={run}
        now={now}
        projectNames={projectNames}
        onCreateTicket={onCreateTicket}
      />
    )),
    upNext: data.upNext.map((card) => (
      <UpNextRow key={card.id} card={card} onOpen={onOpen} />
    )),
  };
  const counts: Record<SectionId, number> = {
    decision: data.decision.length,
    review: data.review.length,
    risk: data.risk.length,
    progress: data.progress.length,
    unticketed: data.unticketed.length,
    upNext: data.upNextTotal,
  };
  const asides: Partial<Record<SectionId, ReactNode>> = {
    decision:
      counts.decision === 0 && nobodyNeedsYou ? (
        <Aside
          placement="needs-you-clear"
          className="border-t border-rule px-4 py-2.5"
        />
      ) : null,
    progress:
      counts.progress === 0 && noLiveRuns ? (
        <Aside
          placement="agents-none"
          className="border-t border-rule px-4 py-2.5"
        />
      ) : null,
  };
  return (
    <>
      {SECTIONS.map((section) => (
        <OverviewSection
          key={section.id}
          section={section}
          count={counts[section.id]}
          aside={asides[section.id]}
        >
          {rows[section.id]}
          {section.id === "upNext" && data.upNextTotal > data.upNext.length ? (
            <li className="px-4 py-2.5 text-xs text-ink-muted">
              {data.upNextTotal - data.upNext.length} more in Up next on the
              board.
            </li>
          ) : null}
        </OverviewSection>
      ))}
    </>
  );
}

// The section's mark: a disc in its role's colour (Needs you in attention,
// review and in progress in their status colours), or an ink icon.
const marks: Record<SectionId, { icon?: IconName; disc: string }> = {
  decision: { disc: attentionClass },
  review: { icon: stateToneIcon.review, disc: stateToneClass.review },
  risk: { icon: "warning", disc: "bg-card text-ink shadow-card" },
  progress: { icon: stateToneIcon.progress, disc: stateToneClass.progress },
  unticketed: { icon: "branch", disc: "bg-card text-ink shadow-card" },
  upNext: { icon: stateToneIcon.neutral, disc: stateToneClass.neutral },
};

function SectionMark({ id, count }: { id: SectionId; count: number }) {
  const mark = marks[id];
  // Attention is for something that needs you: an empty decision section
  // takes the plain disc.
  const disc =
    id === "decision" && count === 0
      ? "bg-card text-ink shadow-card"
      : mark.disc;
  return (
    <span
      aria-hidden="true"
      className={`grid size-7 shrink-0 place-items-center rounded-full ${disc}`}
    >
      {mark.icon ? (
        <Icon name={mark.icon} size={15} />
      ) : (
        <RunStateMark state="needs-you" size={11} still />
      )}
    </span>
  );
}

// OverviewSection is one rounded section card: a head with its mark, title
// and count, then its rows. Empty, the head carries its calm sentence. At
// risk, In progress and the last two tiles open and close.
function OverviewSection({
  section,
  count,
  aside,
  children,
}: {
  section: Section;
  count: number;
  aside?: ReactNode;
  children: ReactNode;
}) {
  const id = useId();
  const titleId = `${id}-title`;
  const bodyId = `${id}-body`;
  const [open, setOpen] = useState(() => sectionOpen(section.id, count));
  const toggles = collapsible(section.id) && count > 0;
  const shown = count > 0 && (open || !toggles);
  const head = (
    <>
      <SectionMark id={section.id} count={count} />
      <span id={titleId}>{section.title}</span>
      {count > 0 ? (
        <span className="grid h-7 min-w-7 shrink-0 place-items-center rounded-full bg-card px-2 text-sm font-semibold tabular-nums text-ink shadow-card">
          {count}
        </span>
      ) : null}
    </>
  );
  const headClass =
    "flex w-full items-center gap-3 px-4 py-3 text-left text-xl leading-tight display-cut";
  return (
    <section
      aria-labelledby={titleId}
      data-section={section.id}
      {...focusList(section.id)}
      className="overflow-hidden rounded-panel border border-rule bg-card shadow-card"
    >
      <div className="flex flex-wrap items-center gap-x-3 bg-column">
        <h2
          {...focusHeading}
          tabIndex={toggles ? undefined : -1}
          className="min-w-0 flex-1"
        >
          {toggles ? (
            <button
              type="button"
              aria-expanded={open}
              aria-controls={bodyId}
              onClick={() => setOpen(!open)}
              className={`${headClass} rounded-panel transition-colors hover:bg-well`}
            >
              {head}
              <Icon
                name="chevronDown"
                size={18}
                className={`ml-auto text-ink-muted transition-transform duration-200 ease-out-expo ${open ? "" : "-rotate-90"}`}
              />
            </button>
          ) : (
            <span className={headClass}>{head}</span>
          )}
        </h2>
        {count === 0 ? (
          <p className="basis-full px-4 pb-3 text-sm text-ink-muted sm:ml-auto sm:basis-auto sm:pb-0">
            {section.empty}
          </p>
        ) : null}
      </div>
      {shown ? (
        <ul id={bodyId} className="divide-y divide-rule border-t border-rule">
          {children}
        </ul>
      ) : null}
      {aside}
    </section>
  );
}

// Row lays out one ticket (or session) on the Overview: its id, title and
// one line of words, then the agent, a time or count, and an action,
// aligned across rows from 48rem and stacked under the title below. item
// names the row across reloads (a ticket, question or run id) so focus can
// follow it.
function Row({
  item,
  ticket,
  title,
  sub,
  agent,
  agentNote,
  meta,
  action,
  children,
}: {
  item: string;
  ticket?: string;
  title: ReactNode;
  sub?: ReactNode;
  agent?: string;
  // agentNote qualifies the agent in a word or two, under its pill.
  agentNote?: string;
  meta?: ReactNode;
  action?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <li className="px-4 py-3" data-row={ticket} {...focusItem(item)}>
      <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-start gap-x-3 gap-y-1.5 md:grid-cols-[5rem_minmax(0,1fr)_5rem_11rem_10.5rem] md:items-center">
        <div className="pt-0.5 text-sm md:pt-0">
          {ticket ? <TicketLink id={ticket} className="text-ink" /> : null}
        </div>
        <div className="min-w-0">
          <div className="text-sm leading-snug font-semibold break-words text-ink">
            {title}
          </div>
          {sub ? (
            <p className="mt-0.5 text-xs break-words text-ink-muted">{sub}</p>
          ) : null}
        </div>
        <div className="col-start-2 flex flex-wrap items-center gap-x-3 gap-y-1.5 md:contents">
          <div className="flex flex-col items-start gap-0.5 max-md:empty:hidden md:justify-self-start">
            {agent ? (
              <span className="rounded-mark border border-rule bg-well px-1.5 py-0.5 text-2xs font-semibold whitespace-nowrap text-ink">
                {agent}
              </span>
            ) : null}
            {agent && agentNote ? (
              <span className="text-2xs whitespace-nowrap text-ink-muted">
                {agentNote}
              </span>
            ) : null}
          </div>
          <div className="text-xs text-ink-muted max-md:empty:hidden">
            {meta}
          </div>
          <div className="text-xs text-ink-muted max-md:empty:hidden md:justify-self-end md:text-right">
            {action}
          </div>
        </div>
      </div>
      {children ? (
        <div className="mt-2.5 md:pl-[calc(5rem+0.75rem)]">{children}</div>
      ) : null}
    </li>
  );
}

function Ago({
  iso,
  now,
  before = "",
  after = "",
}: {
  iso: string;
  now: Date;
  before?: string;
  after?: string;
}) {
  const words = durationWords(iso, now);
  if (!words) return null;
  return (
    <time dateTime={iso} title={absoluteTime(iso)}>
      {before}
      {words}
      {after}
    </time>
  );
}

// TicketTitle opens the ticket's panel; the id beside it opens its page.
function TicketTitle({
  id,
  title,
  onOpen,
}: {
  id: string;
  title: string;
  onOpen(ticket: TicketRef): void;
}) {
  return (
    <button
      type="button"
      onClick={() => onOpen({ id })}
      className="rounded-control text-left hover:underline"
    >
      {title || "Ticket not found"}
    </button>
  );
}

const where = (
  run: Pick<Run, "project" | "branch">,
  names: Map<string, string>,
) =>
  [names.get(run.project) ?? run.project, run.branch]
    .filter(Boolean)
    .join(" · ");

// DecisionRow is a question answered in place (CARD-6), or a permission
// prompt, which Flashheart never grants: it says where to answer it.
export function DecisionRow({
  item,
  now,
  projectNames,
  onOpen,
  onAnswer,
}: {
  item: DecisionItem;
  now: Date;
  projectNames: Map<string, string>;
  onOpen(ticket: TicketRef): void;
  onAnswer?: Answer;
}) {
  const { question, run, ticket } = item;
  const place = where(item, projectNames);
  const title = ticket ? (
    <TicketTitle id={ticket.id} title={ticket.title} onOpen={onOpen} />
  ) : (
    `Session ${shortID(run.short)}`
  );
  if (question) {
    return (
      <Row
        item={item.key}
        ticket={ticket?.id}
        title={title}
        sub={ticket ? undefined : place}
        agent={agentName(run.agent)}
        agentNote={agentNote(run)}
        meta={<Ago iso={item.since} now={now} before="Asked " after=" ago" />}
      >
        <QuestionCard
          question={question}
          asker={question.runShort}
          now={now}
          headingLevel={3}
          onAnswer={
            onAnswer ? (answer) => onAnswer(question.id, answer) : undefined
          }
        />
      </Row>
    );
  }
  return (
    <Row
      item={item.key}
      ticket={ticket?.id}
      title={title}
      sub={item.reason}
      agent={agentName(run.agent)}
      agentNote={agentNote(run)}
      meta={<Ago iso={item.since} now={now} before="Waiting " />}
      action={<span>Answer in the session · {place}</span>}
    />
  );
}

const criteriaMet = (card: Card) =>
  card.criteria.total > 0
    ? `${card.criteria.done}/${card.criteria.total} criteria met`
    : "No acceptance criteria";

function ReviewRow({
  item,
  now,
  onOpen,
  onReview,
}: {
  item: ReviewItem;
  now: Date;
  onOpen(ticket: TicketRef): void;
  onReview(ticket: TicketRef): void;
}) {
  const { card } = item;
  return (
    <Row
      item={card.id}
      ticket={card.id}
      title={<TicketTitle id={card.id} title={card.title} onOpen={onOpen} />}
      sub={criteriaMet(card)}
      agent={card.sessions?.agent ? agentName(card.sessions.agent) : undefined}
      meta={<Ago iso={item.since} now={now} before="Waiting " />}
      action={
        <Button
          variant="primary"
          className="h-9 py-0 whitespace-nowrap"
          aria-label={`Review results of ${card.id}`}
          onClick={() => onReview({ id: card.id })}
        >
          Review results
          <Icon name="next" size={14} />
        </Button>
      }
    />
  );
}

// liveAgent names the agent of a live session, and none otherwise.
const liveAgent = (card: Card) =>
  card.sessions?.state && card.sessions.state !== "ended"
    ? agentName(card.sessions.agent)
    : undefined;

function RiskRow({
  item,
  now,
  onOpen,
}: {
  item: RiskItem;
  now: Date;
  onOpen(ticket: TicketRef): void;
}) {
  const { card } = item;
  return (
    <Row
      item={card.id}
      ticket={card.id}
      title={<TicketTitle id={card.id} title={card.title} onOpen={onOpen} />}
      sub={item.reasons.join(" · ")}
      agent={liveAgent(card)}
      meta={
        card.column === "up-next" && !card.sessions?.run ? (
          "Not started"
        ) : (
          <Ago
            iso={item.since}
            now={now}
            before={item.sinceSession ? "Last activity " : "Last changed "}
            after=" ago"
          />
        )
      }
    />
  );
}

function ProgressRow({
  item,
  now,
  onOpen,
}: {
  item: ProgressItem;
  now: Date;
  onOpen(ticket: TicketRef): void;
}) {
  const { card } = item;
  return (
    <Row
      item={card.id}
      ticket={card.id}
      title={<TicketTitle id={card.id} title={card.title} onOpen={onOpen} />}
      sub={item.words}
      agent={liveAgent(card)}
      meta={
        card.criteria.total > 0
          ? `Criteria ${card.criteria.done}/${card.criteria.total}`
          : "No criteria"
      }
      action={<Ago iso={item.since} now={now} after=" ago" />}
    />
  );
}

// UnticketedRow is a live session with no ticket: where it runs, its state,
// and New ticket for its project.
export function UnticketedRow({
  run,
  now,
  projectNames,
  onCreateTicket,
}: {
  run: Run;
  now: Date;
  projectNames: Map<string, string>;
  onCreateTicket?(project: string): void;
}) {
  return (
    <Row
      item={run.id}
      title={`Session ${shortID(run.short)}`}
      sub={where(run, projectNames)}
      agent={agentName(run.agent)}
      agentNote={agentNote(run)}
      meta={
        <span className="flex flex-wrap items-center gap-x-2">
          <RunStateLabel state={run.state} />
          <Ago iso={run.lastActivity} now={now} after=" ago" />
        </span>
      }
      action={
        onCreateTicket ? (
          <Button
            className="h-9 py-0 whitespace-nowrap"
            onClick={() => onCreateTicket(run.project)}
          >
            <Icon name="plus" size={14} />
            Create ticket
          </Button>
        ) : null
      }
    />
  );
}

function UpNextRow({
  card,
  onOpen,
}: {
  card: Card;
  onOpen(ticket: TicketRef): void;
}) {
  const [blocker] = card.blockedBy;
  return (
    <Row
      item={card.id}
      ticket={card.id}
      title={<TicketTitle id={card.id} title={card.title} onOpen={onOpen} />}
      sub={card.blocked && blocker ? blocker.text : "Ready to start"}
    />
  );
}

// OverviewSkeleton holds the Overview's place while it loads (ui-layout.md
// §8): its section cards in their one column.
export function OverviewSkeleton() {
  return (
    <div aria-hidden="true" data-skeleton="overview" className={column}>
      {[0, 1, 2].map((section) => (
        <div
          key={section}
          className="flex flex-col gap-2 rounded-panel border border-rule bg-card p-3"
        >
          <div className="h-7 w-1/3 rounded-full bg-well" />
          <div className="h-10 animate-pulse rounded-card bg-well" />
          <div className="h-10 animate-pulse rounded-card bg-well" />
        </div>
      ))}
    </div>
  );
}
