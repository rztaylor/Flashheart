import {
  STATUSES,
  type Status,
  statusWord,
  type TicketDetail,
  type WorkstreamBrief,
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
import {
  PriorityMarker,
  priorityMark,
  TypeIcon,
} from "../../components/TicketMarks";
import type { Line } from "../../model/lines";
import { STATE_LABEL } from "../../model/runs";
import { priorityLabel, statusOf } from "../../model/status";
import { PRIORITIES, TICKET_TYPES } from "../../model/tickets";
import { absoluteTime, runningTime } from "../../model/time";
import {
  choiceProperty,
  dependsOnProperty,
  EditableProperty,
  type PropertyEditing,
  PropertyRow,
  tagsProperty,
  textProperty,
} from "./PropertyEdits";

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
// the page. With editing, priority, type, workstream (one of workstreams),
// branch, pull request, depends on and tags edit in place with typed
// controls (FH-66); agent, created and changed stay read-only. On a ticket
// in review, onRequestChanges offers Request changes under Move to
// (REV-6). onMove, onArchive, onRequestChanges and editing are absent on a
// read-only server.
export function PropertiesPanel({
  detail,
  line,
  workstreamTitle,
  workstreams = [],
  onMove,
  onArchive,
  onRequestChanges,
  editing,
}: {
  detail: TicketDetail;
  line?: Line;
  workstreamTitle: string;
  workstreams?: WorkstreamBrief[];
  onMove?(to: Status): void;
  onArchive?(): void;
  onRequestChanges?(): void;
  editing?: PropertyEditing;
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
      {onRequestChanges && detail.column === "review" ? (
        <Button
          variant="secondary"
          onClick={onRequestChanges}
          className="mt-2 w-full justify-center"
        >
          <Icon name="comment" size={14} />
          Request changes
        </Button>
      ) : null}
      {detail.blockedBy.length > 0 ? (
        <Pill tone="blocked" className="mt-2">
          Blocked
        </Pill>
      ) : null}
      <dl className="mt-3 grid grid-cols-[6.5rem_minmax(0,1fr)] gap-x-3 text-sm">
        <EditableProperty
          label="Priority"
          field="priority"
          editing={editing}
          {...choiceProperty(
            "priority",
            "Priority",
            PRIORITIES.map((value) => ({
              value,
              label: priorityLabel(value),
            })),
            priorityMark,
          )}
          view={
            detail.priority ? (
              <>
                <PriorityMarker priority={detail.priority} />
                <span
                  className={detail.priority === "high" ? "font-semibold" : ""}
                >
                  {priorityLabel(detail.priority)}
                </span>
              </>
            ) : (
              none
            )
          }
        />
        <EditableProperty
          label="Type"
          field="type"
          editing={editing}
          {...choiceProperty(
            "type",
            "Type",
            TICKET_TYPES.map((value) => ({ value, label: word(value) })),
          )}
          view={
            detail.type ? (
              <>
                <TypeIcon type={detail.type} />
                {word(detail.type)}
              </>
            ) : (
              none
            )
          }
        />
        <EditableProperty
          label="Workstream"
          field="workstream"
          editing={editing}
          {...choiceProperty("workstream", "Workstream", [
            { value: "", label: "None" },
            ...workstreams.map((item) => ({
              value: item.slug,
              label: item.title,
            })),
          ])}
          view={
            detail.workstream ? (
              <>
                <LineBullet line={line} size="sm" />
                <span className="min-w-0">{workstreamTitle}</span>
              </>
            ) : (
              none
            )
          }
        />
        <EditableProperty
          label="Branch"
          field="branch"
          editing={editing}
          {...textProperty("branch", "Branch")}
          view={
            detail.branch ? (
              <BranchLabel branch={detail.branch} className="text-xs" />
            ) : (
              none
            )
          }
        />
        {detail.pr.trim() ? (
          <EditableProperty
            label="Pull request"
            field="pr"
            editing={editing}
            {...textProperty("pr", "Pull request")}
            view={<PullRequestLabel pr={detail.pr} className="text-xs" />}
          />
        ) : null}
        <EditableProperty
          label="Depends on"
          field="depends-on"
          editing={editing}
          {...dependsOnProperty(detail.id, editing?.edit.tickets)}
          view={
            detail.dependsOn.length + detail.dependsOnWorkstreams.length > 0 ? (
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
            )
          }
        />
        <EditableProperty
          label="Tags"
          field="tags"
          editing={editing}
          {...tagsProperty()}
          view={
            detail.tags.length > 0
              ? detail.tags.map((tag) => <Tag key={tag}>{tag}</Tag>)
              : none
          }
        />
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
