import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { WorkstreamDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import type { Editing } from "../editing/useEditing";
import { WorkstreamPage } from "./WorkstreamPage";

// FH-174: the page's workstream gone from the server, as an agent's rename
// or a removal elsewhere leaves it. The page loads through the real
// useResource from a fake server whose workstreams the test changes.

const body = "\n# Board UI\n\n## Goal\n\nRender the board.\n";
const detail = (over: Partial<WorkstreamDetail> = {}): WorkstreamDetail => ({
  slug: "board-ui",
  title: "Board UI",
  status: "active",
  suspended: false,
  declaredStatus: "active",
  priority: "high",
  created: "2026-10-01",
  done: 0,
  total: 1,
  next: "AL-1",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: [],
  needsRepair: [],
  warnings: [],
  tickets: [
    {
      id: "AL-1",
      title: "Header",
      column: "in-progress",
      blocked: false,
      held: false,
      missing: false,
      dependsOn: [],
      outside: [],
    } as unknown as WorkstreamDetail["tickets"][number],
  ],
  body,
  hash: "base",
  raw: `---\nslug: board-ui\n---\n${body}`,
  ...over,
});

function fakeServer(initial: WorkstreamDetail[]) {
  let workstreams = initial;
  const fetcher = vi.fn(async (input: RequestInfo | URL) => {
    const path = String(input);
    if (path === "/api/projects/alpha/workstreams")
      return Response.json({ revision: 1, workstreams });
    const slug = decodeURIComponent(path.split("/").pop() ?? "");
    const found = workstreams.find((item) => item.slug === slug);
    if (path === "/api/projects/alpha/workstreams/boom")
      return Response.json(
        { error: { code: "internal", message: "disk on fire" } },
        { status: 500 },
      );
    return found
      ? Response.json({ revision: 1, workstream: found })
      : Response.json(
          { error: { code: "not_found", message: `No workstream ${slug}` } },
          { status: 404 },
        );
  }) as unknown as AuthenticatedFetch;
  return {
    fetcher,
    set(next: WorkstreamDetail[]) {
      workstreams = next;
    },
  };
}

function page(
  server: ReturnType<typeof fakeServer>,
  slug = "board-ui",
  extra: { onRenamed?(slug: string): void; editing?: Editing } = {},
) {
  const props = (revision: number) => (
    <WorkstreamPage
      workstream={{ project: "alpha", slug }}
      fetcher={server.fetcher}
      lines={new Map()}
      keys={new Set(["AL"])}
      revision={revision}
      onOpen={() => undefined}
      backLink={{ href: "#/p/alpha/workstreams", label: "Alpha" }}
      {...extra}
    />
  );
  const view = render(props(1));
  return { ...view, moveRevision: () => view.rerender(props(2)) };
}

describe("WorkstreamPage when its workstream is gone", () => {
  it("follows a rename made elsewhere to the new slug, and says so", async () => {
    const server = fakeServer([detail()]);
    const onRenamed = vi.fn();
    const notify = vi.fn();
    const editing = { notify } as unknown as Editing;
    const { moveRevision } = page(server, "board-ui", { onRenamed, editing });
    await screen.findByRole("heading", { level: 1, name: "Board UI" });

    // An agent renamed board-ui to board: same name, same tickets.
    server.set([detail({ slug: "board" })]);
    moveRevision();
    await vi.waitFor(() => expect(onRenamed).toHaveBeenCalledWith("board"));
    expect(notify).toHaveBeenCalledWith({
      text: "Board UI was renamed elsewhere: its short name is now board.",
    });
    expect(screen.queryByText(/could not be loaded/)).toBeNull();
  });

  it("says in words that a removed workstream is gone, with the way back", async () => {
    const server = fakeServer([detail()]);
    const onRenamed = vi.fn();
    const { moveRevision } = page(server, "board-ui", { onRenamed });
    await screen.findByRole("heading", { level: 1, name: "Board UI" });

    server.set([]);
    moveRevision();
    const heading = await screen.findByRole("heading", {
      level: 1,
      name: "Board UI is no longer here",
    });
    expect(heading).toBeTruthy();
    expect(
      screen.getByText(
        "Alpha has no workstream board-ui now: it was renamed or removed.",
      ),
    ).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: "Show the Alpha workstreams" })
        .getAttribute("href"),
    ).toBe("#/p/alpha/workstreams");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(onRenamed).not.toHaveBeenCalled();
  });

  it("says so for an address that was gone before the page opened", async () => {
    const server = fakeServer([detail({ slug: "board" })]);
    page(server, "board-ui");
    await screen.findByRole("heading", {
      level: 1,
      name: "board-ui is no longer here",
    });
    expect(
      screen.getByRole("link", { name: "Show the Alpha workstreams" }),
    ).toBeTruthy();
  });

  it("still shows any other failure as one, with Try again", async () => {
    const server = fakeServer([]);
    page(server, "boom");
    expect(
      await screen.findByText(
        "This workstream could not be loaded: disk on fire",
      ),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
  });
});
