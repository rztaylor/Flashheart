import { useMemo } from "react";

import { type Card, COLUMNS, type Column, type TicketRef } from "../api/board";
import type { AuthenticatedFetch } from "../api/client";
import type { Preferences } from "../api/preferences";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { Icon } from "../components/Icon";
import { ArchiveView } from "../features/archive/ArchiveView";
import { ProjectsArchiveView } from "../features/archive/ProjectsArchiveView";
import { BoardView, NoTickets } from "../features/board/BoardView";
import type { PanelTab } from "../features/card/CardPanel";
import { FilterBar } from "../features/filters/FilterBar";
import { FilterChips } from "../features/filters/FilterChips";
import { OverviewView } from "../features/overview/OverviewView";
import { TableView } from "../features/table/TableView";
import { WorkstreamsView } from "../features/workstreams/WorkstreamsView";
import {
  choiceCount,
  type Filters,
  filterOptions,
  isFiltered,
  noChoice,
} from "../model/filters";
import type { Line } from "../model/lines";
import { type PaintMode, paintKey } from "../model/paint";
import { NeedsYouNotice } from "./NeedsYouNotice";
import { formatRoute, type Route } from "./route";
import type { BoardDataState } from "./useBoardData";
import { V1Notice } from "./V1Notice";

export interface ViewOutletProps {
  route: Route;
  data: BoardDataState;
  fetcher: AuthenticatedFetch;
  // answers: the server records answers to agents' questions (RUN-8).
  answers: boolean;
  root: string;
  preferences: Preferences;
  updatePreferences(change: (current: Preferences) => Preferences): void;
  filters: Filters;
  onFilters(filters: Filters): void;
  // visible are the board's cards the filters keep.
  visible: Card[];
  // narrowColumn is the one column a phone shows.
  narrowColumn: string;
  go(next: Partial<Route>): void;
  navigate(route: Route): void;
  onOpen(ticket: TicketRef, tab?: PanelTab): void;
  onMove(card: Card, to: Column, after?: string): void;
  onNewTicket(project: string): void;
  onProjectArchived(name: string, displayName: string): void;
}

// ViewOutlet is the routed view's body under the page header: the board's
// read errors and notices, then the archive, Overview, Workstreams, or the
// Board or Table with their toolbar.
export function ViewOutlet(props: ViewOutletProps) {
  const { projects } = props.data;
  return (
    <>
      {projects.status === "error" ? (
        <p role="alert" className="m-4 text-sm text-danger">
          The board could not be read: {projects.error}
        </p>
      ) : null}
      {projects.status === "ready" && projects.data.v1Projects.length > 0 ? (
        <V1Notice
          projects={projects.data.v1Projects}
          command={projects.data.migrateCommand}
        />
      ) : null}
      {projects.status === "ready" && projects.data.rootMissing ? (
        <EmptyState title="No board here yet">
          Flashheart reads tickets from{" "}
          <code className="font-mono text-ink">{props.root}</code>, which does
          not exist yet. Agents create it when they first record work, or make a
          project folder there with a{" "}
          <code className="font-mono">tickets/</code> directory inside.
        </EmptyState>
      ) : (
        <ViewBody {...props} />
      )}
    </>
  );
}

function ViewBody(props: ViewOutletProps) {
  const { route, data, fetcher, go, onOpen } = props;
  const { projects, summaries, current, editing, revision } = data;
  const ready = projects.status === "ready";
  const toBoard = () => go({ view: "board", ticket: undefined });
  switch (route.view) {
    case "archive":
      if (current)
        return (
          <ArchiveView
            fetcher={fetcher}
            project={current.name}
            revision={revision}
            editing={editing}
            onOpen={onOpen}
            onBack={toBoard}
            onProjectArchived={(displayName) =>
              props.onProjectArchived(current.name, displayName)
            }
          />
        );
      if (ready && route.scope.kind === "all")
        return (
          <ProjectsArchiveView
            fetcher={fetcher}
            revision={revision}
            projects={summaries}
            editing={editing}
            onArchived={props.onProjectArchived}
            onOpenProject={(project) =>
              props.navigate({
                scope: { kind: "project", project },
                view: "board",
              })
            }
            onBack={toBoard}
          />
        );
      if (ready)
        return (
          <EmptyState title="No such project">
            {data.scopeProject} is not on the board. It may have been archived;
            see All projects › Archive.
          </EmptyState>
        );
      return <BoardSkeleton />;
    case "overview":
      if (!ready) return <BoardSkeleton />;
      return (
        <OverviewView
          fetcher={fetcher}
          project={data.scopeProject}
          projects={summaries}
          revision={revision}
          boardHref={formatRoute({ scope: route.scope, view: "board" })}
          onOpen={onOpen}
          onReview={(ticket) => onOpen(ticket, "review")}
          onAnswer={props.answers ? editing.answer : undefined}
          onCreateTicket={summaries.length > 0 ? props.onNewTicket : undefined}
        />
      );
    case "workstreams":
      if (!ready) return <BoardSkeleton />;
      return (
        <WorkstreamsView
          projects={current ? [current] : summaries}
          fetcher={fetcher}
          lines={data.lines}
          revision={revision}
          editing={editing}
          expanded={props.preferences.expandedWorkstreams}
          onExpanded={(project, slugs) =>
            props.updatePreferences((current) => {
              const expandedWorkstreams = { ...current.expandedWorkstreams };
              if (slugs.length > 0) expandedWorkstreams[project] = slugs;
              else delete expandedWorkstreams[project];
              return { ...current, expandedWorkstreams };
            })
          }
          onOpen={onOpen}
        />
      );
    default:
      return <BoardOutlet {...props} />;
  }
}

