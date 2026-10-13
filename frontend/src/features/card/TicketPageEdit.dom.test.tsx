import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { TicketDetail, WorkstreamBrief } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { leave } from "../../state/leaveGuard";
import { BlockedMoveDialog } from "../editing/BlockedMoveDialog";
import { type Editing, useEditing } from "../editing/useEditing";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// The ticket's useResource is mocked, so the page renders a loaded ticket
// without a server; saves go to a fake server that records them.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({ ...resource.state, reload: () => undefined }),
}));

class FakeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

interface Request {
  method: string;
  path: string;
  body: Record<string, unknown>;
}

// server answers each write with replies in turn (a saved ticket when none
// is left) and records what was sent.
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

const conflictReply = () =>
  Response.json(
    {
      error: { code: "conflict", message: "This ticket changed." },
      current: { hash: "newer", content: "## Description\n\nTheirs." },
    },
    { status: 409 },
  );

const editing = {
  move: vi.fn(),
  archive: vi.fn(),
  notify: vi.fn(),
  answer: vi.fn(),
} as unknown as Editing;

const workstreams = new Map<string, WorkstreamBrief[]>([
  [
    "flashheart",
    [
      {
        slug: "ticket-editing",
        title: "Ticket editing",
        created: "2026-10-01",
        status: "active",
        done: 1,
        total: 3,
      },
    ],
  ],
]);

let server: ReturnType<typeof fakeServer>;

function renderPage(data: TicketDetail = detail(), withEditing = true) {
  resource.state = { status: "ready", data };
  return render(
    <TicketPage
      ticket={{ id: data.id }}
      fetcher={server.fetcher}
      lines={new Map()}
      workstreams={workstreams}
      keys={new Set(["FH"])}
      onOpen={() => undefined}
      onClose={() => undefined}
      revision={0}
      editing={withEditing ? editing : undefined}
      boardLink={() => ({ href: "#/p/flashheart/board", label: "Flashheart" })}
    />,
  );
}

const pencil = (name: string) =>
  screen.getByRole("button", { name: `Edit ${name}` });
const save = () => screen.getByRole("button", { name: "Save" });
const sent = () => server.requests.map(({ path, body }) => ({ path, body }));

