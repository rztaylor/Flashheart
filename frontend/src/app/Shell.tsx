import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { restoreProject } from "../api/archive";
import {
  type Card,
  COLUMNS,
  type Column,
  fetchAllBoard,
  fetchProjectBoard,
  fetchProjects,
  type TicketRef,
} from "../api/board";
import type { Created } from "../api/edit";
import type { ThemePreference } from "../api/info";
import type { Preferences } from "../api/preferences";
import { AsideProvider } from "../components/Aside";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { SearchField, SelectField } from "../components/Field";
import { Icon, type IconName } from "../components/Icon";
import { RunStateMark } from "../components/RunState";
import { Toast, type ToastMessage } from "../components/Toast";
import { AgentsView } from "../features/agents/AgentsView";
import { ArchiveView } from "../features/archive/ArchiveView";
import { ProjectsArchiveView } from "../features/archive/ProjectsArchiveView";
import { BoardView, NoTickets } from "../features/board/BoardView";
import { CardPanel } from "../features/card/CardPanel";
import { TicketPage } from "../features/card/TicketPage";
import { BlockedMoveDialog } from "../features/editing/BlockedMoveDialog";
import { NewTicketDialog } from "../features/editing/NewTicketDialog";
import { type Movable, useEditing } from "../features/editing/useEditing";
import { FilterBar } from "../features/filters/FilterBar";
import { FilterChips } from "../features/filters/FilterChips";
import { ProjectRail } from "../features/projects/ProjectRail";
import { TableView } from "../features/table/TableView";
import { WorkstreamsView } from "../features/workstreams/WorkstreamsView";
import type { SingleserveLifecycle } from "../lifecycle/useSingleserve";
import {
  placeVirtual,
  shownColumns,
  shownVirtual as shownVirtualColumns,
} from "../model/columns";
import {
  applyFilters,
  choiceCount,
  emptyFilters,
  type Filters,
  filterOptions,
  isFiltered,
  noChoice,
} from "../model/filters";
import { type Line, linesByProject } from "../model/lines";
import {
  afterForStep,
  placeCard,
  predecessor,
  type Step,
} from "../model/order";
import { type PaintMode, paintKey } from "../model/paint";
import { completesWorkstream, nextRemark, progressDue } from "../model/remarks";
import { filtersFor, rememberScope, sameScope } from "../model/scopes";
import { viewSummary } from "../model/summary";
import { useResource } from "../state/useResource";
import { useRevision } from "../state/useRevision";
import { BackendStatus } from "./BackendStatus";
import { PageHeader } from "./PageHeader";
import {
  formatRoute,
  type Route,
  type Scope,
  useRoute,
  type View,
} from "./route";
import type { ServerInfoState } from "./useServerInfo";

type BoardData = { cards: Card[]; doneTotal: number; doneShown: number };

interface ShellProps {
  lifecycle: SingleserveLifecycle;
  info: ServerInfoState;
  preferences: Preferences;
  preferencesLoaded: boolean;
  updatePreferences(change: (current: Preferences) => Preferences): void;
}

const views: {
  id: Exclude<View, "archive" | "ticket">;
  label: string;
  icon: IconName;
}[] = [
  { id: "board", label: "Board", icon: "board" },
  { id: "agents", label: "Agents", icon: "agents" },
  { id: "workstreams", label: "Workstreams", icon: "lines" },
  { id: "table", label: "Table", icon: "table" },
];

