import { useCallback, useId, useState } from "react";

import type { ProjectSummary, TicketRef } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { answerQuestion } from "../../api/edit";
import { fetchRun, fetchRuns, type Run, type RunState } from "../../api/runs";
import { Button } from "../../components/Button";
import { EmptyState } from "../../components/EmptyState";
import { Icon } from "../../components/Icon";
import { PlanRoute } from "../../components/PlanRoute";
import { RunStateLabel, RunStateMark } from "../../components/RunState";
import {
  agentName,
  counted,
  type LaneRun,
  laneRuns,
  needsReason,
  STATE_LABEL,
  shortID,
} from "../../model/runs";
import { absoluteTime, runningTime } from "../../model/time";
import { useNow } from "../../state/useNow";
import { useResource } from "../../state/useResource";
import { BranchLabel, RunDetail } from "./RunDetail";

interface AgentsViewProps {
  fetcher: AuthenticatedFetch;
  // project limits the view to one project; "" shows every project.
  project: string;
  projects: ProjectSummary[];
  revision: number;
  onOpen(ticket: TicketRef): void;
}

const emptyLane: Record<RunState, string> = {
  "needs-you": "Nobody needs you right now.",
  working: "No agent is working.",
  quiet: "No working agent has gone quiet.",
  waiting: "No session is waiting for a prompt.",
  ended: "No runs ended in the last day.",
};

// Row columns: state, run, ticket, plan, branch, last activity.
const rowGrid =
  "grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-1 lg:grid-cols-[8.5rem_9rem_minmax(10rem,26rem)_minmax(12rem,22rem)_11rem_minmax(3rem,1fr)] lg:items-center";

// AgentsView is the departure board of agent runs (VIEW-3): lanes by state,
// Needs you first, each run one dense row with its ticket, plan and last
// activity; subagents hang under their session on a spur. A row opens to its
// plan, edited files and activity; the ticket opens the card panel.
export function AgentsView({
  fetcher,
  project,
  projects,
  revision,
  onOpen,
}: AgentsViewProps) {
  const [endedAll, setEndedAll] = useState(false);
  const [open, setOpen] = useState<string>("");
  const now = useNow();
  const load = useCallback(
    (signal: AbortSignal) => fetchRuns(fetcher, project, endedAll, signal),
    [fetcher, project, endedAll],
  );
  const runs = useResource(load, `${project}:${endedAll}`, revision);
  const keys = new Map(projects.map((summary) => [summary.name, summary.key]));

  if (runs.status === "loading") return <AgentsSkeleton />;
  if (runs.status === "error") {
    return (
      <div className="m-4 flex items-center gap-3 text-sm">
        <p role="alert" className="text-danger">
          Agent runs could not be loaded: {runs.error}
        </p>
        <Button onClick={runs.reload}>
          <Icon name="refresh" size={14} />
          Try again
        </Button>
      </div>
    );
  }
  const { runs: list, hiddenEnded } = runs.data;
  if (list.length === 0 && hiddenEnded === 0) {
    return (
      <EmptyState title="No agent runs yet">
        Runs appear here as soon as a Claude Code session with Flashheart's
        hooks starts in one of your repositories. Connect Claude Code with{" "}
        <code className="font-mono text-ink">flashheart setup claude</code>.
      </EmptyState>
    );
  }
  const lanes = laneRuns(list);
  return (
    <div className="relative min-h-0 flex-1 overflow-y-auto">
      <div className="px-4 pb-12">
        {runs.error ? (
          <p role="status" className="mt-3 text-xs text-ink-muted">
            Showing the last good copy: {runs.error}
          </p>
        ) : null}
        {lanes.map((lane) => (
          <LaneSection
            key={lane.state}
            state={lane.state}
            runs={lane.runs}
            keys={project ? undefined : keys}
            open={open}
            onToggle={(id) => setOpen((current) => (current === id ? "" : id))}
            fetcher={fetcher}
            revision={revision}
            now={now}
            onOpen={onOpen}
            footer={
              lane.state === "ended" && (hiddenEnded > 0 || endedAll) ? (
                <Button
                  variant="quiet"
                  className="mt-1 text-xs"
                  onClick={() => setEndedAll(!endedAll)}
                >
                  {endedAll
                    ? "Show the last day only"
                    : `Show ${hiddenEnded} older ended ${hiddenEnded === 1 ? "run" : "runs"}`}
                </Button>
              ) : null
            }
          />
        ))}
      </div>
    </div>
  );
}

