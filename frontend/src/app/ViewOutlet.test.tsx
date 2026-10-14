import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { ProjectsResponse } from "../api/board";
import { defaultPreferences } from "../api/preferences";
import { noRuns } from "../api/runs";
import { emptyFilters, type Filters } from "../model/filters";
import type { Route, View } from "./route";
import { type BoardDataState, staleError } from "./useBoardData";
import { ViewOutlet } from "./ViewOutlet";

const loading = { status: "loading" as const, reload: vi.fn() };

const projectsResponse: ProjectsResponse = {
  revision: 1,
  root: "/board",
  rootMissing: false,
  projects: [],
  archivedProjects: 0,
  runs: noRuns,
};

// data is the board data a test's view reads: everything still loading
// unless the test says otherwise.
function data(patch: Partial<BoardDataState> = {}): BoardDataState {
  return {
    revision: 0,
    projects: loading,
    board: loading,
    reloadAll: vi.fn(),
    editing: { notify: vi.fn() },
    doneAll: false,
    setDoneAll: vi.fn(),
    cards: [],
    summaries: [],
    current: undefined,
    scopeProject: "",
    lines: new Map(),
    workstreams: new Map(),
    keys: new Set(),
    ...patch,
  } as unknown as BoardDataState;
}

function render(
  view: View,
  boardData = data(),
  preferences = defaultPreferences,
  scope: Route["scope"] = { kind: "all" },
  filters: Filters = emptyFilters,
) {
  return renderToStaticMarkup(
    <ViewOutlet
      route={{ scope, view }}
      data={boardData}
      fetcher={() => new Promise<Response>(() => {})}
      answers={false}
      root="/board"
      preferences={preferences}
      updatePreferences={vi.fn()}
      filters={filters}
      onFilters={vi.fn()}
      visible={[]}
      narrowColumn="in-progress"
      go={vi.fn()}
      navigate={vi.fn()}
      onOpen={vi.fn()}
      onMove={vi.fn()}
      onNewTicket={vi.fn()}
      onProjectArchived={vi.fn()}
    />,
  );
}

// The load states of ui-layout.md §8 for what the shell loads itself.
describe("ViewOutlet", () => {
  it("offers Try again when the projects could not be read", () => {
    const html = render(
      "overview",
      data({
        projects: { status: "error", error: "disk full", reload: vi.fn() },
      }),
    );
    expect(html).toContain("The board could not be read: disk full");
    expect(html).toContain("Try again");
    expect(html).not.toContain("data-skeleton");
  });

  it.each([
    ["overview", "overview"],
    ["workstreams", "workstreams"],
    ["archive", "archive"],
    ["board", "board"],
    ["table", "table"],
  ] as const)("loads the %s in its own shape", (view, skeleton) => {
    const html = render(view);
    const views = ["overview", "workstreams", "archive", "board", "table"];
    for (const other of views)
      expect(html.includes(`data-skeleton="${other}"`), other).toBe(
        other === skeleton,
      );
  });

  it("leaves a hidden Backlog out of the board's skeleton", () => {
    const wells = (html: string) =>
      html.match(/data-skeleton-column/g)?.length ?? 0;
    expect(wells(render("board"))).toBe(5);
    expect(
      wells(
        render("board", data(), {
          ...defaultPreferences,
          hiddenColumns: ["backlog"],
        }),
      ),
    ).toBe(4);
  });

  it("offers Try again when the board could not be loaded", () => {
    const html = render(
      "board",
      data({
        projects: {
          status: "ready",
          data: projectsResponse,
          reload: vi.fn(),
        },
        board: { status: "error", error: "disk full", reload: vi.fn() },
      }),
    );
    expect(html).toContain("This board could not be loaded: disk full");
    expect(html).toContain("Try again");
  });
});

// emptyBoard is a ready board whose filters keep no card, with agents
// needing the user that no card shows: needsYouUnticketed of them across
// every project, inAlpha of them in the alpha project.
function emptyBoard(needsYouUnticketed: number, inAlpha = needsYouUnticketed) {
  const runs = { ...noRuns, needsYou: needsYouUnticketed, needsYouUnticketed };
  return data({
    projects: {
      status: "ready",
      data: { ...projectsResponse, runs },
      reload: vi.fn(),
    },
    board: {
      status: "ready",
      data: { cards: [], doneTotal: 0, doneShown: 0 },
      reload: vi.fn(),
    },
    current: {
      name: "alpha",
      runs: { ...noRuns, needsYouUnticketed: inAlpha },
    },
  } as unknown as Partial<BoardDataState>);
}

const needsYou: Filters = { ...emptyFilters, needsYou: true };

// The Needs you filter with nothing a card can show (FH-157): the board
// and table say why and where to answer, never to clear a filter.
describe("ViewOutlet's Needs you filter with no card to show", () => {
  it.each(["board", "table"] as const)(
    "explains agents with no ticket on the %s",
    (view) => {
      const html = render(
        view,
        emptyBoard(1),
        defaultPreferences,
        undefined,
        needsYou,
      );
      expect(html).toContain("1 agent needs you, not about a ticket");
      expect(html).toContain("Answer it in the Overview");
      expect(html).not.toContain("No tickets match these filters");
      expect(html).not.toContain("Clear a filter");
      // The strip above cards is for when some cards show.
      expect(html.match(/Agents without a ticket/g)).toHaveLength(1);
    },
  );

  it("counts a project's own agents in its scope", () => {
    const html = render(
      "board",
      emptyBoard(2, 1),
      defaultPreferences,
      { kind: "project", project: "alpha" },
      needsYou,
    );
    expect(html).toContain("1 agent needs you, not about a ticket");
  });

  it("says nothing needs you when the filter alone empties the board", () => {
    const html = render(
      "board",
      emptyBoard(0),
      defaultPreferences,
      undefined,
      needsYou,
    );
    expect(html).toContain("Nothing needs you right now");
    expect(html).not.toContain("Clear a filter");
  });

  it("keeps the filter advice when other filters may be the reason", () => {
    const html = render("board", emptyBoard(0), defaultPreferences, undefined, {
      ...needsYou,
      query: "zzz",
    });
    expect(html).toContain("No tickets match these filters");
  });

  it("names other filters beside agents with no ticket", () => {
    const html = render("board", emptyBoard(1), defaultPreferences, undefined, {
      ...needsYou,
      query: "zzz",
    });
    expect(html).toContain("1 agent needs you, not about a ticket");
    expect(html).toContain("Other filters are on too");
    expect(html).not.toContain("No tickets match these filters");
  });
});

describe("staleError", () => {
  const stale = (error: string) =>
    ({ status: "ready", data: projectsResponse, error }) as const;

  it("is the board's failed reload on the Board and Table", () => {
    expect(staleError("board", stale("projects"), stale("board"))).toBe(
      "board",
    );
    expect(staleError("table", stale("projects"), stale("board"))).toBe(
      "board",
    );
  });

  it("is the projects' failed reload in every view", () => {
    for (const view of [
      "board",
      "overview",
      "workstreams",
      "archive",
      "ticket",
    ] as const)
      expect(staleError(view, stale("projects"), { status: "loading" })).toBe(
        "projects",
      );
  });

  it("ignores a board left behind by another view", () => {
    expect(
      staleError("overview", { status: "loading" }, stale("board")),
    ).toBeUndefined();
  });

  it("is nothing while every copy is fresh or never loaded", () => {
    expect(
      staleError(
        "board",
        { status: "error", error: "x" },
        { status: "loading" },
      ),
    ).toBeUndefined();
  });
});
