import { useId, useState } from "react";

import type { TicketDetail, TicketRef } from "../../api/board";
import type { Run } from "../../api/runs";
import { Icon } from "../../components/Icon";
import { RunStateLabel } from "../../components/RunState";
import { StateNote } from "../../components/StateNote";
import {
  agentName,
  counted,
  needsReason,
  sessionsOf,
  shortID,
} from "../../model/runs";
import { absoluteTime, runningTime } from "../../model/time";
import { useNow } from "../../state/useNow";
import { BranchLabel, RunDetail } from "./RunDetail";

// RunsTab lists the agent runs linked to a ticket, most recent first (CARD-1):
// each session with its state, plan, edited files and activity, and its
// subagents beneath it as a tree (agent-protocol §10), each opening to its
// own plan, files and checkpoints.
export function RunsTab({
  detail,
  onOpen,
}: {
  detail: Pick<TicketDetail, "id" | "branch" | "runs">;
  onOpen?(ticket: TicketRef): void;
}) {
  const now = useNow();
  const sessions = sessionsOf(detail.runs);
  if (sessions.length === 0) {
    return (
      <p className="text-sm text-ink-muted">
        No agent run is linked to {detail.id} in the last two days. Sessions
        link by claiming the ticket, or by working on its branch
        {detail.branch ? (
          <>
            {" "}
            (<code className="font-mono text-ink">{detail.branch}</code>)
          </>
        ) : null}{" "}
        while it is in progress.
      </p>
    );
  }
  return (
    <ol className="flex flex-col gap-8">
      {sessions.map((run) => (
        <li key={run.id}>
          <RunCard
            run={run}
            ticket={detail.id}
            subagents={detail.runs
              .filter((child) => child.parent === run.id)
              .sort((a, b) => a.started.localeCompare(b.started))}
            now={now}
            onOpen={onOpen}
          />
        </li>
      ))}
    </ol>
  );
}