interface LaneSectionProps {
  state: RunState;
  runs: LaneRun[];
  keys?: Map<string, string>;
  open: string;
  onToggle(id: string): void;
  fetcher: AuthenticatedFetch;
  revision: number;
  now: Date;
  onOpen(ticket: TicketRef): void;
  footer?: React.ReactNode;
}

function LaneSection({
  state,
  runs,
  keys,
  open,
  onToggle,
  fetcher,
  revision,
  now,
  onOpen,
  footer,
}: LaneSectionProps) {
  const headingId = useId();
  return (
    <section
      aria-labelledby={headingId}
      data-lane={state}
      className="mt-6 first:mt-4"
    >
      <h2
        id={headingId}
        className="flex items-baseline justify-between border-t-[5px] border-rule-strong pt-2 pb-1.5 text-md station-sign"
      >
        <span className="flex items-center gap-2">
          <RunStateMark
            state={state}
            size={12}
            className="translate-y-[1px]"
            still
          />
          {STATE_LABEL[state]}
        </span>
        <span className="text-sm font-semibold text-ink">{runs.length}</span>
      </h2>
      {runs.length === 0 ? (
        <p className="border-t border-rule py-2.5 text-xs text-ink-muted">
          {emptyLane[state]}
        </p>
      ) : (
        <ul className="border-t border-rule">
          {runs.map((entry) => (
            <li key={entry.run.id} className="border-b border-rule">
              <RunRow
                run={entry.run}
                subagents={entry.children}
                projectKey={keys?.get(entry.run.project)}
                expanded={open === entry.run.id}
                onToggle={() => onToggle(entry.run.id)}
                fetcher={fetcher}
                revision={revision}
                now={now}
                onOpen={onOpen}
              />
              {entry.children.length > 0 ? (
                <ul
                  aria-label={`Subagents of ${entry.run.short}`}
                  className="mb-2 ml-3 border-l border-ink/40 pl-0"
                >
                  {entry.children.map((child) => (
                    <li
                      key={child.id}
                      className="relative pl-5 before:absolute before:top-[1.15rem] before:left-0 before:h-px before:w-3.5 before:bg-ink/40"
                    >
                      <RunRow
                        run={child}
                        expanded={open === child.id}
                        onToggle={() => onToggle(child.id)}
                        fetcher={fetcher}
                        revision={revision}
                        now={now}
                        onOpen={onOpen}
                        subagent
                      />
                    </li>
                  ))}
                </ul>
              ) : null}
            </li>
          ))}
        </ul>
      )}
      {footer}
    </section>
  );
}

interface RunRowProps {
  run: Run;
  projectKey?: string;
  expanded: boolean;
  onToggle(): void;
  fetcher: AuthenticatedFetch;
  revision: number;
  now: Date;
  onOpen(ticket: TicketRef): void;
  subagent?: boolean;
  // subagents are a session's subagents, to say which one needs you.
  subagents?: Run[];
}

