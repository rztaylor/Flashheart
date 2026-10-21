import { useEffect, useMemo, useState } from "react";
import { restoreProject } from "../api/archive";
import type { Card, Column } from "../api/board";
import type { Created } from "../api/edit";
import { clearProjectImage, setProjectImage } from "../api/files";
import type { EditorMode, Preferences } from "../api/preferences";
import { AsideProvider } from "../components/Aside";
import { Toast } from "../components/Toast";
import { CardPanel } from "../features/card/CardPanel";
import { TicketPage } from "../features/card/TicketPage";
import { BlockedMoveDialog } from "../features/editing/BlockedMoveDialog";
import {
  type EditPlace,
  LateSavesProvider,
} from "../features/editing/lateSaves";
import { NewTicketDialog } from "../features/editing/NewTicketDialog";
import { EditorModeProvider } from "../features/editor/mode";
import { ProjectImageMenu } from "../features/projects/ProjectImageMenu";
import { ProjectRail } from "../features/projects/ProjectRail";
import { NewWorkstreamDialog } from "../features/workstreams/NewWorkstreamDialog";
import { WorkstreamPage } from "../features/workstreams/WorkstreamPage";
import type { SingleserveLifecycle } from "../lifecycle/useSingleserve";
import { shownColumns } from "../model/columns";
import { messageOf } from "../model/errors";
import { applyFilters, isFiltered } from "../model/filters";
import { afterForStep, predecessor, type Step } from "../model/order";
import type { Scope } from "../model/scopes";
import { viewSummary } from "../model/summary";
import { leave } from "../state/leaveGuard";
import { Band, BandNotices, type BandView } from "./Band";
import { NarrowControls } from "./NarrowControls";
import { pressNeedsYou } from "./needsYou";
import { PageActions, PageHeader } from "./PageHeader";
import { formatRoute, type Route, useRoute } from "./route";
import { SettingsMenu } from "./SettingsMenu";
import { renameWorkstreamIn } from "./scopeMemory";
import { useBoardData } from "./useBoardData";
import { useScopeMemory } from "./useScopeMemory";
import type { ServerInfoState } from "./useServerInfo";
import { useTicketPanel } from "./useTicketPanel";
import { ViewOutlet } from "./ViewOutlet";

interface ShellProps {
  lifecycle: SingleserveLifecycle;
  info: ServerInfoState;
  preferences: Preferences;
  preferencesLoaded: boolean;
  updatePreferences(change: (current: Preferences) => Preferences): void;
  // refreshPreferences reads the saved preferences again (FH-173);
  // preferencesRefreshed counts the reads that changed them.
  refreshPreferences(): void;
  preferencesRefreshed: number;
}

