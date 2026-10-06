import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  type Card,
  COLUMNS,
  fetchAllBoard,
  fetchProjectBoard,
  fetchProjects,
  type TicketRef,
} from "../api/board";
import type { Created } from "../api/edit";
import type { ThemePreference } from "../api/info";
import type { Preferences } from "../api/preferences";
import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { SearchField, SelectField } from "../components/Field";
import { Icon, type IconName } from "../components/Icon";
import { RunStateMark } from "../components/RunState";
import { Toast } from "../components/Toast";
import { AgentsView } from "../features/agents/AgentsView";
import { BoardView, NoTickets } from "../features/board/BoardView";
import { CardPanel } from "../features/card/CardPanel";
import { BlockedMoveDialog } from "../features/editing/BlockedMoveDialog";
import { NewTicketDialog } from "../features/editing/NewTicketDialog";
import { useEditing } from "../features/editing/useEditing";
import { FilterBar } from "../features/filters/FilterBar";
import { ProjectRail } from "../features/projects/ProjectRail";
import { TableView } from "../features/table/TableView";
import { WorkstreamsView } from "../features/workstreams/WorkstreamsView";
import type { SingleserveLifecycle } from "../lifecycle/useSingleserve";
import { shownVirtual as shownVirtualColumns } from "../model/columns";
import {
  applyFilters,
  emptyFilters,
  type Filters,
  filterOptions,
  isFiltered,
} from "../model/filters";
import { linesByProject } from "../model/lines";
import { filtersFor, rememberScope, sameScope } from "../model/scopes";
import { useResource } from "../state/useResource";
import { useRevision } from "../state/useRevision";
import { BackendStatus } from "./BackendStatus";
import { type Route, type Scope, useRoute, type View } from "./route";
import type { ServerInfoState } from "./useServerInfo";

type BoardData = { cards: Card[]; doneTotal: number; doneShown: number };

interface ShellProps {
  lifecycle: SingleserveLifecycle;
  info: ServerInfoState;
  preferences: Preferences;
  preferencesLoaded: boolean;
  updatePreferences(change: (current: Preferences) => Preferences): void;
}

const views: { id: View; label: string; icon: IconName }[] = [
  { id: "board", label: "Board", icon: "board" },
  { id: "agents", label: "Agents", icon: "agents" },
  { id: "workstreams", label: "Workstreams", icon: "lines" },
  { id: "table", label: "Table", icon: "table" },
];