// Shell composes the signage band, the project rail, the filter bar, the
// routed view and the card panel, or under the band a ticket's full page.
export function Shell({
  lifecycle,
  info,
  preferences,
  preferencesLoaded,
  updatePreferences,
}: ShellProps) {
  const { state, fetch: fetcher, ready } = lifecycle;
  const [route, navigate] = useRoute();
  const [filters, setFiltersState] = useState<Filters>(emptyFilters);
  const { density, colourBy: paint } = preferences;
  const [newTicket, setNewTicket] = useState(false);
  const [doneAll, setDoneAll] = useState(false);
  const [narrowColumn, setNarrowColumn] = useState<string>("in-progress");
  const opener = useRef<HTMLElement | null>(null);
  const stopping = state.phase === "stopping";

  const loadProjects = useCallback(
    (signal: AbortSignal) => fetchProjects(fetcher, signal),
    [fetcher],
  );
  // Live updates: every view reloads when the board revision moves (LIFE-3).
  const revision = useRevision(fetcher, ready);
  const projects = useResource(
    ready ? loadProjects : undefined,
    "projects",
    revision,
  );

  const scopeKey =
    route.scope.kind === "all" ? "all" : `p:${route.scope.project}`;
  const scopeProject =
    route.scope.kind === "project" ? route.scope.project : "";
  const loadBoard = useCallback(
    (signal: AbortSignal): Promise<BoardData> =>
      scopeProject
        ? fetchProjectBoard(fetcher, scopeProject, doneAll, signal)
        : fetchAllBoard(fetcher, doneAll, signal),
    [fetcher, scopeProject, doneAll],
  );
  const board = useResource(
    ready && (route.view === "board" || route.view === "table")
      ? loadBoard
      : undefined,
    `${scopeKey}:${doneAll}`,
    revision,
  );
  const reloadAll = useCallback(() => {
    projects.reload();
    board.reload();
  }, [projects.reload, board.reload]);
  // A move to Done may earn a remark in its toast (FH-22): completion only
  // when it finishes a workstream, otherwise progress now and then.
  const cardsForRemarks = useRef<Card[]>([]);
  const lastProgress = useRef<number | undefined>(undefined);
  const remark = useCallback(
    (ticket: Movable, to: Column): ToastMessage["aside"] => {
      if (to !== "done" || ticket.column === "done") return undefined;
      const cards = cardsForRemarks.current;
      const card = cards.find((item) => item.id === ticket.id);
      if (
        card &&
        completesWorkstream(cards, { ...card, column: ticket.column }, to)
      )
        return { placement: "completion", text: nextRemark("completion").text };
      const now = Date.now();
      if (!progressDue(lastProgress.current, now)) return undefined;
      lastProgress.current = now;
      return { placement: "progress", text: nextRemark("progress").text };
    },
    [],
  );
  const editing = useEditing(fetcher, reloadAll, remark);
  const [brand] = useState(() => nextRemark("brand").text);
  const { reconcile } = editing;

  // The view and filters are remembered per project (VIEW-7, CFG-2).
  const scopeKeyName = route.scope.kind === "all" ? "all" : route.scope.project;
  const saved = preferences.scopes[scopeKeyName];
  // biome-ignore lint/correctness/useExhaustiveDependencies: restore only when the scope changes or preferences first load.
  useEffect(() => {
    if (preferencesLoaded)
      setFiltersState((current) => filtersFor(saved, current.query));
  }, [scopeKeyName, preferencesLoaded]);
  // The archive and a ticket's full page are side trips, never the
  // remembered view.
  const remember = (
    nextFilters: Filters,
    view: Exclude<View, "archive" | "ticket">,
  ) => {
    const entry = rememberScope(nextFilters, view);
    updatePreferences((current) =>
      sameScope(current.scopes[scopeKeyName], entry)
        ? current
        : { ...current, scopes: { ...current.scopes, [scopeKeyName]: entry } },
    );
  };
  const setFilters = (next: Filters) => {
    setFiltersState(next);
    if (route.view !== "archive" && route.view !== "ticket")
      remember(next, route.view);
  };

  const summaries = projects.status === "ready" ? projects.data.projects : [];
  const lines = useMemo(
    () =>
      linesByProject(
        summaries.map((project) => ({
          project: project.name,
          workstreams: project.workstreams,
        })),
      ),
    [summaries],
  );
  const workstreams = useMemo(
    () =>
      new Map(summaries.map((project) => [project.name, project.workstreams])),
    [summaries],
  );
  const keys = useMemo(
    () => new Set(summaries.map((project) => project.key)),
    [summaries],
  );
  const current = summaries.find((project) => project.name === scopeProject);
  const v1Projects =
    projects.status === "ready" ? projects.data.v1Projects : [];
  // Needs you is visible from every view and project (SPEC §7).
  const needsYou =
    projects.status === "ready" ? projects.data.runs.needsYou : 0;
  const shownVirtual = preferences.virtualColumns;
  const hiddenColumns = preferences.hiddenColumns;
  const projectNames = useMemo(
    () =>
      route.scope.kind === "all"
        ? new Map(
            summaries.map((project) => [project.name, project.displayName]),
          )
        : undefined,
    [route.scope.kind, summaries],
  );

  const loadedCards = board.status === "ready" ? board.data.cards : undefined;
  useEffect(() => {
    if (loadedCards) reconcile(loadedCards);
  }, [loadedCards, reconcile]);
  // Moves show in their new column and place at once, before the save
  // returns.
  const allCards: Card[] = useMemo(
    () =>
      Object.entries(editing.pending).reduce(
        (cards, [id, entry]) => placeCard(cards, id, entry.column, entry.after),
        loadedCards ?? [],
      ),
    [loadedCards, editing.pending],
  );
  cardsForRemarks.current = allCards;
  const visible = useMemo(
    () => applyFilters(allCards, filters),
    [allCards, filters],
  );
  const options = useMemo(() => {
    const found = filterOptions(allCards);
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
  }, [allCards, summaries]);
  // paints are the colour key's values before filtering, so a chip stays to
  // restore once it filters the others away (FH-39).
  const paints = useMemo(
    () => paintKey(allCards, paint, new Date()),
    [allCards, paint],
  );
  // The age filter has chips only while cards are coloured by age.
  const choosePaint = (value: PaintMode) => {
    updatePreferences((current) => ({ ...current, colourBy: value }));
    if (value !== "age" && choiceCount(filters.age) > 0)
      setFilters({ ...filters, age: noChoice });
  };
  // place moves a ticket (to a place in a column when after is given;
  // EDIT-9), remembering where it was for Undo.
  const place = (card: Card, to: Column, after?: string) =>
    void editing.move(
      card,
      to,
      "",
      after === undefined
        ? {}
        : { after, undoAfter: predecessor(allCards, card) },
    );
  // stepTicket is the panel's Position buttons: a step among the visible cards
  // of the ticket's column, offered only while the board shows the ticket.
  const shownTicket = allCards.find((card) => card.id === route.ticket?.id);
  const stepTicket =
    shownTicket &&
    shownTicket.column !== "done" &&
    visible.some((card) => card.id === shownTicket.id)
      ? (step: Step) => {
          const after = afterForStep(visible, shownTicket, step);
          if (after !== null) place(shownTicket, shownTicket.column, after);
        }
      : undefined;
  // The phone column picker falls back to In progress when the virtual
  // column it showed has emptied and gone, or its column was hidden.
  const narrowVirtual = shownVirtualColumns(visible, shownVirtual);
  const narrowReal = shownColumns(hiddenColumns);
  const narrowShown =
    narrowReal.some((column) => column.id === narrowColumn) ||
    narrowVirtual.some((column) => column.id === narrowColumn)
      ? narrowColumn
      : "in-progress";

  const go = (next: Partial<Route>) => navigate({ ...route, ...next });
  const selectScope = (scope: Scope) => {
    setDoneAll(false);
    const name = scope.kind === "all" ? "all" : scope.project;
    const view = preferences.scopes[name]?.view || route.view;
    go({ scope, view, ticket: undefined });
  };
  const openTicket = (ticket: TicketRef) => {
    // Remember what opened the panel (a card, row or station) so Escape can
    // return focus there; links inside the panel keep the original opener.
    const active = document.activeElement;
    if (active instanceof HTMLElement && !active.closest("aside, dialog")) {
      opener.current = active;
    }
    go({ ticket });
  };
  const closeTicket = useCallback(() => {
    navigate({ ...route, ticket: undefined });
    const element = opener.current;
    const id = route.ticket?.id;
    opener.current = null;
    window.setTimeout(() => {
      // Return to what opened the panel, or else to the ticket's own card.
      if (element?.isConnected) element.focus();
      else if (id)
        document
          .querySelector<HTMLElement>(
            `[data-ticket="${CSS.escape(id)}"]:not([data-mirrored])`,
          )
          ?.focus();
    }, 0);
  }, [navigate, route]);
  // projectArchived leaves an archived project's scope for the projects
  // archive, with Undo (PRJ-5).
  const projectArchived = (name: string, displayName: string) => {
    navigate({ scope: { kind: "all" }, view: "archive" });
    reloadAll();
    editing.notify({
      text: `Archived project ${displayName}.`,
      action: {
        label: "Undo",
        run: () =>
          void restoreProject(fetcher, name)
            .then(reloadAll)
            .catch((error: unknown) =>
              editing.notify({
                text: `${displayName} was not restored: ${
                  error instanceof Error ? error.message : "the request failed"
                }`,
              }),
            ),
      },
    });
  };
  const workstreamTitle = (project: string, slug: string) =>
    workstreams.get(project)?.find((workstream) => workstream.slug === slug)
      ?.title ?? slug;

  const scopeName =
    route.scope.kind === "all"
      ? "All projects"
      : (current?.displayName ?? scopeProject);
  const rootMissing = projects.status === "ready" && projects.data.rootMissing;
  // A read-only server shows agents' questions without an answer form.
  const answers = info.status === "ready" && info.info.answers;
  const root =
    info.status === "ready"
      ? info.info.root
      : projects.status === "ready"
        ? projects.data.root
        : "";

  const summary = viewSummary({
    view: route.view,
    board:
      board.status === "ready"
        ? { shown: visible.length, total: allCards.length }
        : undefined,
    filtered: isFiltered(filters),
    runs: current
      ? current.runs
      : projects.status === "ready"
        ? projects.data.runs
        : undefined,
    workstreams: (current ? [current] : summaries).flatMap(
      (project) => project.workstreams,
    ),
    archived: current
      ? { count: current.archived, of: "tickets" }
      : projects.status === "ready" && route.scope.kind === "all"
        ? { count: projects.data.archivedProjects, of: "projects" }
        : undefined,
  });

  return (
    <AsideProvider>
      <div className="grid h-full grid-cols-[minmax(0,1fr)] grid-rows-[3.5rem_auto_1fr]">
        <header className="relative flex min-w-0 items-center gap-1.5 overflow-hidden bg-band px-3 text-on-band sm:gap-6 sm:px-5">
          <span className="flex items-center gap-2">
            <Bolt />
            <span className="wordmark text-xl max-sm:sr-only" title={brand}>
              Flashheart
            </span>
          </span>
          <nav
            aria-label="Views"
            className="flex items-center gap-0.5 sm:gap-1"
          >
            {views.map((view) => {
              const active = route.view === view.id;
              return (
                <a
                  key={view.id}
                  href={`#${view.id}`}
                  aria-current={active ? "page" : undefined}
                  onClick={(event) => {
                    event.preventDefault();
                    // From a ticket's full page the band leaves the ticket.
                    go(
                      route.view === "ticket"
                        ? { view: view.id, ticket: undefined }
                        : { view: view.id },
                    );
                    remember(filters, view.id);
                  }}
                  className={`flex h-9 items-center gap-2 rounded-control px-2 text-sm font-semibold transition-colors focus-visible:outline-on-band sm:px-3 ${
                    active
                      ? "bg-band-tab-active text-on-band-tab-active shadow-card"
                      : "text-on-band-muted hover:bg-band-field hover:text-on-band"
                  }`}
                >
                  <Icon name={view.icon} size={16} />
                  <span className="max-sm:sr-only">{view.label}</span>
                </a>
              );
            })}
          </nav>
          <div className="ml-auto flex shrink-0 items-center gap-1 sm:gap-3">
            {needsYou > 0 ? (
              <button
                type="button"
                onClick={() => {
                  go({
                    scope: { kind: "all" },
                    view: "agents",
                    ticket: undefined,
                  });
                }}
                className="flex h-8 items-center gap-1.5 rounded-full bg-attention px-2.5 text-xs font-semibold whitespace-nowrap sm:h-9 sm:px-3.5 sm:text-sm text-on-attention transition-opacity hover:opacity-90 focus-visible:outline-on-band"
              >
                <RunStateMark state="needs-you" size={11} />
                {needsYou}
                <span className="hidden sm:inline">
                  {needsYou === 1 ? " needs you" : " need you"}
                </span>
                <span className="sm:hidden">
                  {needsYou === 1 ? " agent needs you" : " agents need you"}
                </span>
              </button>
            ) : null}
            {route.view === "board" || route.view === "table" ? (
              <div className="hidden md:flex">
                <SearchField
                  label="Search tickets"
                  placeholder="Search id, title, text, tags"
                  value={filters.query}
                  onChange={(query) => setFilters({ ...filters, query })}
                />
              </div>
            ) : null}
            <BackendStatus
              state={state}
              checking={lifecycle.checking}
              onCheck={() => void lifecycle.checkHealth()}
            />
            <Button
              variant="band"
              className="h-9 py-0 max-sm:px-2"
              onClick={() => void lifecycle.quit()}
              disabled={!ready || stopping}
            >
              <Icon name="power" size={14} />
              <span className="max-sm:sr-only">
                {stopping ? "Quitting…" : "Quit"}
              </span>
            </Button>
          </div>
        </header>

        <div>
          {state.shutdownDenied ? (
            <div
              role="alert"
              className="flex items-center justify-between gap-4 border-b border-rule bg-danger-surface px-4 py-2 text-sm"
            >
              <span>
                <strong className="font-semibold">
                  Flashheart did not quit.
                </strong>{" "}
                {state.shutdownDenied}
              </span>
              <Button variant="quiet" onClick={lifecycle.dismissDenial}>
                Dismiss
              </Button>
            </div>
          ) : null}
          {board.status === "ready" && board.error ? (
            <p
              role="status"
              className="border-b border-rule bg-well px-4 py-1.5 text-xs text-ink-muted"
            >
              Showing the last good copy: {board.error}
            </p>
          ) : null}
        </div>

        {route.view === "ticket" && route.ticket ? (
          <main
            className="min-h-0 overflow-y-auto"
            aria-label={`Ticket ${route.ticket.id}`}
          >
            <TicketPage
              key={route.ticket.id}
              ticket={route.ticket}
              fetcher={fetcher}
              lines={lines}
              workstreamTitle={workstreamTitle}
              keys={keys}
              onOpen={(ticket) =>
                navigate({ scope: { kind: "all" }, view: "ticket", ticket })
              }
              onClose={() =>
                navigate({ scope: { kind: "all" }, view: "board" })
              }
              revision={revision}
              editing={editing}
              answers={answers}
              workstreamsOf={(project) => workstreams.get(project) ?? []}
              boardLink={(project) => ({
                href: formatRoute({
                  scope: { kind: "project", project },
                  view: "board",
                  ticket: route.ticket,
                }),
                label:
                  summaries.find((item) => item.name === project)
                    ?.displayName ?? project,
              })}
            />
          </main>
        ) : (
          // Below 1440px an open panel shrinks the rail to its key badges so
          // the board keeps three whole columns.
          <div
            data-panel={route.ticket ? "open" : undefined}
            className="group/work grid min-h-0 grid-cols-1 md:grid-cols-[15.5rem_minmax(0,1fr)_auto] md:max-[90rem]:data-[panel=open]:grid-cols-[4rem_minmax(0,1fr)_auto]"
          >
            <div className="hidden min-h-0 flex-col border-r border-rail-rule bg-rail md:flex">
              <div className="min-h-0 flex-1">
                <ProjectRail
                  projects={summaries}
                  runs={
                    projects.status === "ready" ? projects.data.runs : undefined
                  }
                  scope={route.scope}
                  onSelect={selectScope}
                />
              </div>
              <RailFooter
                root={root}
                info={info}
                theme={preferences.theme}
                onTheme={(theme) =>
                  updatePreferences((current) => ({ ...current, theme }))
                }
              />
            </div>

            <main
              className="flex min-h-0 min-w-0 flex-col"
              aria-label={`${scopeName} ${route.view}`}
            >
              <PageHeader
                projectKey={current?.key}
                title={scopeName}
                summary={summary}
                live={route.view === "board" || route.view === "table"}
                aside={
                  <>
                    {route.view !== "archive" &&
                    projects.status === "ready" &&
                    (current || route.scope.kind === "all") ? (
                      <a
                        href={formatRoute({
                          ...route,
                          view: "archive",
                          ticket: undefined,
                        })}
                        onClick={(event) => {
                          event.preventDefault();
                          go({ view: "archive", ticket: undefined });
                        }}
                        className="flex items-center gap-1.5 rounded-control px-2 py-1 text-xs text-ink-muted transition-colors hover:bg-well hover:text-ink"
                      >
                        <Icon name="archive" size={14} />
                        Archive
                        <span className="tabular-nums">
                          {current
                            ? current.archived
                            : projects.data.archivedProjects}
                        </span>
                      </a>
                    ) : null}
                    {(route.view === "board" || route.view === "table") &&
                    summaries.length > 0 ? (
                      <Button
                        variant="primary"
                        className="h-9 py-0"
                        onClick={() => setNewTicket(true)}
                      >
                        <Icon name="plus" size={16} />
                        New ticket
                      </Button>
                    ) : null}
                  </>
                }
              />
              <div className="flex flex-wrap items-center gap-3 border-b border-rule px-4 py-2 md:hidden">
                {route.view === "board" || route.view === "table" ? (
                  <div className="flex w-full">
                    <SearchField
                      tone="plain"
                      label="Search tickets"
                      placeholder="Search tickets"
                      value={filters.query}
                      onChange={(query) => setFilters({ ...filters, query })}
                    />
                  </div>
                ) : null}
                <SelectField
                  label="Project"
                  value={scopeProject}
                  onChange={(project) =>
                    selectScope(
                      project ? { kind: "project", project } : { kind: "all" },
                    )
                  }
                >
                  <option value="">All projects</option>
                  {summaries.map((project) => (
                    <option key={project.name} value={project.name}>
                      {project.displayName}
                    </option>
                  ))}
                </SelectField>
                {route.view === "board" ? (
                  <SelectField
                    label="Column"
                    value={narrowShown}
                    onChange={setNarrowColumn}
                  >
                    {placeVirtual<{ id: string; title: string }>(
                      narrowReal,
                      narrowVirtual,
                    ).map((column) => (
                      <option key={column.id} value={column.id}>
                        {column.title}
                      </option>
                    ))}
                  </SelectField>
                ) : null}
              </div>

              {projects.status === "error" ? (
                <p role="alert" className="m-4 text-sm text-danger">
                  The board could not be read: {projects.error}
                </p>
              ) : null}

              {v1Projects.length > 0 && projects.status === "ready" ? (
                <V1Notice
                  projects={v1Projects}
                  command={projects.data.migrateCommand}
                />
              ) : null}

              {rootMissing ? (
                <EmptyState title="No board here yet">
                  Flashheart reads tickets from{" "}
                  <code className="font-mono text-ink">{root}</code>, which does
                  not exist yet. Agents create it when they first record work,
                  or make a project folder there with a{" "}
                  <code className="font-mono">tickets/</code> directory inside.
                </EmptyState>
              ) : route.view === "archive" ? (
                current ? (
                  <ArchiveView
                    fetcher={fetcher}
                    project={current.name}
                    revision={revision}
                    editing={editing}
                    onOpen={openTicket}
                    onBack={() => go({ view: "board", ticket: undefined })}
                    onProjectArchived={(displayName) =>
                      projectArchived(current.name, displayName)
                    }
                  />
                ) : projects.status === "ready" &&
                  route.scope.kind === "all" ? (
                  <ProjectsArchiveView
                    fetcher={fetcher}
                    revision={revision}
                    projects={summaries}
                    editing={editing}
                    onArchived={projectArchived}
                    onOpenProject={(project) =>
                      navigate({
                        scope: { kind: "project", project },
                        view: "board",
                      })
                    }
                    onBack={() => go({ view: "board", ticket: undefined })}
                  />
                ) : projects.status === "ready" ? (
                  <EmptyState title="No such project">
                    {scopeProject} is not on the board. It may have been
                    archived; see All projects › Archive.
                  </EmptyState>
                ) : (
                  <BoardSkeleton />
                )
              ) : route.view === "agents" ? (
                projects.status === "ready" ? (
                  <AgentsView
                    fetcher={fetcher}
                    project={scopeProject}
                    projects={summaries}
                    revision={revision}
                    onOpen={openTicket}
                    onAnswer={answers ? editing.answer : undefined}
                  />
                ) : (
                  <BoardSkeleton />
                )
              ) : route.view === "workstreams" ? (
                projects.status === "ready" ? (
                  <WorkstreamsView
                    projects={current ? [current] : summaries}
                    fetcher={fetcher}
                    lines={lines}
                    revision={revision}
                    editing={editing}
                    onOpen={openTicket}
                  />
                ) : (
                  <BoardSkeleton />
                )
              ) : (
                <>
                  <FilterBar
                    filters={filters}
                    options={options}
                    onChange={setFilters}
                    view={
                      route.view === "board"
                        ? {
                            density,
                            onDensity: (value) =>
                              updatePreferences((current) => ({
                                ...current,
                                density: value,
                              })),
                            virtualColumns: shownVirtual,
                            onVirtualColumns: (virtualColumns) =>
                              updatePreferences((current) => ({
                                ...current,
                                virtualColumns,
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
                        (current && lines.get(current.name)) ??
                        new Map<string, Line>()
                      }
                      cards={allCards}
                      paint={paint}
                      paints={paints}
                      filters={filters}
                      onChange={setFilters}
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
                  {board.status === "ready" &&
                  visible.length > 0 &&
                  route.view === "board" ? (
                    <div
                      className="min-h-0 flex-1"
                      data-narrow-column={narrowShown}
                    >
                      <BoardView
                        cards={visible}
                        lines={lines}
                        workstreams={workstreams}
                        projectNames={projectNames}
                        density={density}
                        paint={paint}
                        virtualColumns={shownVirtual}
                        hiddenColumns={hiddenColumns}
                        selected={route.ticket}
                        doneTotal={board.data.doneTotal}
                        doneShown={board.data.doneShown}
                        doneAll={doneAll}
                        onDoneAll={setDoneAll}
                        onOpen={openTicket}
                        onMove={place}
                      />
                    </div>
                  ) : null}
                  {board.status === "ready" &&
                  visible.length > 0 &&
                  route.view === "table" ? (
                    <div className="min-h-0 flex-1">
                      <TableView
                        cards={visible}
                        lines={lines}
                        workstreams={workstreams}
                        projectNames={projectNames}
                        selected={route.ticket}
                        onOpen={openTicket}
                      />
                    </div>
                  ) : null}
                </>
              )}
            </main>

            {route.ticket ? (
              <CardPanel
                key={route.ticket.id}
                ticket={route.ticket}
                fetcher={fetcher}
                lines={lines}
                workstreamTitle={workstreamTitle}
                keys={keys}
                onOpen={openTicket}
                onClose={closeTicket}
                revision={revision}
                editing={editing}
                onStep={stepTicket}
                answers={answers}
                workstreamsOf={(project) => workstreams.get(project) ?? []}
              />
            ) : null}
          </div>
        )}
        {editing.blocked ? (
          <BlockedMoveDialog
            move={editing.blocked}
            onConfirm={editing.confirmBlocked}
            onCancel={editing.cancelBlocked}
          />
        ) : null}
        {newTicket ? (
          <NewTicketDialog
            fetcher={fetcher}
            projects={summaries}
            project={scopeProject}
            onClose={() => setNewTicket(false)}
            onCreated={(created: Created) => {
              setNewTicket(false);
              reloadAll();
              editing.notify({ text: `Created ${created.id}.` });
              openTicket({ id: created.id });
            }}
          />
        ) : null}
        <Toast message={editing.toast} onDismiss={editing.dismiss} />
      </div>
    </AsideProvider>
  );
}

// Bolt is the brand mark beside the wordmark: the one flash of signal yellow.
function Bolt() {
  return (
    <svg
      viewBox="0 0 16 20"
      width={13}
      height={16}
      aria-hidden="true"
      className="shrink-0 text-flash"
    >
      <path d="M10 0 1 11.5h6L5.5 20 15 7.5H8.8L10 0Z" fill="currentColor" />
    </svg>
  );
}

// Board format v1 projects are not shown until they are migrated (MIG-1).
function V1Notice({
  projects,
  command,
}: {
  projects: string[];
  command: string;
}) {
  const names = projects.join(", ");
  return (
    <section
      aria-label="Older board format"
      className="mx-4 mt-1 mb-3 rounded-card border border-rule bg-well px-4 py-3 text-sm text-ink-muted md:mx-6"
    >
      <p>
        {projects.length === 1 ? "The project " : "The projects "}
        <span className="text-ink">{names}</span>{" "}
        {projects.length === 1 ? "uses" : "use"} the older board format and{" "}
        {projects.length === 1 ? "is" : "are"} hidden until migrated. Preview
        the change, then add <code className="font-mono">--write</code> to apply
        it:
      </p>
      <pre className="mt-2 overflow-x-auto font-mono text-xs text-ink">
        {command}
      </pre>
    </section>
  );
}

const themes: { value: ThemePreference; label: string }[] = [
  { value: "system", label: "System" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

// RailFooter shows the board root and version, and the theme choice, saved
// with the other preferences (CFG-2).
function RailFooter({
  root,
  info,
  theme,
  onTheme,
}: {
  root: string;
  info: ServerInfoState;
  theme: ThemePreference;
  onTheme(theme: ThemePreference): void;
}) {
  return (
    <section
      aria-label="Board root"
      className="border-t border-rail-rule px-4 py-3 text-2xs text-on-rail-muted max-[90rem]:group-data-[panel=open]/work:hidden"
    >
      <fieldset className="mb-2.5 flex items-center gap-2">
        <legend className="sr-only">Theme</legend>
        <span aria-hidden="true">Theme</span>
        <span className="flex rounded-control border border-rail-rule p-0.5">
          {themes.map((option) => (
            <label
              key={option.value}
              className={`cursor-pointer rounded-inner px-2 py-0.5 text-xs transition-colors has-focus-visible:outline-2 has-focus-visible:outline-focus ${
                theme === option.value
                  ? "bg-rail-active font-semibold text-on-rail-active shadow-card"
                  : "text-on-rail-muted hover:text-on-rail"
              }`}
            >
              <input
                type="radio"
                name="theme"
                value={option.value}
                checked={theme === option.value}
                onChange={() => onTheme(option.value)}
                className="sr-only"
              />
              {option.label}
            </label>
          ))}
        </span>
      </fieldset>
      <dl>
        <dt className="sr-only">Board root</dt>
        {/* Truncated from the start so the meaningful tail stays visible. */}
        <dd
          className="truncate text-left font-mono [direction:rtl]"
          title={root}
        >
          <bdi>{root}</bdi>
        </dd>
        {info.status === "ready" ? (
          <>
            <dt className="sr-only">Version</dt>
            <dd className="mt-0.5">
              {info.info.version} · agent protocol {info.info.protocolVersion}
            </dd>
          </>
        ) : null}
      </dl>
    </section>
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