// BoardOutlet is the Board or the Table: the filter bar with, on the
// Board, its view options and chips, then the scope's tickets.
function BoardOutlet({
  route,
  data,
  preferences,
  updatePreferences,
  filters,
  onFilters,
  visible,
  narrowColumn,
  go,
  onOpen,
  onMove,
}: ViewOutletProps) {
  const { board, cards, summaries, current, lines } = data;
  const { density, colourBy: paint, hiddenColumns } = preferences;
  const projectNames = useMemo(
    () =>
      route.scope.kind === "all"
        ? new Map(
            summaries.map((project) => [project.name, project.displayName]),
          )
        : undefined,
    [route.scope.kind, summaries],
  );
  const options = useMemo(() => {
    const found = filterOptions(cards);
    const titles = new Map(
      summaries.flatMap((project) =>
        project.workstreams.map((line) => [line.slug, line.title] as const),
      ),
    );
    return {
      ...found,
      workstreams: found.workstreams.map((slug) => ({
        value: slug,
        label: titles.get(slug) ?? slug,
      })),
    };
  }, [cards, summaries]);
  // paints are the colour key's values before filtering, so a chip stays to
  // restore once it filters the others away (FH-39).
  const paints = useMemo(
    () => paintKey(cards, paint, new Date()),
    [cards, paint],
  );
  // The age filter has chips only while cards are coloured by age.
  const choosePaint = (value: PaintMode) => {
    updatePreferences((current) => ({ ...current, colourBy: value }));
    if (value !== "age" && choiceCount(filters.age) > 0)
      onFilters({ ...filters, age: noChoice });
  };
  const shown = board.status === "ready" && visible.length > 0;
  return (
    <>
      <FilterBar
        filters={filters}
        options={options}
        onChange={onFilters}
        view={
          route.view === "board"
            ? {
                density,
                onDensity: (value) =>
                  updatePreferences((current) => ({
                    ...current,
                    density: value,
                  })),
                hiddenColumns,
                onHiddenColumns: (hiddenColumns) =>
                  updatePreferences((current) => ({
                    ...current,
                    hiddenColumns,
                  })),
                paint,
                onPaint: choosePaint,
              }
            : undefined
        }
      />
      {route.view === "board" && board.status === "ready" ? (
        <FilterChips
          workstreams={current?.workstreams}
          lines={
            (current && lines.get(current.name)) ?? new Map<string, Line>()
          }
          cards={cards}
          paint={paint}
          paints={paints}
          filters={filters}
          onChange={onFilters}
        />
      ) : null}
      {filters.needsYou ? (
        <NeedsYouNotice
          count={
            data.projects.status === "ready"
              ? data.projects.data.runs.needsYouUnticketed
              : 0
          }
          onOpenOverview={() => go({ view: "overview", ticket: undefined })}
        />
      ) : null}
      {board.status === "loading" ? <BoardSkeleton /> : null}
      {board.status === "error" ? (
        <div className="m-4 flex items-center gap-3 text-sm">
          <p role="alert" className="text-danger">
            This board could not be loaded: {board.error}
          </p>
          <Button onClick={board.reload}>
            <Icon name="refresh" size={14} />
            Try again
          </Button>
        </div>
      ) : null}
      {board.status === "ready" && visible.length === 0 ? (
        <NoTickets filtered={isFiltered(filters)} />
      ) : null}
      {shown && route.view === "board" ? (
        <div className="min-h-0 flex-1" data-narrow-column={narrowColumn}>
          <BoardView
            cards={visible}
            lines={lines}
            workstreams={data.workstreams}
            projectNames={projectNames}
            density={density}
            paint={paint}
            hiddenColumns={hiddenColumns}
            selected={route.ticket}
            doneTotal={board.data.doneTotal}
            doneShown={board.data.doneShown}
            doneAll={data.doneAll}
            onDoneAll={data.setDoneAll}
            onOpen={onOpen}
            onMove={onMove}
          />
        </div>
      ) : null}
      {shown && route.view === "table" ? (
        <div className="min-h-0 flex-1">
          <TableView
            cards={visible}
            lines={lines}
            workstreams={data.workstreams}
            projectNames={projectNames}
            selected={route.ticket}
            onOpen={onOpen}
          />
        </div>
      ) : null}
    </>
  );
}

function BoardSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="grid grid-cols-5 gap-4 px-4 py-2 md:px-6"
    >
      {COLUMNS.map((column) => (
        <div
          key={column.id}
          className="flex flex-col gap-2.5 rounded-panel border border-rule bg-column p-3"
        >
          <div className="h-6 w-2/3 rounded-full bg-well" />
          <div className="h-24 animate-pulse rounded-card bg-card" />
          <div className="h-20 animate-pulse rounded-card bg-card" />
        </div>
      ))}
    </div>
  );
}
