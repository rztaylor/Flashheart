import { type CSSProperties, useCallback, useEffect, useState } from "react";
import {
  fetchWorkstreams,
  type ProjectSummary,
  type TicketRef,
  type Workstream,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { reorderWorkstream } from "../../api/edit";
import { Aside } from "../../components/Aside";
import { EmptyState } from "../../components/EmptyState";
import { Icon } from "../../components/Icon";
import { KeyBadge } from "../../components/KeyBadge";
import { LineBullet } from "../../components/LineBullet";
import { StateNote } from "../../components/StateNote";
import type { Line } from "../../model/lines";
import { useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";
import { TransitLine } from "./TransitLine";

interface WorkstreamsViewProps {
  projects: ProjectSummary[];
  fetcher: AuthenticatedFetch;
  lines: Map<string, Map<string, Line>>;
  // revision reloads the lines when the board changes (LIFE-3).
  revision: number;
  // editing enables reordering stations (EDIT-4); absent when read-only.
  editing?: Editing;
  onOpen(ticket: TicketRef): void;
}

// WorkstreamsView draws every workstream of the given projects as a route
// card, one section per project (VIEW-4, ui-layout.md §4).
export function WorkstreamsView({
  projects,
  fetcher,
  lines,
  revision,
  editing,
  onOpen,
}: WorkstreamsViewProps) {
  const withLines = projects.filter(
    (project) => project.workstreams.length > 0,
  );
  if (withLines.length === 0) {
    return (
      <EmptyState title="No workstreams yet">
        A workstream is an ordered run of tickets toward one goal. Add one as{" "}
        <code className="font-mono">workstreams/&lt;slug&gt;.md</code> with a{" "}
        <code className="font-mono">tickets:</code> list, and it appears here as
        a line.
        <Aside placement="workstreams-none" className="mt-3" />
      </EmptyState>
    );
  }
  return (
    <div className="flex flex-col gap-10 overflow-y-auto px-4 pt-1 pb-16 md:px-6">
      {withLines.map((project) => (
        <ProjectLines
          key={project.name}
          project={project}
          showName={projects.length > 1}
          fetcher={fetcher}
          lines={lines.get(project.name) ?? new Map()}
          revision={revision}
          editing={editing}
          onOpen={onOpen}
        />
      ))}
    </div>
  );
}

function ProjectLines({
  project,
  showName,
  fetcher,
  lines,
  revision,
  editing,
  onOpen,
}: {
  project: ProjectSummary;
  showName: boolean;
  fetcher: AuthenticatedFetch;
  lines: Map<string, Line>;
  revision: number;
  editing?: Editing;
  onOpen(ticket: TicketRef): void;
}) {
  const load = useCallback(
    (signal: AbortSignal) => fetchWorkstreams(fetcher, project.name, signal),
    [fetcher, project.name],
  );
  const resource = useResource(load, project.name, revision);
  // A new order shows at once and settles when the lines reload.
  const [orders, setOrders] = useState<Record<string, string[]>>({});
  const data = resource.status === "ready" ? resource.data : undefined;
  // biome-ignore lint/correctness/useExhaustiveDependencies: clear once fresh lines arrive.
  useEffect(() => setOrders({}), [data]);
  const reorder = editing
    ? (workstream: Workstream) => (ids: string[]) => {
        setOrders((current) => ({ ...current, [workstream.slug]: ids }));
        reorderWorkstream(fetcher, project.name, workstream.slug, ids)
          .then(() => {
            editing.notify({ text: `Reordered ${workstream.title}.` });
            resource.reload();
          })
          .catch((error: unknown) => {
            setOrders({});
            resource.reload();
            editing.notify({
              text: `${workstream.title} was not reordered: ${error instanceof Error ? error.message : "unknown error"}`,
            });
          });
      }
    : undefined;
  const ordered = (workstream: Workstream): Workstream => {
    const ids = orders[workstream.slug];
    if (!ids) return workstream;
    const byId = new Map(
      workstream.tickets.map((ticket) => [ticket.id, ticket]),
    );
    return {
      ...workstream,
      tickets: ids.flatMap((id) => {
        const ticket = byId.get(id);
        return ticket ? [ticket] : [];
      }),
    };
  };
  return (
    <section
      aria-label={`${project.displayName} workstreams`}
      className="flex flex-col gap-4"
    >
      {showName ? (
        <h2 className="flex items-center gap-2.5 text-lg heading-cut">
          <KeyBadge>{project.key}</KeyBadge>
          {project.displayName}
        </h2>
      ) : null}
      {resource.status === "loading" ? (
        <div
          aria-hidden="true"
          className="h-44 animate-pulse rounded-panel border border-rule bg-card"
        />
      ) : null}
      {resource.status === "error" ? (
        <p role="alert" className="text-sm text-danger">
          Workstreams could not be loaded: {resource.error}
        </p>
      ) : null}
      {resource.status === "ready"
        ? resource.data.map((workstream) => (
            <WorkstreamLine
              key={workstream.slug}
              headingLevel={showName ? 3 : 2}
              project={project.name}
              workstream={ordered(workstream)}
              line={lines.get(workstream.slug)}
              onOpen={onOpen}
              onReorder={reorder?.(workstream)}
            />
          ))
        : null}
    </section>
  );
}

const statusLabel = {
  active: "Running",
  blocked: "Blocked",
  completed: "Completed",
};

const statusIcon = {
  active: "half",
  blocked: "diamond",
  completed: "check",
} as const;

function WorkstreamLine({
  project,
  workstream,
  line,
  headingLevel,
  onOpen,
  onReorder,
}: {
  headingLevel: 2 | 3;
  project: string;
  workstream: Workstream;
  line?: Line;
  onOpen(ticket: TicketRef): void;
  onReorder?(ids: string[]): void;
}) {
  const Heading = headingLevel === 2 ? "h2" : "h3";
  // The route card takes a wash of its line colour (none in dark), and its
  // stations sit on that surface.
  const colour = line ? `var(--fh-line-${line.colour})` : "var(--fh-ink-faint)";
  const surface = {
    "--route-surface": `color-mix(in srgb, ${colour} var(--fh-route-wash), var(--fh-card))`,
    "--route-ink": line
      ? `var(--fh-line-ink-${line.colour})`
      : "var(--fh-card)",
  } as CSSProperties;
  return (
    <article
      aria-labelledby={`ws-${project}-${workstream.slug}`}
      style={surface}
      className="rounded-panel border border-rule bg-(--route-surface) px-5 pt-5 pb-3 shadow-card"
    >
      <header className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <LineBullet line={line} size="xl" />
        <div className="flex min-w-0 flex-col gap-0.5">
          <Heading
            id={`ws-${project}-${workstream.slug}`}
            className="text-2xl leading-tight display-cut"
          >
            {workstream.title}
          </Heading>
          <span
            className={`flex items-center gap-1 text-xs ${workstream.status === "blocked" ? "font-semibold text-ink" : "text-ink-muted"}`}
          >
            <Icon name={statusIcon[workstream.status]} size={12} />
            {statusLabel[workstream.status]}
            {/* An unordered workstream is an epic: say its tickets have no
                set sequence, since its stations are not on a route. */}
            {workstream.ordered ? null : (
              <span className="font-normal text-ink-muted">· Any order</span>
            )}
          </span>
        </div>
        <span className="ml-auto text-sm text-ink-muted">
          <span className="font-semibold text-ink">{workstream.done}</span> of{" "}
          {workstream.total} served
        </span>
      </header>
      {workstream.blockedBy.length > 0 ||
      workstream.needsRepair.length > 0 ||
      workstream.warnings.length > 0 ? (
        <div className="mt-2 flex flex-col gap-1">
          {workstream.needsRepair.map((problem) => (
            <StateNote key={problem} kind="repair">
              {problem}
            </StateNote>
          ))}
          {workstream.blockedBy.map((reason) => (
            <StateNote key={reason.text} kind="blocked">
              {reason.text}
            </StateNote>
          ))}
          {workstream.warnings.map((warning) => (
            <StateNote key={warning} kind="warning">
              {warning}
            </StateNote>
          ))}
        </div>
      ) : null}
      <TransitLine
        workstream={workstream}
        line={line}
        onOpen={onOpen}
        onReorder={onReorder}
      />
    </article>
  );
}
