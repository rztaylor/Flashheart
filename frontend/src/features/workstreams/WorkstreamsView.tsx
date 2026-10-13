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
import { LoadError } from "../../components/LoadError";
import { StaleNote } from "../../components/StaleNote";
import { StateNote } from "../../components/StateNote";
import { messageOf } from "../../model/errors";
import type { Line } from "../../model/lines";
import { workstreamPageHref } from "../../model/markdown";
import { useResource } from "../../state/useResource";
import type { Editing } from "../editing/useEditing";
import { isFinished, keptExpanded } from "./collapse";
import { TransitLine } from "./TransitLine";

interface WorkstreamsViewProps {
  projects: ProjectSummary[];
  fetcher: AuthenticatedFetch;
  lines: Map<string, Map<string, Line>>;
  // revision reloads the lines when the board changes (LIFE-3).
  revision: number;
  // editing enables reordering stations (EDIT-4); absent when read-only.
  editing?: Editing;
  // expanded lists per project name the finished workstreams the user
  // expanded, a saved preference (FH-68); onExpanded replaces a project's.
  expanded: Record<string, string[]>;
  onExpanded(project: string, slugs: string[]): void;
  onOpen(ticket: TicketRef): void;
}

// WorkstreamsView draws every workstream of the given projects as a route
// card, one section per project (VIEW-4, ui-layout.md §4). A finished
// workstream shows collapsed to its head unless the user expanded it.
export function WorkstreamsView({
  projects,
  fetcher,
  lines,
  revision,
  editing,
  expanded,
  onExpanded,
  onOpen,
}: WorkstreamsViewProps) {
  const withLines = projects.filter(
    (project) => project.workstreams.length > 0,
  );
  if (withLines.length === 0) {
    return (
      <EmptyState title="No workstreams yet">
        A workstream is a group of tickets toward one goal. Add one as{" "}
        <code className="font-mono">workstreams/&lt;slug&gt;.md</code> with a{" "}
        <code className="font-mono">tickets:</code> list, and it appears here as
        a line, with track wherever one ticket depends on another.
        <Aside placement="workstreams-none" className="mt-3" />
      </EmptyState>
    );
  }
  return (
    <div className={gutter}>
      {withLines.map((project) => (
        <ProjectLines
          key={project.name}
          project={project}
          showName={projects.length > 1}
          fetcher={fetcher}
          lines={lines.get(project.name) ?? new Map()}
          revision={revision}
          editing={editing}
          expanded={expanded[project.name] ?? noSlugs}
          onExpanded={(slugs) => onExpanded(project.name, slugs)}
          onOpen={onOpen}
        />
      ))}
    </div>
  );
}

const noSlugs: string[] = [];

// gutter is the view's column, shared with its skeleton.
const gutter = "flex flex-col gap-10 overflow-y-auto px-4 pt-1 pb-16 md:px-6";

// WorkstreamsSkeleton holds the view's place while the projects load
// (ui-layout.md §8): route cards in its column.
export function WorkstreamsSkeleton() {
  return (
    <div data-skeleton="workstreams" className={gutter}>
      <div className="flex flex-col gap-4">
        <RouteCardSkeleton />
        <RouteCardSkeleton />
      </div>
    </div>
  );
}