beforeEach(() => {
  server = fakeServer();
  vi.stubGlobal("IntersectionObserver", FakeObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe("opening an in-place edit (FH-66)", () => {
  it("names a pencil for the title, each ticket section and each editable property", () => {
    renderPage();
    for (const name of [
      "title",
      "acceptance criteria",
      "description",
      "test plan",
      "priority",
      "type",
      "workstream",
      "branch",
      "depends on",
      "tags",
    ])
      expect(pencil(name)).toBeTruthy();
    // Context is empty: it offers Add context in its place instead.
    expect(screen.queryByRole("button", { name: "Edit context" })).toBeNull();
    expect(screen.getByRole("button", { name: "Add context" })).toBeTruthy();
  });

  it("offers no edit of Handoff, the review, Activity or what is derived", () => {
    renderPage();
    for (const region of ["Handoff", "Summary", "Tests", "Activity", "Files"])
      expect(
        within(screen.getByRole("region", { name: new RegExp(`^${region}`) }))
          .queryAllByRole("button", { name: /^(Edit|Add) / })
          .map((button) => button.getAttribute("aria-label")),
      ).toEqual([]);
    for (const name of ["handoff", "activity", "review", "agent", "created"])
      expect(screen.queryByRole("button", { name: `Edit ${name}` })).toBeNull();
  });

  it("offers nothing to edit on a read-only server", () => {
    renderPage(detail(), false);
    expect(screen.queryAllByRole("button", { name: /^(Edit|Add) / })).toEqual(
      [],
    );
  });

  it("opens by the pencil, a double-click on the content, or Enter", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("description"));
    const editor = screen.getByRole("textbox", { name: "Description" });
    expect((editor as HTMLTextAreaElement).value).toBe(
      "Lay the ticket out in three columns.",
    );
    expect(document.activeElement).toBe(editor);
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    await user.dblClick(screen.getByText("Red first."));
    expect(screen.getByRole("textbox", { name: "Test plan" })).toBeTruthy();
    await user.keyboard("{Escape}");

    pencil("title").focus();
    await user.keyboard("{Enter}");
    expect(
      (screen.getByRole("textbox", { name: "Title" }) as HTMLInputElement)
        .value,
    ).toBe("Outline the page");
  });
});

describe("leaving an in-place edit", () => {
  it("saves only the section, on the version it opened, and returns focus", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("description"));
    const editor = screen.getByRole("textbox", { name: "Description" });
    await user.clear(editor);
    await user.type(editor, "Three columns, one page.");
    await user.click(save());
    expect(sent()).toEqual([
      {
        path: "/api/tickets/FH-65/section",
        body: {
          base: "abc",
          section: "Description",
          content: "Three columns, one page.",
        },
      },
    ]);
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Description" })).toBeNull(),
    );
    expect(document.activeElement).toBe(pencil("description"));
  });

  it("saves with Ctrl or Cmd and Enter, and cancels with Escape", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("title"));
    const title = screen.getByRole("textbox", { name: "Title" });
    await user.type(title, " again");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Title" })).toBeNull();
    expect(document.activeElement).toBe(pencil("title"));
    expect(server.requests).toEqual([]);

    await user.click(pencil("title"));
    await user.type(screen.getByRole("textbox", { name: "Title" }), " again");
    await user.keyboard("{Control>}{Enter}{/Control}");
    await user.click(pencil("branch"));
    await user.type(screen.getByRole("textbox", { name: "Branch" }), "-2");
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    expect(sent()).toEqual([
      {
        path: "/api/tickets/FH-65",
        body: { base: "abc", fields: { title: "Outline the page again" } },
      },
      {
        path: "/api/tickets/FH-65",
        body: {
          base: "abc",
          fields: { branch: "feature/ticket-page-outline-2" },
        },
      },
    ]);
    await waitFor(() => expect(document.activeElement).toBe(pencil("branch")));
  });

  it("writes nothing for an untouched save", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("test plan"));
    await user.click(save());
    expect(screen.queryByRole("textbox", { name: "Test plan" })).toBeNull();
    expect(server.requests).toEqual([]);
    expect(document.activeElement).toBe(pencil("test plan"));
  });

  it("keeps a failed save open with the error beside it", async () => {
    const user = userEvent.setup();
    renderPage();
    server.replies.push(() =>
      Response.json(
        { error: { code: "write_failed", message: "The disk is full." } },
        { status: 500 },
      ),
    );
    await user.click(pencil("test plan"));
    await user.type(screen.getByRole("textbox", { name: "Test plan" }), "!");
    await user.click(save());
    expect((await screen.findByRole("alert")).textContent).toContain(
      "The disk is full.",
    );
    expect(
      (
        screen.getByRole("textbox", {
          name: "Test plan",
        }) as HTMLTextAreaElement
      ).value,
    ).toBe("Red first.!");
    // Focus stays in the edit, so Escape still cancels it.
    expect(document.activeElement).toBe(save());
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Test plan" })).toBeNull();
    expect(document.activeElement).toBe(pencil("test plan"));
  });

  it("opens the conflict dialog when the file changed on disk (EDIT-7)", async () => {
    const user = userEvent.setup();
    renderPage();
    server.replies.push(conflictReply);
    await user.click(pencil("description"));
    await user.type(
      screen.getByRole("textbox", { name: "Description" }),
      " Mine.",
    );
    await user.click(save());
    const dialog = await screen.findByRole("dialog", {
      name: "FH-65 changed while you were editing",
    });
    expect(
      within(dialog).getByRole("region", { name: "Your version" }).textContent,
    ).toContain("Lay the ticket out in three columns. Mine.");
    expect(
      within(dialog).getByRole("region", { name: "Now on disk" }).textContent,
    ).toContain("Theirs.");
    await user.click(
      within(dialog).getByRole("button", { name: "Overwrite with mine" }),
    );
    expect(server.requests[1]?.body).toMatchObject({
      base: "newer",
      section: "Description",
    });
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Description" })).toBeNull(),
    );
  });
});

