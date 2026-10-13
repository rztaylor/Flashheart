import type { ReactNode } from "react";
import {
  STATUSES,
  type Status,
  statusWord,
  type TicketDetail,
} from "../../api/board";
import { BranchLabel } from "../../components/BranchLabel";
import { Button } from "../../components/Button";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import {
  Pill,
  StatusPill,
  stateToneClass,
  stateToneIcon,
  Tag,
} from "../../components/Pill";
import { PullRequestLabel } from "../../components/PullRequestLabel";
import { RunStateMark } from "../../components/RunState";
import { TicketLink } from "../../components/TicketLink";
import type { Line } from "../../model/lines";
import { STATE_LABEL } from "../../model/runs";
import { priorityLabel, statusOf } from "../../model/status";
import { absoluteTime, runningTime } from "../../model/time";

// PropertyRow is one property: its name and value. Each row is where an
// in-place edit of that property will sit (FH-66).
export function PropertyRow({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="contents">
      <dt className="py-1.5 text-ink-muted">{label}</dt>
      <dd className="flex min-w-0 flex-wrap items-center gap-1.5 py-1.5 text-ink">
        {children}
      </dd>
    </div>
  );
}

const none = <span className="text-ink-faint">None</span>;

// StatusSelect is Move to (EDIT-1) drawn as the ticket's status: the
// state's fill, icon and word, opening the statuses, Rejected among them
// (FH-58).
function StatusSelect({
  detail,
  onMove,
}: {
  detail: TicketDetail;
  onMove(to: Status): void;
}) {
  const { tone } = statusOf(detail.column, detail.rejected);
  const crossed = detail.rejected && detail.column === "done";
  return (
    <label className="flex flex-col gap-1 text-xs text-ink-muted">
      Move to
      <span
        className={`relative flex items-center rounded-control ${stateToneClass[tone]}`}
      >
        <Icon
          name={crossed ? "close" : stateToneIcon[tone]}
          size={14}
          className="pointer-events-none absolute left-3"
        />
        <select
          value={statusWord(detail)}
          onChange={(event) => onMove(event.target.value as Status)}
          className="h-9 w-full cursor-pointer appearance-none rounded-control bg-transparent pr-9 pl-9 text-sm font-semibold text-current"
        >
          {STATUSES.map((status) => (
            <option key={status.id} value={status.id}>
              {status.title}
            </option>
          ))}
        </select>
        <Icon
          name="chevronDown"
          size={12}
          className="pointer-events-none absolute right-3"
        />
      </span>
    </label>
  );
}

const changed = (modified: string) => {
  const age = runningTime(modified);
  if (!age) return "";
  return age === "now" ? "just now" : `${age} ago`;
};

const word = (text: string) =>
  text ? text.charAt(0).toUpperCase() + text.slice(1) : "";

// PropertiesPanel is the full page's properties (FH-65): the status with
// Move to, priority, type, workstream, branch, pull request, depends on,
// tags, agent, created and changed, then Archive. It stays in view beside
// the page. onMove and onArchive are absent on a read-only server.
export function PropertiesPanel({
  detail,
  line,
  workstreamTitle,
  onMove,
  onArchive,
}: {
  detail: TicketDetail;
  line?: Line;
  workstreamTitle: string;
  onMove?(to: Status): void;
  onArchive?(): void;
}) {
  const session = detail.sessions;
  return (
    <section
      aria-labelledby="properties-heading"
      className="rounded-panel border border-rule bg-column p-4"
    >
      <h2 id="properties-heading" className="sr-only">
        Properties
      </h2>
      {onMove ? (
        <StatusSelect detail={detail} onMove={onMove} />
      ) : (
        <StatusPill column={detail.column} rejected={detail.rejected} />
      )}
      {detail.blockedBy.length > 0 ? (
        <Pill tone="blocked" className="mt-2">
          Blocked
        </Pill>
      ) : null}
      <dl className="mt-3 grid grid-cols-[6.5rem_minmax(0,1fr)] gap-x-3 text-sm">
        <PropertyRow label="Priority">
          {detail.priority ? (
            <span className={detail.priority === "high" ? "font-semibold" : ""}>
              {priorityLabel(detail.priority)}
            </span>
          ) : (
            none
          )}
        </PropertyRow>
        <PropertyRow label="Type">
          {detail.type ? word(detail.type) : none}
        </PropertyRow>
        <PropertyRow label="Workstream">
          {detail.workstream ? (
            <>
              <LineBullet line={line} size="sm" />
              <span className="min-w-0">{workstreamTitle}</span>
            </>
          ) : (
            none
          )}
        </PropertyRow>
        <PropertyRow label="Branch">
          {detail.branch ? (
            <BranchLabel branch={detail.branch} className="text-xs" />
          ) : (
            none
          )}
        </PropertyRow>
        {detail.pr.trim() ? (
          <PropertyRow label="Pull request">
            <PullRequestLabel pr={detail.pr} className="text-xs" />
          </PropertyRow>
        ) : null}
        <PropertyRow label="Depends on">
          {detail.dependsOn.length + detail.dependsOnWorkstreams.length > 0 ? (
            <>
              {detail.dependsOn.map((id) => (
                <TicketLink key={id} id={id} className="text-ink" />
              ))}
              {detail.dependsOnWorkstreams.map((slug) => (
                <span key={slug}>{slug}</span>
              ))}
            </>
          ) : (
            none
          )}
        </PropertyRow>
        <PropertyRow label="Tags">
          {detail.tags.length > 0
            ? detail.tags.map((tag) => <Tag key={tag}>{tag}</Tag>)
            : none}
        </PropertyRow>
        <PropertyRow label="Agent">
          {session?.short ? (
            <>
              {session.state ? (
                <RunStateMark state={session.state} still />
              ) : null}
              <span className="min-w-0 break-all">{session.short}</span>
              {session.state ? (
                <span className="text-xs text-ink-muted">
                  {STATE_LABEL[session.state]}
                </span>
              ) : null}
            </>
          ) : (
            none
          )}
        </PropertyRow>
        <PropertyRow label="Created">{detail.created || none}</PropertyRow>
        <PropertyRow label="Changed">
          <span title={absoluteTime(detail.modified)}>
            {changed(detail.modified) || "Unknown"}
          </span>
        </PropertyRow>
      </dl>
      {onArchive ? (
        <div className="mt-3 border-t border-rule pt-3">
          <Button className="h-8 w-full py-0 text-xs" onClick={onArchive}>
            <Icon name="archive" size={14} />
            Archive
          </Button>
        </div>
      ) : null}
    </section>
  );
}
