import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { WorkstreamDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import type { Editing } from "../editing/useEditing";
import { WorkstreamPage } from "./WorkstreamPage";

// The workstream's useResource is mocked, so the page renders a loaded
// workstream without a server; saves go to a fake server that records
// them.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
  reloads: 0,
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({
    ...resource.state,
    reload: () => {
      resource.reloads += 1;
    },
  }),
}));

// The formatted editor (FH-16) is stood in for by a text area, as on the
// ticket page's tests; the editor runs in the browser suite.
vi.mock("../editor/MarkdownField", () => ({
  MarkdownField: ({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: string;
    onChange(value: string): void;
  }) => (
    <textarea
      aria-label={label}
      data-autofocus
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}));

interface Request {
  method: string;
  path: string;
  body: Record<string, unknown>;
}

function fakeServer() {
  const requests: Request[] = [];
  const replies: (() => Response)[] = [];
  const fetcher = vi.fn(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const method = init?.method ?? "GET";
      if (method === "GET") return new Promise<Response>(() => {});
      requests.push({
        method,
        path: String(input),
        body: JSON.parse(String(init?.body ?? "{}")),
      });
      const reply = replies.shift();
      return reply
        ? reply()
        : Response.json({ hash: "saved", revision: 2, warnings: [] });
    },
  ) as unknown as AuthenticatedFetch;
  return { fetcher, requests, replies };
}

const body =
  "\n# Board UI\n\n## Goal\n\nRender **and** edit the board.\n\n## Scope\n\n## Notes\n\nFixture workstream.\n";

const workstream = (
  over: Partial<WorkstreamDetail> = {},
): WorkstreamDetail => ({
  slug: "board-ui",
  title: "Board UI",
  status: "active",
  suspended: false,
  declaredStatus: "active",
  priority: "high",
  created: "2026-10-01",
  done: 1,
  total: 3,
  next: "AL-3",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: ["ui"],
  needsRepair: [],
  warnings: [],
  tickets: [],
  body,
  hash: "base",
  raw: `---\nslug: board-ui\n---\n${body}`,
  ...over,
});

const conflict = (content: string) => () =>
  Response.json(
    {
      error: { code: "conflict", message: "This workstream changed." },
      current: { hash: "newer", content },
    },
    { status: 409 },
  );

let server: ReturnType<typeof fakeServer>;
let editing: Editing;

function renderPage(
  data = workstream(),
  withEditing = true,
  onRenamed?: (slug: string) => void,
) {
  resource.state = { status: "ready", data };
  return render(
    <WorkstreamPage
      workstream={{ project: "alpha", slug: "board-ui" }}
      fetcher={server.fetcher}
      lines={new Map()}
      keys={new Set(["AL"])}
      revision={1}
      editing={withEditing ? editing : undefined}
      onOpen={() => undefined}
      onRenamed={onRenamed}
      backLink={{ href: "#/p/alpha/workstreams", label: "Alpha" }}
    />,
  );
}

beforeEach(() => {
  server = fakeServer();
  resource.reloads = 0;
  editing = {
    move: vi.fn(),
    archive: vi.fn(),
    notify: vi.fn(),
    answer: vi.fn(),
  } as unknown as Editing;
});