describe("one edit at a time", () => {
  it("asks to keep or discard unsaved changes before opening another item", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("description"));
    await user.type(
      screen.getByRole("textbox", { name: "Description" }),
      " More.",
    );
    await user.click(pencil("priority"));
    let ask = screen.getByRole("dialog", {
      name: "Unsaved changes to the description",
    });
    await user.click(within(ask).getByRole("button", { name: "Keep editing" }));
    const editor = screen.getByRole("textbox", { name: "Description" });
    expect((editor as HTMLTextAreaElement).value).toContain("More.");
    await waitFor(() => expect(document.activeElement).toBe(editor));
    expect(screen.queryByRole("combobox", { name: "Priority" })).toBeNull();

    await user.click(pencil("priority"));
    ask = screen.getByRole("dialog", {
      name: "Unsaved changes to the description",
    });
    await user.click(
      within(ask).getByRole("button", { name: "Discard changes" }),
    );
    expect(screen.queryByRole("textbox", { name: "Description" })).toBeNull();
    expect(screen.getByRole("combobox", { name: "Priority" })).toBeTruthy();
    expect(server.requests).toEqual([]);
  });

  it("switches at once when the open item has no changes", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("description"));
    await user.click(pencil("type"));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("textbox", { name: "Description" })).toBeNull();
    expect(screen.getByRole("combobox", { name: "Type" })).toBeTruthy();
  });

  it("asks before leaving the page with unsaved changes", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("title"));
    await user.type(screen.getByRole("textbox", { name: "Title" }), "!");
    const go = vi.fn();
    act(() => leave(go));
    const ask = screen.getByRole("dialog", {
      name: "Unsaved changes to the title",
    });
    await user.click(within(ask).getByRole("button", { name: "Keep editing" }));
    expect(go).not.toHaveBeenCalled();
    act(() => leave(go));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "Unsaved changes to the title" }),
      ).getByRole("button", { name: "Discard changes" }),
    );
    expect(go).toHaveBeenCalledOnce();
    // Discarded: leaving again asks nothing.
    act(() => leave(go));
    expect(go).toHaveBeenCalledTimes(2);
  });
});

describe("typed property controls (EDIT-6)", () => {
  it("edits priority, type and workstream with selects", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("priority"));
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Priority" }),
      "high",
    );
    await user.click(save());
    await user.click(pencil("type"));
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Type" }),
      "bug",
    );
    await user.click(save());
    await user.click(pencil("workstream"));
    const picker = screen.getByRole("combobox", { name: "Workstream" });
    expect(
      within(picker)
        .getAllByRole("option")
        .map((option) => option.textContent),
    ).toEqual(["None", "Ticket editing"]);
    await user.selectOptions(picker, "ticket-editing");
    await user.click(save());
    expect(sent().map((request) => request.body.fields)).toEqual([
      { priority: "high" },
      { type: "bug" },
      { workstream: "ticket-editing" },
    ]);
  });

  it("edits tags as chips", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("tags"));
    const tags = screen.getByRole("group", { name: "Tags" });
    await user.click(
      within(tags).getByRole("button", { name: "Remove tag ui" }),
    );
    await user.type(
      within(tags).getByRole("textbox", { name: "Add a tag" }),
      "a11y{Enter}editor",
    );
    expect(
      within(tags).getByRole("button", { name: "Remove tag a11y" }),
    ).toBeTruthy();
    // Text not yet added as a chip is saved with the rest.
    await user.click(save());
    expect(sent()[0]?.body.fields).toEqual({
      tags: ["ticket-page", "a11y", "editor"],
    });
  });

  it("edits depends on as a ticket picker that takes ticket ids only", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("depends on"));
    const picker = screen.getByRole("group", { name: "Depends on" });
    const input = within(picker).getByRole("combobox", {
      name: "Add a ticket",
    });
    await user.type(input, "nope{Enter}");
    expect(within(picker).getByText(/is not a ticket id/)).toBeTruthy();
    await user.clear(input);
    await user.type(input, "fh-12{Enter}");
    await user.click(
      within(picker).getByRole("button", { name: "Remove FH-99" }),
    );
    await user.click(save());
    expect(sent()[0]?.body.fields).toEqual({ "depends-on": ["FH-12"] });
  });

  it("refuses an empty title without saving", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("title"));
    await user.clear(screen.getByRole("textbox", { name: "Title" }));
    await user.click(save());
    expect(screen.getByRole("alert").textContent).toContain("needs a title");
    expect(server.requests).toEqual([]);
  });
});