// RouteCardSkeleton holds one route card's place while a project's lines
// load: the bullet, title and progress over a track of stations.
function RouteCardSkeleton() {
  return (
    <div
      aria-hidden="true"
      data-skeleton="route-card"
      className="rounded-panel border border-rule bg-card px-5 pt-5 pb-6 shadow-card"
    >
      <div className="flex items-center gap-4">
        <div className="size-10 shrink-0 animate-pulse rounded-full bg-well" />
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="h-6 w-1/3 animate-pulse rounded-card bg-well" />
          <div className="h-3 w-20 animate-pulse rounded-card bg-well" />
        </div>
        <div className="h-4 w-24 animate-pulse rounded-card bg-well" />
      </div>
      <div className="relative mt-8 flex items-center justify-between px-2">
        <div className="absolute inset-x-2 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-well" />
        {[0, 1, 2, 3, 4].map((station) => (
          <div
            key={station}
            className="relative size-6 rounded-full border-4 border-well bg-card"
          />
        ))}
      </div>
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
  expanded,
  onExpanded,
  onOpen,
}: {
  project: ProjectSummary;
  showName: boolean;
  fetcher: AuthenticatedFetch;
  lines: Map<string, Line>;
  revision: number;
  editing?: Editing;
  expanded: string[];
  onExpanded(slugs: string[]): void;
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
  // An expanded choice lasts only while its workstream stays finished: one
  // that reopens shows in full and collapses again once it finishes.
  // biome-ignore lint/correctness/useExhaustiveDependencies: onExpanded is a fresh closure each render; run when the lines or the choices change.
  useEffect(() => {
    if (!data) return;
    const kept = keptExpanded(data, expanded);
    if (kept !== expanded) onExpanded(kept);
  }, [data, expanded]);
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
              text: `${workstream.title} was not reordered: ${messageOf(error, "unknown error")}`,
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
      {resource.status === "loading" ? <RouteCardSkeleton /> : null}
      {resource.status === "error" ? (
        <LoadError
          what="Workstreams could not be loaded"
          error={resource.error}
          onRetry={resource.reload}
        />
      ) : null}
      {resource.status === "ready" && resource.error ? (
        <StaleNote error={resource.error} />
      ) : null}
      {resource.status === "ready"
        ? resource.data.map((workstream) => (
            <WorkstreamLine
              key={workstream.slug}
              headingLevel={showName ? 3 : 2}
              project={project.name}
              workstream={ordered(workstream)}
              line={lines.get(workstream.slug)}
              expanded={expanded.includes(workstream.slug)}
              onToggle={() =>
                onExpanded(
                  expanded.includes(workstream.slug)
                    ? expanded.filter((slug) => slug !== workstream.slug)
                    : [...expanded, workstream.slug],
                )
              }
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
  blocked: "blocked",
  completed: "check",
} as const;

// LineStatus is a workstream's derived status in words with its mark.
export function LineStatus({ workstream }: { workstream: Workstream }) {
  return (
    <span
      className={`flex items-center gap-1 text-xs ${workstream.status === "blocked" ? "font-semibold text-ink" : "text-ink-muted"}`}
    >
      <Icon name={statusIcon[workstream.status]} size={12} />
      {statusLabel[workstream.status]}
    </span>
  );
}

// Served is a workstream's progress: d of t served.
export function Served({ workstream }: { workstream: Workstream }) {
  return (
    <span className="text-sm text-ink-muted">
      <span className="font-semibold text-ink">{workstream.done}</span> of{" "}
      {workstream.total} served
    </span>
  );
}

// routeSurface is a route card's wash of its line colour (none in dark),
// on which its stations sit.
export function routeSurface(line?: Line): CSSProperties {
  const colour = line ? `var(--fh-line-${line.colour})` : "var(--fh-ink-faint)";
  return {
    "--route-surface": `color-mix(in srgb, ${colour} var(--fh-route-wash), var(--fh-card))`,
    "--route-ink": line
      ? `var(--fh-line-ink-${line.colour})`
      : "var(--fh-card)",
  } as CSSProperties;
}

// LineNotes are a workstream's repair, blocker and warning notes.
export function LineNotes({ workstream }: { workstream: Workstream }) {
  if (
    workstream.blockedBy.length === 0 &&
    workstream.needsRepair.length === 0 &&
    workstream.warnings.length === 0
  )
    return null;
  return (
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
  );
}

// WorkstreamLine is one route card. A finished workstream's head is a
// disclosure button and its body shows only when expanded (FH-68).
export function WorkstreamLine({
  project,
  workstream,
  line,
  headingLevel,
  expanded,
  onToggle,
  onOpen,
  onReorder,
}: {
  headingLevel: 2 | 3;
  project: string;
  workstream: Workstream;
  line?: Line;
  // expanded is the user's choice for a finished workstream.
  expanded: boolean;
  onToggle(): void;
  onOpen(ticket: TicketRef): void;
  onReorder?(ids: string[]): void;
}) {
  const Heading = headingLevel === 2 ? "h2" : "h3";
  const titleId = `ws-${project}-${workstream.slug}`;
  const bodyId = `${titleId}-body`;
  const collapsible = isFinished(workstream);
  const open = !collapsible || expanded;
  const surface = routeSurface(line);
  const page = workstreamPageHref({ project, slug: workstream.slug });
  // The head reads the same collapsed or not; only a finished workstream's
  // is a button, holding phrasing content only. The title alone names the
  // card.
  const Stack = collapsible ? "span" : "div";
  const head = (
    <>
      <LineBullet line={line} size="xl" />
      <Stack className="flex min-w-0 flex-col gap-0.5">
        {collapsible ? (
          <span id={titleId} className="text-2xl leading-tight display-cut">
            {workstream.title}
          </span>
        ) : (
          // The title opens the workstream's page (FH-139).
          <Heading id={titleId} className="text-2xl leading-tight display-cut">
            <a
              href={page}
              className="rounded-control hover:underline decoration-2 underline-offset-4"
            >
              {workstream.title}
            </a>
          </Heading>
        )}
        <LineStatus workstream={workstream} />
      </Stack>
      <span className="ml-auto">
        <Served workstream={workstream} />
      </span>
    </>
  );
  return (
    <article
      aria-labelledby={titleId}
      style={surface}
      className={`rounded-panel border border-rule bg-(--route-surface) px-5 pt-5 shadow-card ${open ? "pb-3" : "pb-5"}`}
    >
      {collapsible ? (
        <header className="flex items-start gap-2">
          <Heading className="min-w-0 flex-1">
            <button
              type="button"
              aria-expanded={open}
              aria-controls={bodyId}
              onClick={onToggle}
              className="group -m-2 flex w-[calc(100%+1rem)] flex-wrap items-center gap-x-4 gap-y-1 rounded-control p-2 text-left"
            >
              {head}
              <Icon
                name="chevronDown"
                size={20}
                className={`text-ink-muted transition-transform ease-out-expo group-hover:text-ink ${open ? "" : "-rotate-90"}`}
              />
            </button>
          </Heading>
          {/* A finished line's head is its disclosure, so its page opens
              from a link beside it. */}
          <a
            href={page}
            aria-label={`Open ${workstream.title}`}
            title={`Open ${workstream.title}`}
            className="mt-1.5 grid size-8 shrink-0 place-items-center rounded-control text-ink-muted hover:bg-well hover:text-ink"
          >
            <Icon name="expand" size={16} />
          </a>
        </header>
      ) : (
        <header className="flex flex-wrap items-center gap-x-4 gap-y-1">
          {head}
        </header>
      )}
      {open ? (
        <div id={bodyId}>
          <LineNotes workstream={workstream} />
          <TransitLine
            workstream={workstream}
            line={line}
            onOpen={onOpen}
            onReorder={onReorder}
          />
        </div>
      ) : null}
    </article>
  );
}