describe("WorkstreamPage", () => {
  it("shows the workstream's title and sections, an empty one as Add", () => {
    renderPage();
    expect(
      screen.getByRole("heading", { level: 1, name: "Board UI" }),
    ).toBeTruthy();
    const goal = screen.getByRole("region", { name: "Goal" });
    expect(goal.textContent).toContain("Render and edit the board.");
    expect(
      within(goal).getByRole("button", { name: "Edit goal" }),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "Add scope" })).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "Add success criteria" }),
    ).toBeTruthy();
    expect(screen.getByRole("region", { name: "Notes" }).textContent).toContain(
      "Fixture workstream.",
    );
    expect(
      screen
        .getByRole("link", { name: /Alpha workstreams/ })
        .getAttribute("href"),
    ).toBe("#/p/alpha/workstreams");
  });

  it("saves only the edited section on the version it opened on", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(screen.getByRole("button", { name: "Edit goal" }));
    const editor = screen.getByRole("textbox", { name: "Goal" });
    expect((editor as HTMLTextAreaElement).value).toBe(
      "Render **and** edit the board.",
    );
    expect(document.activeElement).toBe(editor);
    await user.type(editor, " Fast.");
    await user.keyboard("{Control>}{Enter}{/Control}");
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Goal" })).toBeNull(),
    );
    expect(server.requests).toEqual([
      {
        method: "PUT",
        path: "/api/projects/alpha/workstreams/board-ui/section",
        body: {
          base: "base",
          section: "Goal",
          content: "Render **and** edit the board. Fast.",
        },
      },
    ]);
    expect(resource.reloads).toBeGreaterThan(0);
    expect(editing.notify).toHaveBeenCalledWith(
      expect.objectContaining({ text: "Saved Board UI." }),
    );
    // Focus returns to the section's pencil.
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Edit goal" }),
    );
  });

  it("writes nothing for an untouched save, and Escape cancels", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(screen.getByRole("button", { name: "Edit goal" }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.queryByRole("textbox", { name: "Goal" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Add scope" }));
    await user.type(screen.getByRole("textbox", { name: "Scope" }), "Lost");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Scope" })).toBeNull();
    expect(server.requests).toEqual([]);
  });

  it("saves over a change elsewhere in the file, and asks over the same section", async () => {
    const user = userEvent.setup();
    renderPage();
    // Someone changed Notes: the Goal saves again on that version.
    server.replies.push(
      conflict(
        "# Board UI\n\n## Goal\n\nRender **and** edit the board.\n\n## Notes\n\nTheirs.\n",
      ),
    );
    await user.click(screen.getByRole("button", { name: "Edit goal" }));
    await user.type(screen.getByRole("textbox", { name: "Goal" }), " Mine.");
    await user.keyboard("{Control>}{Enter}{/Control}");
    await waitFor(() => expect(server.requests).toHaveLength(2));
    expect(server.requests.map((request) => request.body.base)).toEqual([
      "base",
      "newer",
    ]);
    expect(screen.queryByRole("dialog")).toBeNull();

    // Someone changed the Goal itself: the conflict dialog.
    await user.click(screen.getByRole("button", { name: "Edit goal" }));
    server.replies.push(
      conflict("# Board UI\n\n## Goal\n\nTheirs.\n\n## Notes\n\nNone.\n"),
    );
    await user.type(screen.getByRole("textbox", { name: "Goal" }), " Again.");
    await user.keyboard("{Control>}{Enter}{/Control}");
    const dialog = await screen.findByRole("dialog", {
      name: "Board UI changed while you were editing",
    });
    expect(dialog.textContent).toContain("Theirs.");
    expect(dialog.textContent).toContain("workstream");
  });

  it("renames the workstream in place and moves to its new slug (FH-167)", async () => {
    const user = userEvent.setup();
    const onRenamed = vi.fn();
    renderPage(workstream(), true, onRenamed);
    await user.click(screen.getByRole("button", { name: "Edit name" }));
    const name = screen.getByRole("textbox", { name: "Name" });
    expect((name as HTMLInputElement).value).toBe("Board UI");
    expect(document.activeElement).toBe(name);
    // The page keeps its h1 for the outline while the name is edited.
    expect(
      screen.getByRole("heading", { level: 1, name: "Board UI" }),
    ).toBeTruthy();
    server.replies.push(() =>
      Response.json({ slug: "board-lanes", hash: "renamed", revision: 3 }),
    );
    await user.clear(name);
    await user.type(name, "  Board lanes ");
    await user.keyboard("{Control>}{Enter}{/Control}");
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Name" })).toBeNull(),
    );
    expect(server.requests).toEqual([
      {
        method: "PATCH",
        path: "/api/projects/alpha/workstreams/board-ui",
        body: {
          base: "base",
          fields: { title: "Board lanes" },
          slugFromTitle: true,
        },
      },
    ]);
    expect(onRenamed).toHaveBeenCalledWith("board-lanes");
    expect(editing.notify).toHaveBeenCalledWith(
      expect.objectContaining({ text: "Renamed Board UI to Board lanes." }),
    );
  });

  it("keeps the slug's page when the new name makes the same slug", async () => {
    const user = userEvent.setup();
    const onRenamed = vi.fn();
    renderPage(workstream(), true, onRenamed);
    await user.click(screen.getByRole("button", { name: "Edit name" }));
    server.replies.push(() =>
      Response.json({ slug: "board-ui", hash: "renamed", revision: 3 }),
    );
    const name = screen.getByRole("textbox", { name: "Name" });
    await user.clear(name);
    await user.type(name, "Board ui");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(server.requests).toHaveLength(1));
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Name" })).toBeNull(),
    );
    expect(onRenamed).not.toHaveBeenCalled();
    expect(resource.reloads).toBeGreaterThan(0);
  });

  it("keeps the name open with the reason when the name is taken", async () => {
    const user = userEvent.setup();
    const onRenamed = vi.fn();
    renderPage(workstream(), true, onRenamed);
    await user.click(screen.getByRole("button", { name: "Edit name" }));
    const words =
      'Another workstream in alpha already has the short name offline that "Offline" would take; choose another name.';
    server.replies.push(() =>
      Response.json(
        { error: { code: "exists", message: words } },
        { status: 409 },
      ),
    );
    const name = screen.getByRole("textbox", { name: "Name" });
    await user.clear(name);
    await user.type(name, "Offline");
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect((await screen.findByRole("alert")).textContent).toBe(words);
    expect(
      (screen.getByRole("textbox", { name: "Name" }) as HTMLInputElement).value,
    ).toBe("Offline");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(onRenamed).not.toHaveBeenCalled();
  });

  it("refuses an empty name without sending it, and Escape cancels", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(screen.getByRole("button", { name: "Edit name" }));
    await user.clear(screen.getByRole("textbox", { name: "Name" }));
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(screen.getByRole("alert").textContent).toBe(
      "A workstream needs a name.",
    );
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Name" })).toBeNull();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Edit name" }),
    );
    expect(server.requests).toEqual([]);
  });

  it("offers no edits where the board cannot save", () => {
    renderPage(workstream(), false);
    expect(screen.queryByRole("button", { name: /^Edit / })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Add / })).toBeNull();
    renderPage(workstream({ hash: "" }));
    expect(screen.queryByRole("button", { name: /^Edit / })).toBeNull();
  });
});