// The page with the app's real move coordination, so a blocked move shows
// the dialog the Shell shows (EDIT-2).
function PageWithMoves() {
  const moves = useEditing(server.fetcher, () => undefined);
  return (
    <>
      <TicketPage
        ticket={{ id: "FH-65" }}
        fetcher={server.fetcher}
        lines={new Map()}
        workstreams={workstreams}
        keys={new Set(["FH"])}
        onOpen={() => undefined}
        onClose={() => undefined}
        revision={0}
        editing={moves}
        boardLink={() => ({ href: "#/", label: "Flashheart" })}
      />
      {moves.blocked ? (
        <BlockedMoveDialog
          move={moves.blocked}
          onConfirm={moves.confirmBlocked}
          onCancel={moves.cancelBlocked}
        />
      ) : null}
    </>
  );
}

describe("the status", () => {
  it("moves with the status select, and a blocked start opens the blocked move dialog", async () => {
    const user = userEvent.setup();
    resource.state = {
      status: "ready",
      data: detail({
        column: "backlog",
        blocked: true,
        blockedBy: [
          { kind: "ticket", text: "Waits for FH-99", missing: false },
        ],
      }),
    };
    server.replies.push(() =>
      Response.json(
        {
          error: {
            code: "confirm_blocked",
            message: "This ticket is blocked.",
            blockedBy: ["Waits for FH-99"],
          },
        },
        { status: 409 },
      ),
    );
    render(<PageWithMoves />);
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Move to" }),
      "in-progress",
    );
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Waits for FH-99");
    expect(sent()[0]).toMatchObject({
      path: "/api/tickets/FH-65/move",
      body: { to: "in-progress" },
    });
  });
});

describe("sections", () => {
  it("adds an empty section in place", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(screen.getByRole("button", { name: "Add context" }));
    await user.type(
      screen.getByRole("textbox", { name: "Context" }),
      "Why it matters.",
    );
    await user.click(save());
    expect(sent()).toEqual([
      {
        path: "/api/tickets/FH-65/section",
        body: { base: "abc", section: "Context", content: "Why it matters." },
      },
    ]);
    await waitFor(() =>
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: "Add context" }),
      ),
    );
  });

  it("adds, rewords, removes and reorders criteria, keeping what is ticked", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(pencil("acceptance criteria"));
    const list = screen.getByRole("group", { name: "Acceptance criteria" });
    const criterion = (n: number) =>
      within(list).getByRole("textbox", { name: `Criterion ${n}` });
    expect((criterion(1) as HTMLInputElement).value).toBe("Three columns");
    await user.clear(criterion(2));
    await user.type(criterion(2), "No tabs anywhere");
    await user.click(
      within(list).getByRole("button", { name: "Add a criterion" }),
    );
    expect(document.activeElement).toBe(criterion(3));
    await user.type(criterion(3), "Keyboard first");
    await user.click(
      within(list).getByRole("button", { name: "Move criterion 3 up" }),
    );
    await user.click(
      within(list).getByRole("button", { name: "Remove criterion 1" }),
    );
    await user.click(save());
    expect(sent()).toEqual([
      {
        path: "/api/tickets/FH-65/section",
        body: {
          base: "abc",
          section: "Acceptance Criteria",
          content: "- [ ] Keyboard first\n- [ ] No tabs anywhere",
        },
      },
    ]);
  });

  it("keeps ticking a criterion one click while nothing is open", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(screen.getByRole("checkbox", { name: "No tabs" }));
    expect(sent()).toEqual([
      {
        path: "/api/tickets/FH-65/criteria",
        body: { base: "abc", index: 1, checked: true },
      },
    ]);
  });
});