// Shell composes the signage band, the project rail, the filter bar, the
// routed view and the card panel.
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
    ready && route.view !== "workstreams" && route.view !== "agents"
      ? loadBoard
      : undefined,
    `${scopeKey}:${doneAll}`,
    revision,
  );
  const reloadAll = useCallback(() => {
    projects.reload();
    board.reload();
  }, [projects.reload, board.reload]);
  const editing = useEditing(fetcher, reloadAll);
  const { reconcile } = editing;

  // The view and filters are remembered per project (VIEW-7, CFG-2).
  const scopeKeyName = route.scope.kind === "all" ? "all" : route.scope.project;
  const saved = preferences.scopes[scopeKeyName];
  // biome-ignore lint/correctness/useExhaustiveDependencies: restore only when the scope changes or preferences first load.
  useEffect(() => {
    if (preferencesLoaded)
      setFiltersState((current) => filtersFor(saved, current.query));
  }, [scopeKeyName, preferencesLoaded]);
  const remember = (nextFilters: Filters, view: View) => {
    const entry = rememberScope(nextFilters, view);
    updatePreferences((current) =>
      sameScope(current.scopes[scopeKeyName], entry)
        ? current
        : { ...current, scopes: { ...current.scopes, [scopeKeyName]: entry } },
    );
  };
  const setFilters = (next: Filters) => {
    setFiltersState(next);
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
  // Moves show in their new column at once, before the save returns.
  const allCards: Card[] = useMemo(
    () =>
      (loadedCards ?? []).map((card) => {
        const column = editing.pending[card.id];
        return column ? { ...card, column } : card;
      }),
    [loadedCards, editing.pending],
  );
  const visible = useMemo(
    () => applyFilters(allCards, filters),
    [allCards, filters],
  );
  const options = useMemo(() => filterOptions(allCards), [allCards]);
  // The phone column picker falls back to In progress when the virtual
  // column it showed has emptied and gone.
  const narrowVirtual = shownVirtualColumns(visible, shownVirtual);
  const narrowShown =
    COLUMNS.some((column) => column.id === narrowColumn) ||
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
  const workstreamTitle = (project: string, slug: string) =>
    workstreams.get(project)?.find((workstream) => workstream.slug === slug)
      ?.title ?? slug;

  const scopeName =
    route.scope.kind === "all"
      ? "All projects"
      : (current?.displayName ?? scopeProject);
  const rootMissing = projects.status === "ready" && projects.data.rootMissing;
  const root =
    info.status === "ready"
      ? info.info.root
      : projects.status === "ready"
        ? projects.data.root
        : "";

  return (
    <div className="grid h-full grid-cols-[minmax(0,1fr)] grid-rows-[3rem_auto_1fr]">
      <header className="flex min-w-0 items-center gap-2 overflow-hidden bg-band px-3 text-on-band sm:gap-4 sm:px-4">
        <span className="flex items-center gap-1.5">
          <Bolt />
          <span className="wordmark text-md max-sm:sr-only">Flashheart</span>
        </span>
        <span aria-hidden="true" className="h-5 w-px bg-on-band-muted/40" />
        <h1
          className="flex min-w-0 max-w-64 items-center gap-2 text-sm leading-tight station-sign max-sm:sr-only"
          title={scopeName}
        >
          {current ? (
            <span className="rounded-[3px] bg-on-band px-1 py-px text-2xs leading-4 text-band">
              {current.key}
            </span>
          ) : null}
          <span className="truncate border-t-2 border-on-band pt-0.5">
            {scopeName}
          </span>
        </h1>
        <nav aria-label="Views" className="flex h-full items-stretch sm:ml-2">
          {views.map((view) => {
            const active = route.view === view.id;
            return (
              <a
                key={view.id}
                href={`#${view.id}`}
                aria-current={active ? "page" : undefined}
                onClick={(event) => {
                  event.preventDefault();
                  go({ view: view.id });
                  remember(filters, view.id);
                }}
                className={`flex items-center gap-1.5 border-b-3 px-2 pt-[3px] text-sm sm:px-3 transition-colors focus-visible:-outline-offset-2 focus-visible:outline-on-band ${
                  active
                    ? "border-on-band text-on-band"
                    : "border-transparent text-on-band-muted hover:text-on-band"
                }`}
              >
                <Icon name={view.icon} size={15} />
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
              className="flex h-7 items-center gap-1.5 rounded-[3px] bg-on-band px-2 text-xs font-semibold whitespace-nowrap text-band transition-opacity hover:opacity-90 focus-visible:outline-on-band"
            >
              <RunStateMark state="needs-you" size={10} />
              {needsYou}
              <span className="hidden sm:inline">
                {needsYou === 1 ? " needs you" : " need you"}
              </span>
              <span className="sm:hidden">
                {needsYou === 1 ? " agent needs you" : " agents need you"}
              </span>
            </button>
          ) : null}
          {route.view !== "workstreams" && route.view !== "agents" ? (
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
            className="h-8 py-0"
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

      {/* Below 1440px an open panel shrinks the rail to its key badges so the
          board keeps three whole columns. */}
      <div
        data-panel={route.ticket ? "open" : undefined}
        className="group/work grid min-h-0 grid-cols-1 md:grid-cols-[15.5rem_minmax(0,1fr)_auto] md:max-[90rem]:data-[panel=open]:grid-cols-[3.5rem_minmax(0,1fr)_auto]"
      >
        <div className="hidden min-h-0 flex-col bg-band md:flex">
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
          <div className="flex flex-wrap items-center gap-3 border-b border-rule px-4 py-2 md:hidden">
            {route.view !== "workstreams" && route.view !== "agents" ? (
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
                {narrowVirtual.map((column) => (
                  <option key={column.id} value={column.id}>
                    {column.title}
                  </option>
                ))}
                {COLUMNS.map((column) => (
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
              <code className="font-mono text-ink">{root}</code>, which does not
              exist yet. Agents create it when they first record work, or make a
              project folder there with a{" "}
              <code className="font-mono">tickets/</code> directory inside.
            </EmptyState>
          ) : route.view === "agents" ? (
            projects.status === "ready" ? (
              <AgentsView
                fetcher={fetcher}
                project={scopeProject}
                projects={summaries}
                revision={revision}
                onOpen={openTicket}
                onAnswer={editing.answer}
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
                shown={visible.length}
                total={allCards.length}
                density={route.view === "board" ? density : undefined}
                onDensity={
                  route.view === "board"
                    ? (value) =>
                        updatePreferences((current) => ({
                          ...current,
                          density: value,
                        }))
                    : undefined
                }
                virtualColumns={
                  route.view === "board" ? shownVirtual : undefined
                }
                onVirtualColumns={
                  route.view === "board"
                    ? (virtualColumns) =>
                        updatePreferences((current) => ({
                          ...current,
                          virtualColumns,
                        }))
                    : undefined
                }
                paint={route.view === "board" ? paint : undefined}
                onPaint={
                  route.view === "board"
                    ? (value) =>
                        updatePreferences((current) => ({
                          ...current,
                          colourBy: value,
                        }))
                    : undefined
                }
                onNewTicket={
                  summaries.length > 0 ? () => setNewTicket(true) : undefined
                }
              />
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
                    legend={
                      current
                        ? {
                            project: current.name,
                            workstreams: current.workstreams,
                          }
                        : undefined
                    }
                    projectNames={projectNames}
                    density={density}
                    paint={paint}
                    virtualColumns={shownVirtual}
                    selected={route.ticket}
                    doneTotal={board.data.doneTotal}
                    doneShown={board.data.doneShown}
                    doneAll={doneAll}
                    onDoneAll={setDoneAll}
                    onOpen={openTicket}
                    onMove={(card, to) => void editing.move(card, to)}
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
            workstreamsOf={(project) => workstreams.get(project) ?? []}
          />
        ) : null}
      </div>

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
      className="mx-4 mt-3 rounded-card border border-rule bg-well px-4 py-3 text-sm text-ink-muted"
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
      className="border-t border-band-track px-4 py-3 text-2xs text-on-band-muted max-[90rem]:group-data-[panel=open]/work:hidden"
    >
      <fieldset className="mb-2.5 flex items-center gap-2">
        <legend className="sr-only">Theme</legend>
        <span aria-hidden="true">Theme</span>
        <span className="flex rounded-control border border-band-track p-0.5">
          {themes.map((option) => (
            <label
              key={option.value}
              className={`cursor-pointer rounded-[3px] px-1.5 py-px transition-colors has-focus-visible:outline-2 has-focus-visible:outline-on-band ${
                theme === option.value
                  ? "bg-on-band text-band"
                  : "text-on-band-muted hover:text-on-band"
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
    <div aria-hidden="true" className="grid grid-cols-5 gap-4 p-4">
      {COLUMNS.map((column) => (
        <div key={column.id} className="flex flex-col gap-2">
          <div className="h-5 border-t-3 border-rule" />
          <div className="h-20 animate-pulse rounded-card bg-well" />
          <div className="h-16 animate-pulse rounded-card bg-well" />
        </div>
      ))}
    </div>
  );
}
