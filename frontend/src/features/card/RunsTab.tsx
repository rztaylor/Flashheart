import { useId } from "react";

import type { TicketDetail } from "../../api/board";
import type { Run } from "../../api/runs";
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
import { BranchLabel, RunDetail } from "../agents/RunDetail";

// RunsTab lists the agent runs linked to a ticket, most recent first (CARD-1):
// each session with its state, plan, edited files and activity, and its
// subagents beneath it.
export function RunsTab({ detail }: { detail: TicketDetail }) {
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
            subagents={detail.runs.filter((child) => child.parent === run.id)}
            now={now}
          />
        </li>
      ))}
    </ol>
  );
}

function RunCard({
  run,
  subagents,
  now,
}: {
  run: Run;
  subagents: Run[];
  now: Date;
}) {
  const headingId = useId();
  const subagentsId = useId();
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
          <span className="font-medium text-ink">{agentName(run.agent)}</span>
          <span className="font-normal text-ink-muted" title={run.id}>
            {shortID(run.id)}
          </span>
        </h3>
        <span className="text-ink-muted">
          {run.linkedBy === "claim" ? "claimed" : "linked by branch"}
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
        <section
          aria-labelledby={subagentsId}
          className="border-l border-ink/40 pl-4 text-xs"
        >
          <h4 id={subagentsId} className="mb-1.5 text-sm heading-cut">
            Subagents
          </h4>
          <ul className="flex flex-col gap-1">
            {subagents.map((child) => (
              <li key={child.id} className="flex items-center gap-3">
                <RunStateLabel state={child.state} />
                <span className="text-ink">
                  {child.agentType || "Subagent"}
                </span>
                <span className="text-ink-muted">
                  {counted(child.tools, "tool")}
                  {child.edits > 0 ? `, ${counted(child.edits, "edit")}` : ""}
                </span>
                <span className="ml-auto text-2xs text-ink-muted">
                  {runningTime(child.lastActivity, now)}
                </span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </article>
  );
}
