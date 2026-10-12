import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Editing } from "../editing/useEditing";
import { ArchiveSkeleton, ArchiveView } from "./ArchiveView";
import { ProjectsArchiveView } from "./ProjectsArchiveView";

// resource is what the view's useResource returns, so a test can show its
// loading, error and stale states without a server.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
  reload: vi.fn(),
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({ ...resource.state, reload: resource.reload }),
}));

const fetcher = () => new Promise<Response>(() => {});
const editing = { notify: vi.fn() } as unknown as Editing;
const noop = () => undefined;

const ticketArchive = () =>
  renderToStaticMarkup(
    <ArchiveView
      fetcher={fetcher}
      project="flashheart"
      revision={0}
      editing={editing}
      onOpen={noop}
      onBack={noop}
      onProjectArchived={noop}
    />,
  );

const projectsArchive = () =>
  renderToStaticMarkup(
    <ProjectsArchiveView
      fetcher={fetcher}
      revision={0}
      projects={[]}
      editing={editing}
      onArchived={noop}
      onOpenProject={noop}
      onBack={noop}
    />,
  );

afterEach(() => {
  resource.state = { status: "loading" };
});

// The load states of ui-layout.md §8 in both archives.
describe.each([
  ["a project's archive", ticketArchive],
  ["the projects archive", projectsArchive],
])("%s", (_, render) => {
  it("loads in the shape of its table, not as words", () => {
    const html = render();
    expect(html).toContain('data-skeleton="archive"');
    expect(html).not.toContain("Loading the archive");
  });

  it("says why it could not load, with Try again", () => {
    resource.state = { status: "error", error: "disk full" };
    const html = render();
    expect(html).toContain("The archive could not be loaded: disk full");
    expect(html).toContain("Try again");
  });
});

describe("stale archives", () => {
  it("keep a project's archived tickets with a note when a reload fails", () => {
    resource.state = {
      status: "ready",
      data: {
        tickets: [
          {
            id: "FH-9",
            title: "Old idea",
            type: "feature",
            priority: "low",
            workstream: "",
            column: "backlog",
            archived: "2026-10-01T10:00:00Z",
            attachments: 0,
            hasReview: false,
          },
        ],
      },
      error: "disk full",
    };
    const html = ticketArchive();
    expect(html).toContain("Showing the last good copy: disk full");
    expect(html).toContain("Old idea");
  });

  it("keep the archived projects with a note when a reload fails", () => {
    resource.state = {
      status: "ready",
      data: {
        projects: [
          {
            name: "beta",
            displayName: "Beta",
            key: "BE",
            repos: [],
            tickets: 3,
            archived: "2026-10-01T10:00:00Z",
          },
        ],
      },
      error: "disk full",
    };
    const html = projectsArchive();
    expect(html).toContain("Showing the last good copy: disk full");
    expect(html).toContain("Beta");
  });

  it("draws the skeleton as a table surface with rows", () => {
    const html = renderToStaticMarkup(<ArchiveSkeleton />);
    expect(html).toContain('aria-hidden="true"');
    expect(html).toContain("rounded-panel");
  });
});