function RunRow({
  run,
  projectKey,
  expanded,
  onToggle,
  fetcher,
  revision,
  now,
  onOpen,
  subagent,
  subagents = [],
}: RunRowProps) {
  const detailId = useId();
  const name = subagent
    ? `${run.agentType || "Subagent"}`
    : agentName(run.agent);
  const note =
    run.state === "needs-you"
      ? needsReason(run, subagents)
      : run.noHandoff
        ? "No handoff since its edits"
        : "";
  return (
    <div data-run={run.id} className={expanded ? "bg-well" : undefined}>
      <div className={`${rowGrid} px-2 py-2`}>
        <div className="flex min-w-0 flex-col items-start gap-0.5 text-xs max-lg:col-span-2 max-lg:flex-row max-lg:items-center max-lg:gap-3">
          <RunStateLabel state={run.state} />
          {note ? (
            <span
              className={`text-2xs ${run.state === "needs-you" ? "font-semibold text-ink" : "text-ink-muted"}`}
            >
              {run.noHandoff && run.state !== "needs-you" ? (
                <Icon
                  name="warning"
                  size={10}
                  className="mr-1 inline align-[-1px]"
                />
              ) : null}
              {note}
            </span>
          ) : null}
        </div>
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={detailId}
          onClick={onToggle}
          title={expanded ? "Hide what this run did" : "Show what this run did"}
          className="group/run flex min-w-0 items-center gap-1.5 rounded-control text-left text-sm"
        >
          <Icon
            name="chevronDown"
            size={12}
            className={`text-ink-muted transition-transform duration-200 ease-out-expo ${expanded ? "" : "-rotate-90"}`}
          />
          <span className="truncate font-medium text-ink group-hover/run:underline">
            {name}
          </span>
          <span className="shrink-0 text-2xs text-ink-muted">
            {/* A subagent by its own id; a session by its session id. */}
            {shortID(run.id)}
          </span>
        </button>
        <div className="flex min-w-0 items-center gap-2 text-sm max-lg:col-span-2">
          {projectKey && !subagent ? (
            <span
              title={run.project}
              className="shrink-0 rounded-[3px] px-1 text-2xs leading-4 font-bold station-sign shadow-[inset_0_0_0_1.25px_var(--fh-ink-muted)]"
            >
              {projectKey}
            </span>
          ) : null}
          {run.ticket ? (
            <button
              type="button"
              onClick={() => onOpen({ id: run.ticket })}
              className="flex min-w-0 items-baseline gap-2 rounded-control text-left hover:underline"
              title={
                run.linkedBy === "branch"
                  ? `Linked by branch ${run.branch}`
                  : "Claimed"
              }
            >
              <span className="shrink-0 text-2xs font-semibold tracking-[0.02em] text-ink">
                {run.ticket}
              </span>
              <span className="truncate text-ink">
                {run.ticketTitle || "Ticket not found"}
              </span>
            </button>
          ) : subagent ? (
            <span className="text-xs text-ink-muted">
              {counted(run.tools, "tool")}
              {run.edits > 0 ? `, ${counted(run.edits, "edit")}` : ""}
            </span>
          ) : (
            <span className="text-xs text-ink-muted">Unassigned</span>
          )}
        </div>
        <div className="flex min-w-0 items-center gap-2 text-xs">
          <PlanRoute
            plan={run.plan}
            done={run.progress.done}
            total={run.progress.total}
          />
          {run.progress.current ? (
            <span className="truncate text-ink" title={run.progress.current}>
              {run.progress.current}
            </span>
          ) : run.progress.total === 0 ? (
            <span className="text-ink-faint">No plan</span>
          ) : null}
        </div>
        <div className="min-w-0 max-lg:col-span-2">
          {subagent ? null : <BranchLabel branch={run.branch} />}
        </div>
        <time
          dateTime={run.lastActivity}
          title={`Last activity ${absoluteTime(run.lastActivity)}`}
          className="text-right text-2xs text-ink-muted max-lg:row-start-2 max-lg:col-start-2"
        >
          {runningTime(run.lastActivity, now)}
        </time>
      </div>
      {expanded ? (
        <div
          id={detailId}
          className="px-2 pt-1 pb-4 lg:pl-[calc(8.5rem+1rem+0.5rem)]"
        >
          <ExpandedRun
            run={run}
            fetcher={fetcher}
            revision={revision}
            now={now}
          />
        </div>
      ) : null}
    </div>
  );
}

// ExpandedRun loads the run's timeline when its row opens.
function ExpandedRun({
  run,
  fetcher,
  revision,
  now,
}: {
  run: Run;
  fetcher: AuthenticatedFetch;
  revision: number;
  now: Date;
}) {
  const load = useCallback(
    (signal: AbortSignal) => fetchRun(fetcher, run.id, signal),
    [fetcher, run.id],
  );
  const detail = useResource(load, run.id, revision);
  const answer = (id: string, text: string): Promise<string | undefined> =>
    answerQuestion(fetcher, id, text)
      .then(() => {
        detail.reload();
        return undefined;
      })
      .catch(
        (error: unknown) =>
          `Not sent: ${error instanceof Error ? error.message : "unknown error"}`,
      );
  if (detail.status === "error") {
    return (
      <p className="text-xs text-ink-muted">
        This run's activity could not be loaded: {detail.error}
      </p>
    );
  }
  return (
    <RunDetail
      run={detail.status === "ready" ? detail.data : run}
      timeline={detail.data?.timeline}
      now={now}
      onAnswer={answer}
    />
  );
}

function AgentsSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col gap-6 p-4">
      {[0, 1, 2].map((lane) => (
        <div key={lane} className="flex flex-col gap-2">
          <div className="h-6 border-t-[5px] border-rule" />
          <div className="h-9 animate-pulse rounded-card bg-well" />
          <div className="h-9 animate-pulse rounded-card bg-well" />
        </div>
      ))}
    </div>
  );
}
