import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type {
  ProjectSummary,
  Workstream,
  WorkstreamTicket,
} from "../../api/board";
import { noRuns } from "../../api/runs";
import {
  WorkstreamLine,
  WorkstreamsSkeleton,
  WorkstreamsView,
} from "./WorkstreamsView";

// resource is what each project's useResource returns, so a test can show
// its loading, error and stale states without a server.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
  reload: vi.fn(),
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({ ...resource.state, reload: resource.reload }),
}));

// A finished workstream (every ticket Done, Rejected or Archived) shows as
// its route card head with a disclosure chevron and no stations until the
// user expands it; every other workstream shows in full (FH-68,
// ui-layout.md §4).
const ticket = (
  id: string,
  column: WorkstreamTicket["column"],
  extra: Partial<WorkstreamTicket> = {},
): WorkstreamTicket => ({
  id,
  title: `Ticket ${id}`,
  column,
  blocked: false,
  held: false,
  missing: false,
  dependsOn: [],
  outside: [],
  ...extra,
});

const workstream = (
  tickets: WorkstreamTicket[],
  status: Workstream["status"] = "completed",
): Workstream => ({
  slug: "foundation",
  title: "Foundation",
  status,
  suspended: false,
  declaredStatus: "",
  priority: "high",
  created: "2026-10-01",
  done: tickets.filter((t) => t.column === "done").length,
  total: tickets.length,
  next: "",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: [],
  needsRepair: [],
  warnings: [],
  tickets,
});

// chevron matches the chevronDown icon's svg, with its classes.
const chevron = /<svg[^>]*class="([^"]*)"[^>]*><path d="M6 9l6 6 6-6"/;

const render = (line: Workstream, expanded = false) =>
  renderToStaticMarkup(
    <WorkstreamLine
      headingLevel={2}
      project="flashheart"
      workstream={line}
      line={{ colour: 1, initials: "FO" }}
      expanded={expanded}
      onToggle={() => undefined}
      onOpen={() => undefined}
    />,
  );

const finished = workstream([
  ticket("FH-1", "done"),
  ticket("FH-2", "done", { rejected: true }),
  ticket("FH-3", "archived"),
]);

describe("WorkstreamLine", () => {
  it("collapses a finished workstream to its head", () => {
    const markup = render(finished);
    expect(markup).toMatch(/<button[^>]*aria-expanded="false"/);
    expect(markup).toContain("Foundation");
    expect(markup).toContain("Completed");
    expect(markup.replace(/<[^>]*>/g, "")).toContain("2 of 3 served");
    expect(markup).toMatch(chevron);
    expect(markup).not.toContain("data-station");
  });

  it("names the workstream in the disclosure button", () => {
    const markup = render(finished);
    const button = markup.slice(
      markup.indexOf("<button"),
      markup.indexOf("</button>"),
    );
    expect(button).toContain("Foundation");
  });

  it("shows the full card when the user expanded it", () => {
    const markup = render(finished, true);
    expect(markup).toMatch(/<button[^>]*aria-expanded="true"/);
    expect(markup).toContain('data-station="FH-1"');
  });

  it("stays expanded while a ticket waits in Ready to review", () => {
    const markup = render(
      workstream([ticket("FH-1", "done"), ticket("FH-2", "review")]),
    );
    expect(markup).toContain("Completed");
    expect(markup).not.toContain("aria-expanded");
    expect(markup).toContain('data-station="FH-2"');
  });

  it("always shows an active workstream in full", () => {
    const markup = render(
      workstream([ticket("FH-1", "done"), ticket("FH-2", "up-next")], "active"),
    );
    expect(markup).toContain("Running");
    expect(markup).not.toContain("aria-expanded");
    expect(markup).toContain('data-station="FH-2"');
  });

  it("marks a blocked workstream with the no-entry sign and the word", () => {
    const markup = render(
      workstream([ticket("FH-1", "up-next", { blocked: true })], "blocked"),
    );
    expect(markup).toMatch(
      /data-icon="blocked"(?:(?!<\/svg>)[\s\S])*<\/svg>Blocked</,
    );
  });

  it("opens the workstream's page from its title, or beside a finished line's head (FH-139)", () => {
    const active = render(workstream([ticket("FH-1", "up-next")], "active"));
    expect(active).toMatch(
      /<a href="#\/workstream\/flashheart\/foundation"[^>]*>Foundation<\/a>/,
    );
    const collapsed = render(finished);
    const button = collapsed.slice(
      collapsed.indexOf("<button"),
      collapsed.indexOf("</button>"),
    );
    expect(button).not.toContain("<a ");
    expect(collapsed).toMatch(
      /<a href="#\/workstream\/flashheart\/foundation" aria-label="Open Foundation"/,
    );
  });

  it("does not animate the chevron beyond the token-driven transition", () => {
    // Reduced motion sets the default transition duration to 0 ms
    // (styles/index.css); an explicit duration would escape it.
    const classes = render(finished).match(chevron)?.[1] ?? "";
    expect(classes).toContain("transition-transform");
    expect(classes).not.toMatch(/duration-/);
  });
});

const project: ProjectSummary = {
  name: "flashheart",
  displayName: "Flashheart",
  key: "FH",
  keyDerived: false,
  repos: [],
  counts: {
    backlog: 0,
    "up-next": 0,
    "in-progress": 0,
    review: 0,
    done: 0,
  },
  needsRepair: 0,
  blocked: 0,
  warnings: [],
  lastModified: "",
  workstreams: [
    {
      slug: "foundation",
      title: "Foundation",
      created: "2026-10-01",
      status: "active",
      done: 0,
      total: 1,
    },
  ],
  archived: 0,
  rejected: 0,
  runs: noRuns,
};

const renderView = () =>
  renderToStaticMarkup(
    <WorkstreamsView
      projects={[project]}
      fetcher={() => new Promise<Response>(() => {})}
      lines={new Map()}
      revision={0}
      expanded={{}}
      onExpanded={() => undefined}
      onOpen={() => undefined}
    />,
  );

describe("WorkstreamsView load states (ui-layout.md §8)", () => {
  afterEach(() => {
    resource.state = { status: "loading" };
  });

  it("loads in the shape of route cards", () => {
    expect(renderView()).toContain('data-skeleton="route-card"');
    expect(renderToStaticMarkup(<WorkstreamsSkeleton />)).toContain(
      'data-skeleton="workstreams"',
    );
  });

  it("says why a project's lines could not load, with Try again", () => {
    resource.state = { status: "error", error: "disk full" };
    const html = renderView();
    expect(html).toContain("Workstreams could not be loaded: disk full");
    expect(html).toContain("Try again");
  });

  it("keeps the last good lines with a note when a reload fails", () => {
    resource.state = {
      status: "ready",
      data: [workstream([ticket("FH-1", "up-next")], "active")],
      error: "disk full",
    };
    const html = renderView();
    expect(html).toContain("Showing the last good copy: disk full");
    expect(html).toContain('data-station="FH-1"');
  });
});