// Shell composes the signage band, the project rail, the page header, the
// routed view and the card panel, or under the band a ticket's full page.
// Loading lives in useBoardData, the remembered filters in useScopeMemory
// and the views' bodies in ViewOutlet.
export function Shell({
  lifecycle,
  info,
  preferences,
  preferencesLoaded,
  updatePreferences,
  refreshPreferences,
  preferencesRefreshed,
}: ShellProps) {
  const { state, fetch: fetcher, ready } = lifecycle;
  const [route, navigate] = useRoute();
  const data = useBoardData({
    fetcher,
    ready,
    scope: route.scope,
    view: route.view,
  });
  const { projects, board, editing, cards, summaries, current } = data;
  // The saved preferences are read again as the board changes, so an
  // agent's workstream rename that rewrote them reaches this tab (FH-173).
  useEffect(() => {
    if (data.revision > 0) refreshPreferences();
  }, [data.revision, refreshPreferences]);
  const memory = useScopeMemory({
    route,
    preferences,
    preferencesLoaded,
    refreshed: preferencesRefreshed,
    updatePreferences,
  });
  const { filters, setFilters } = memory;
  const panel = useTicketPanel(route, navigate);
  const { openTicket } = panel;
  // needsYouFrom is the scope the Needs you filter was turned on from, to
  // return to when it is turned off (FH-44).
  const [needsYouFrom, setNeedsYouFrom] = useState<Scope>();
  // newTicket is the project New ticket opens in, while its dialog is open.
  const [newTicket, setNewTicket] = useState<{ project: string }>();
  // newWorkstream is the project New workstream opens in (FH-17).
  const [newWorkstream, setNewWorkstream] = useState<{ project: string }>();
  const [narrowColumn, setNarrowColumn] = useState<string>("in-progress");

  const visible = useMemo(() => applyFilters(cards, filters), [cards, filters]);
  // place moves a ticket (to a place in a column when after is given;
  // EDIT-9), remembering where it was for Undo.
  const place = (card: Card, to: Column, after?: string) =>
    void editing.move(
      card,
      to,
      "",
      after === undefined ? {} : { after, undoAfter: predecessor(cards, card) },
    );
  // stepTicket is the panel's Position buttons: a step among the visible cards
  // of the ticket's column, offered only while the board shows the ticket.
  const shownTicket = cards.find((card) => card.id === route.ticket?.id);
  const stepTicket =
    shownTicket &&
    shownTicket.column !== "done" &&
    visible.some((card) => card.id === shownTicket.id)
      ? (step: Step) => {
          const after = afterForStep(visible, shownTicket, step);
          if (after !== null) place(shownTicket, shownTicket.column, after);
        }
      : undefined;
  // The section editor's view, formatted or markdown, is remembered with
  // the other preferences (FH-16).
  const editorMode = {
    mode: preferences.editor,
    setMode: (editor: EditorMode) =>
      updatePreferences((current) => ({ ...current, editor })),
  };
  // The phone column picker falls back to In progress when the column it
  // showed was hidden.
  const pickerColumns = shownColumns(preferences.hiddenColumns);
  const narrowShown = pickerColumns.some((column) => column.id === narrowColumn)
    ? narrowColumn
    : "in-progress";

  const go = (next: Partial<Route>) => navigate({ ...route, ...next });
  // From a ticket's full page or a workstream's page the band leaves it.
  const chooseView = (view: BandView) => {
    go(
      route.view === "ticket" || route.view === "workstream"
        ? { view, ticket: undefined, workstream: undefined }
        : { view },
    );
    memory.rememberView(view);
  };
  // The band's Needs you pill filters All projects' Board or Table, and
  // turned off returns to where it was turned on (FH-44).
  const toggleNeedsYou = () => {
    const press = pressNeedsYou(route, filters.needsYou, needsYouFrom);
    setNeedsYouFrom(press.cameFrom);
    if (!press.go) {
      setFilters({ ...filters, needsYou: press.on });
      return;
    }
    // A new scope restores its own filters, carrying this one over.
    memory.carryFilters({ ...filters, needsYou: press.on });
    go(press.go);
  };
  const selectScope = (scope: Scope) => {
    data.setDoneAll(false);
    go({ scope, view: memory.viewFor(scope), ticket: undefined });
  };
  // projectArchived leaves an archived project's scope for the projects
  // archive, with Undo (PRJ-5).
  const projectArchived = (name: string, displayName: string) => {
    navigate({ scope: { kind: "all" }, view: "archive" });
    data.reloadAll();
    editing.notify({
      text: `Archived project ${displayName}.`,
      action: {
        label: "Undo",
        run: () =>
          void restoreProject(fetcher, name)
            .then(data.reloadAll)
            .catch((error: unknown) =>
              editing.notify({
                text: `${displayName} was not restored: ${messageOf(
                  error,
                  "the request failed",
                )}`,
              }),
            ),
      },
    });
  };

  // backToEdit is a late save's Back to the edit (FH-156): it goes back to
  // where the edit was made, asking first while this page has unsaved
  // work, and has the edit reopened there (ready).
  const backToEdit = (place: EditPlace, ready: () => void) =>
    leave(() => {
      ready();
      if (place.kind === "panel") {
        const onPage = route.view === "ticket" || route.view === "workstream";
        openTicket(
          { id: place.id },
          place.tab,
          onPage ? { scope: { kind: "all" }, view: "board" } : route,
        );
      } else
        navigate(
          place.kind === "ticket"
            ? {
                scope: { kind: "all" },
                view: "ticket",
                ticket: { id: place.id },
              }
            : {
                scope: { kind: "all" },
                view: "workstream",
                workstream: { project: place.project, slug: place.slug },
              },
        );
    });

  const scopeName =
    route.scope.kind === "all"
      ? "All projects"
      : (current?.displayName ?? data.scopeProject);
  // A read-only server shows agents' questions without an answer form.
  const answers = info.status === "ready" && info.info.answers;
  const root =
    info.status === "ready"
      ? info.info.root
      : projects.status === "ready"
        ? projects.data.root
        : "";
  const boardOrTable = route.view === "board" || route.view === "table";
  const search = {
    query: filters.query,
    onChange: (query: string) => setFilters({ ...filters, query }),
  };

  const summary = viewSummary({
    view: route.view,
    board:
      board.status === "ready"
        ? { shown: visible.length, total: cards.length }
        : undefined,
    filtered: isFiltered(filters),
    counts:
      projects.status === "ready"
        ? (current ? [current] : summaries).map((project) => project.counts)
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
      <LateSavesProvider notify={editing.notify} back={backToEdit}>
        <div className="grid h-full grid-cols-[minmax(0,1fr)] grid-rows-[3.5rem_auto_1fr]">
          <Band
            lifecycle={lifecycle}
            view={route.view}
            onView={chooseView}
            // Needs you is visible from every view and project (SPEC §7).
            needsYou={
              projects.status === "ready" ? projects.data.runs.needsYou : 0
            }
            needsYouOn={filters.needsYou}
            onNeedsYou={toggleNeedsYou}
            search={boardOrTable ? search : undefined}
            settings={
              <SettingsMenu
                root={root}
                info={info}
                theme={preferences.theme}
                onTheme={(theme) =>
                  updatePreferences((current) => ({ ...current, theme }))
                }
              />
            }
          />
          <BandNotices
            shutdownDenied={state.shutdownDenied}
            onDismiss={lifecycle.dismissDenial}
            stale={data.stale}
          />

          {route.view === "ticket" && route.ticket ? (
            <main
              className="relative min-h-0 overflow-y-auto"
              aria-label={`Ticket ${route.ticket.id}`}
            >
              <EditorModeProvider value={editorMode}>
                <TicketPage
                  key={route.ticket.id}
                  ticket={route.ticket}
                  fetcher={fetcher}
                  lines={data.lines}
                  workstreams={data.workstreams}
                  keys={data.keys}
                  onOpen={(ticket) =>
                    navigate({ scope: { kind: "all" }, view: "ticket", ticket })
                  }
                  onClose={() =>
                    navigate({ scope: { kind: "all" }, view: "board" })
                  }
                  revision={data.revision}
                  editing={editing}
                  answers={answers}
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
              </EditorModeProvider>
            </main>
          ) : route.view === "workstream" && route.workstream ? (
            <main
              className="relative min-h-0 overflow-y-auto"
              aria-label={`Workstream ${route.workstream.slug}`}
            >
              <EditorModeProvider value={editorMode}>
                <WorkstreamPage
                  key={`${route.workstream.project}/${route.workstream.slug}`}
                  workstream={route.workstream}
                  fetcher={fetcher}
                  lines={data.lines}
                  keys={data.keys}
                  revision={data.revision}
                  editing={editing}
                  onOpen={(ticket) =>
                    navigate({ scope: { kind: "all" }, view: "ticket", ticket })
                  }
                  onRenamed={(slug) => {
                    // The page moves to the new slug in place of the old
                    // address (FH-167), and the saved filters and expanded
                    // choices follow it as the server's did (FH-170).
                    const { project, slug: from } = route.workstream ?? {
                      project: "",
                      slug: "",
                    };
                    updatePreferences((current) =>
                      renameWorkstreamIn(current, project, from, slug),
                    );
                    data.reloadAll();
                    navigate(
                      { ...route, workstream: { project, slug } },
                      { replace: true },
                    );
                  }}
                  backLink={{
                    href: formatRoute({
                      scope: {
                        kind: "project",
                        project: route.workstream.project,
                      },
                      view: "workstreams",
                    }),
                    label:
                      summaries.find(
                        (item) => item.name === route.workstream?.project,
                      )?.displayName ?? route.workstream.project,
                  }}
                  siblings={summaries.find(
                    (item) => item.name === route.workstream?.project,
                  )}
                />
              </EditorModeProvider>
            </main>
          ) : (
            // The rail is a slim column of key badges at every width from 48rem
            // (FH-150), so the board keeps its width with the panel open too.
            <div className="grid min-h-0 grid-cols-1 md:grid-cols-[4rem_minmax(0,1fr)_auto]">
              <div className="hidden min-h-0 border-r border-rail-rule bg-rail md:block">
                <ProjectRail
                  projects={summaries}
                  runs={
                    projects.status === "ready" ? projects.data.runs : undefined
                  }
                  scope={route.scope}
                  onSelect={selectScope}
                />
              </div>

              <main
                className="flex min-h-0 min-w-0 flex-col"
                aria-label={`${scopeName} ${route.view}`}
              >
                <PageHeader
                  projectKey={current?.key}
                  projectImage={current?.image}
                  badge={
                    // The project's badge opens its image menu (FH-151);
                    // a change shows as soon as the projects list is read
                    // again.
                    current ? (
                      <ProjectImageMenu
                        project={current}
                        onSet={async (file) => {
                          await setProjectImage(fetcher, current.name, file);
                          projects.reload();
                        }}
                        onClear={async () => {
                          await clearProjectImage(fetcher, current.name);
                          projects.reload();
                        }}
                        onDone={(text) => editing.notify({ text })}
                      />
                    ) : undefined
                  }
                  title={scopeName}
                  summary={summary}
                  live={boardOrTable}
                  aside={
                    <PageActions
                      archive={
                        route.view !== "archive" &&
                        projects.status === "ready" &&
                        (current || route.scope.kind === "all")
                          ? {
                              href: formatRoute({
                                ...route,
                                view: "archive",
                                ticket: undefined,
                              }),
                              count: current
                                ? current.archived
                                : projects.data.archivedProjects,
                              onOpen: () =>
                                go({ view: "archive", ticket: undefined }),
                            }
                          : undefined
                      }
                      onNewTicket={
                        boardOrTable && summaries.length > 0
                          ? () => setNewTicket({ project: data.scopeProject })
                          : undefined
                      }
                      onNewWorkstream={
                        route.view === "workstreams" && summaries.length > 0
                          ? () =>
                              setNewWorkstream({ project: data.scopeProject })
                          : undefined
                      }
                    />
                  }
                />
                <NarrowControls
                  search={boardOrTable ? search : undefined}
                  project={data.scopeProject}
                  projects={summaries}
                  onScope={selectScope}
                  column={
                    route.view === "board"
                      ? {
                          value: narrowShown,
                          columns: pickerColumns,
                          onChange: setNarrowColumn,
                        }
                      : undefined
                  }
                />

                <ViewOutlet
                  route={route}
                  data={data}
                  fetcher={fetcher}
                  answers={answers}
                  root={root}
                  preferences={preferences}
                  updatePreferences={updatePreferences}
                  filters={filters}
                  onFilters={setFilters}
                  visible={visible}
                  narrowColumn={narrowShown}
                  go={go}
                  navigate={navigate}
                  onOpen={openTicket}
                  onMove={place}
                  onNewTicket={(project) => setNewTicket({ project })}
                  onProjectArchived={projectArchived}
                />
              </main>

              {route.ticket ? (
                <CardPanel
                  // A Review results press reopens the panel on Review.
                  key={`${route.ticket.id}:${panel.initialTab ?? ""}`}
                  initialTab={panel.initialTab}
                  ticket={route.ticket}
                  fetcher={fetcher}
                  lines={data.lines}
                  workstreams={data.workstreams}
                  keys={data.keys}
                  onOpen={openTicket}
                  onClose={panel.closeTicket}
                  revision={data.revision}
                  editing={editing}
                  onStep={stepTicket}
                  answers={answers}
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
              project={newTicket.project}
              onClose={() => setNewTicket(undefined)}
              onCreated={(created: Created) => {
                setNewTicket(undefined);
                data.reloadAll();
                editing.notify({ text: `Created ${created.id}.` });
                // The new ticket opens ready to edit: on its Edit tab, focus
                // in its first field, for what the dialog does not ask
                // (FH-17).
                openTicket({ id: created.id }, "edit");
              }}
            />
          ) : null}
          {newWorkstream ? (
            <NewWorkstreamDialog
              fetcher={fetcher}
              projects={summaries}
              project={newWorkstream.project}
              onClose={() => setNewWorkstream(undefined)}
              onCreated={(created) => {
                setNewWorkstream(undefined);
                data.reloadAll();
                editing.notify({
                  text: `Created workstream ${created.title}.`,
                });
                // Its page is where its scope and success criteria are
                // written; tickets join it from their Edit tab (FH-17).
                navigate({
                  scope: { kind: "all" },
                  view: "workstream",
                  workstream: { project: created.project, slug: created.slug },
                });
              }}
            />
          ) : null}
          <Toast message={editing.toast} onDismiss={editing.dismiss} />
        </div>
      </LateSavesProvider>
    </AsideProvider>
  );
}