function RunCard({
  run,
  ticket,
  subagents,
  now,
  onOpen,
}: {
  run: Run;
  ticket: string;
  subagents: Run[];
  now: Date;
  onOpen?(ticket: TicketRef): void;
}) {
  const headingId = useId();
  const [open, setOpen] = useState<string | null>(null);
  return (
    <article
      aria-labelledby={headingId}
      className="flex flex-col gap-3 rounded-card border border-rule bg-card p-4"
    >
      <header className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
        <RunStateLabel state={run.state} />
        {run.state === "needs-you" ? (
          <span className="font-semibold text-ink">
            {needsReason(run, subagents)}
          </span>
        ) : null}
        <h3 id={headingId} className="flex items-baseline gap-1.5">
          {run.parent ? (
            <>
              <span className="font-medium text-ink">
                {run.agentType || "Subagent"}
              </span>
              <span className="font-normal text-ink-muted">
                subagent of{" "}
                <span title={run.parent}>{shortID(run.parent)}</span>
              </span>
            </>
          ) : (
            <>
              <span className="font-medium text-ink">
                {agentName(run.agent)}
              </span>
              <span className="font-normal text-ink-muted" title={run.id}>
                {shortID(run.id)}
              </span>
            </>
          )}
        </h3>
        <span className="text-ink-muted">
          {run.linkedBy === "claim"
            ? "claimed"
            : run.linkedBy === "branch"
              ? "linked by branch"
              : "with its session"}
        </span>
        <BranchLabel branch={run.branch} />
        <time
          dateTime={run.lastActivity}
          title={`Last activity ${absoluteTime(run.lastActivity)}`}
          className="ml-auto text-2xs text-ink-muted"
        >
          {runningTime(run.lastActivity, now)}
        </time>
      </header>
      {run.noHandoff ? (
        <StateNote kind="warning">
          {`Ended without a handoff: ${counted(run.edits, "edit")} since its last checkpoint.`}
        </StateNote>
      ) : null}
      <RunDetail
        run={run}
        timeline={run.timeline ?? []}
        now={now}
        headingLevel={4}
      />
      {subagents.length > 0 ? (
        <section className="text-xs">
          <h4 className="mb-1.5 text-sm heading-cut">
            Subagents{" "}
            <span className="font-normal text-ink-muted">
              {subagents.length}
            </span>
          </h4>
          <ul
            aria-label={`Subagents of ${shortID(run.id)}`}
            className="ml-1.5 border-l border-ink/40"
          >
            {subagents.map((child) => (
              <li
                key={child.id}
                className="relative pl-5 before:absolute before:top-[1.05rem] before:left-0 before:h-px before:w-3.5 before:bg-ink/40"
              >
                <SubagentRow
                  run={child}
                  ticket={ticket}
                  expanded={open === child.id}
                  onToggle={() =>
                    setOpen((current) =>
                      current === child.id ? null : child.id,
                    )
                  }
                  now={now}
                  onOpen={onOpen}
                />
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </article>
  );
}

// SubagentRow is one subagent in a session's tree: its state, type, the
// ticket it claimed when that is another, and its current step (or, with
// no plan, its tool and edit counts); it opens to the subagent's own plan,
// files and activity, where its checkpoints show.
function SubagentRow({
  run,
  ticket,
  expanded,
  onToggle,
  now,
  onOpen,
}: {
  run: Run;
  ticket: string;
  expanded: boolean;
  onToggle(): void;
  now: Date;
  onOpen?(ticket: TicketRef): void;
}) {
  const detailId = useId();
  const elsewhere = run.ticket && run.ticket !== ticket ? run.ticket : "";
  return (
    <div className="py-1">
      <div className="flex min-w-0 items-center gap-x-3">
        <RunStateLabel state={run.state} className="shrink-0" />
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={detailId}
          onClick={onToggle}
          title={expanded ? "Hide what this run did" : "Show what this run did"}
          className="group/run flex shrink-0 items-center gap-1 rounded-control"
        >
          <Icon
            name="chevronDown"
            size={12}
            className={`text-ink-muted transition-transform duration-200 ease-out-expo motion-reduce:transition-none ${expanded ? "" : "-rotate-90"}`}
          />
          <span
            title={run.id}
            className="font-medium text-ink group-hover/run:underline"
          >
            {run.agentType || "Subagent"}
          </span>
        </button>
        <span className="min-w-0 flex-1 truncate text-ink-muted">
          {run.state === "needs-you" ? (
            <span className="font-semibold text-ink">
              {needsReason(run, [])}{" "}
            </span>
          ) : null}
          {elsewhere ? (
            <>
              on{" "}
              {onOpen ? (
                <a
                  href={`#ticket-${elsewhere}`}
                  onClick={(event) => {
                    event.preventDefault();
                    onOpen({ id: elsewhere });
                  }}
                  title={run.ticketTitle}
                  className="font-medium text-ink underline decoration-rule-strong/40 hover:decoration-rule-strong"
                >
                  {elsewhere}
                </a>
              ) : (
                <span title={run.ticketTitle} className="text-ink">
                  {elsewhere}
                </span>
              )}{" "}
            </>
          ) : null}
          {run.progress.total > 0 ? (
            <span title={run.progress.current}>
              <span className="tabular-nums">
                {run.progress.done}/{run.progress.total}
              </span>
              {run.progress.current ? ` · ${run.progress.current}` : ""}
            </span>
          ) : null}
        </span>
        {run.progress.total === 0 ? (
          <span className="shrink-0 text-ink-muted">
            {counted(run.tools, "tool")}
            {run.edits > 0 ? `, ${counted(run.edits, "edit")}` : ""}
          </span>
        ) : null}
        <time
          dateTime={run.lastActivity}
          title={`Last activity ${absoluteTime(run.lastActivity)}`}
          className="shrink-0 text-2xs text-ink-muted"
        >
          {runningTime(run.lastActivity, now)}
        </time>
      </div>
      <div id={detailId} hidden={!expanded} className="pt-3 pb-2">
        {expanded ? (
          <RunDetail
            run={run}
            timeline={run.timeline ?? []}
            now={now}
            headingLevel={5}
          />
        ) : null}
      </div>
    </div>
  );
}
