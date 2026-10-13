import type { MouseEvent, ReactNode } from "react";

import {
  STATUSES,
  type Status,
  statusWord,
  type TicketDetail,
} from "../../api/board";
import { BranchLabel } from "../../components/BranchLabel";
import { Button } from "../../components/Button";
import { Select } from "../../components/Field";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import { Pill, StatusPill, Tag } from "../../components/Pill";
import { PullRequestLabel } from "../../components/PullRequestLabel";
import { idCut, TicketLink } from "../../components/TicketLink";
import type { Line } from "../../model/lines";
import type { Step } from "../../model/order";
import { priorityLabel } from "../../model/status";
import { absoluteTime, runningTime } from "../../model/time";

// TicketIdentity is the id and project, then the title (ui-layout.md §3).
// As the panel's heading (level 2) the id links to the ticket's full page;
// as the page's own heading (level 1) it is plain text.
// On the full page, title replaces the heading while it is edited, and
// action (its pencil) sits after it (FH-66).
export function TicketIdentity({
  detail,
  level,
  title,
  action,
  onDoubleClick,
}: {
  detail: TicketDetail;
  level: 1 | 2;
  title?: ReactNode;
  action?: ReactNode;
  onDoubleClick?(event: MouseEvent): void;
}) {
  const Heading = level === 1 ? "h1" : "h2";
  const heading = (
    <Heading
      onDoubleClick={onDoubleClick}
      className={`mt-1 leading-tight display-cut ${level === 1 ? "text-3xl" : "text-2xl"} ${action ? "min-w-0" : ""}`}
    >
      {detail.title}
    </Heading>
  );
  return (
    <>
      <p className="flex items-center gap-2 text-sm text-ink-muted">
        {level === 1 ? (
          <span className={`${idCut} text-ink`}>{detail.id}</span>
        ) : (
          <TicketLink id={detail.id} className="text-ink" />
        )}
        <span>{detail.project}</span>
      </p>
      {title ??
        (action ? (
          <div className="group/edit flex items-start gap-2">
            {heading}
            <span className="mt-2">{action}</span>
          </div>
        ) : (
          heading
        ))}
    </>
  );
}

// TicketProperties is the pill row (column, blocker, priority, type) and
// the meta row (workstream, created, branch, pull request, last change).
export function TicketProperties({
  detail,
  line,
  workstreamTitle,
}: {
  detail: TicketDetail;
  line?: Line;
  workstreamTitle: string;
}) {
  const priority = detail.priority
    ? `${priorityLabel(detail.priority)} priority`
    : "";
  return (
    <>
      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        <StatusPill column={detail.column} rejected={detail.rejected} />
        {detail.blockedBy.length > 0 ? (
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
        <BranchLabel branch={detail.branch} />
        <PullRequestLabel pr={detail.pr} />
        <span title={absoluteTime(detail.modified)}>
          {changedLabel(detail.modified)}
        </span>
      </p>
    </>
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

// TicketActions moves the ticket to any column, rejects it (FH-58), moves
// it to a place in its column (the menu alternatives to dragging, EDIT-1,
// EDIT-9) or archives it (EDIT-8).
export function TicketActions({
  detail,
  onMove,
  onStep,
  onArchive,
}: {
  detail: TicketDetail;
  onMove(to: Status): void;
  onStep?: (step: Step) => void;
  onArchive(): void;
}) {
  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      {/* biome-ignore lint/a11y/noLabelWithoutControl: the Select inside is the control. */}
      <label className="flex items-center gap-2 text-xs text-ink-muted">
        Move to
        <Select
          value={statusWord(detail)}
          onChange={(value) => onMove(value as Status)}
        >
          {STATUSES.map((status) => (
            <option key={status.id} value={status.id}>
              {status.title}
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

function changedLabel(modified: string): string {
  const age = runningTime(modified);
  if (!age) return "";
  return age === "now" ? "changed just now" : `changed ${age} ago`;
}
