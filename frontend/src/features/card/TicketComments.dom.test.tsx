import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Comment, History, TicketDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { leave } from "../../state/leaveGuard";
import type { Editing } from "../editing/useEditing";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// Comments and the Activity filter on the ticket's full page (FH-125). The
// ticket's useResource is mocked, so the page renders a loaded ticket
// without a server; writes go to a fake server that records them.
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

const saved = (overrides: Partial<Comment> = {}) =>
  Response.json(
    { comment: comment(overrides), activityHash: "a2", revision: 3 },
    { status: 201 },
  );

function fakeServer() {
  const requests: Request[] = [];
  const replies: (() => Response | Promise<Response>)[] = [];
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
      return reply ? reply() : saved();
    },
  ) as unknown as AuthenticatedFetch;
  return { fetcher, requests, replies };
}

function comment(overrides: Partial<Comment> = {}): Comment {
  return {
    id: "c1",
    time: "2026-10-12T12:10:00Z",
    by: "Robert",
    byPerson: true,
    state: "open",
    edited: "",
    quoteFrom: "",
    quote: "",
    body: "Keep the outline.",
    ...overrides,
  };
}

function history(overrides: Partial<History> = {}): History {
  return {
    id: "h1",
    time: "2026-10-11T08:00:00Z",
    by: "Robert",
    byPerson: true,
    changes: [
      {
        what: "created",
        from: "",
        to: "backlog",
        fields: [],
        criteria: [],
        steps: [],
      },
    ],
    ...overrides,
  };
}

const editing = {
  move: vi.fn(),
  archive: vi.fn(),
  notify: vi.fn(),
  answer: vi.fn(),
} as unknown as Editing;

let server: ReturnType<typeof fakeServer>;

function renderPage(data: TicketDetail = detail(), withEditing = true) {
  resource.state = { status: "ready", data };
  return render(
    <TicketPage
      ticket={{ id: data.id }}
      fetcher={server.fetcher}
      lines={new Map()}
      workstreams={new Map()}
      keys={new Set(["FH"])}
      onOpen={() => undefined}
      onClose={() => undefined}
      revision={0}
      editing={withEditing ? editing : undefined}
      boardLink={() => ({ href: "#/p/flashheart/board", label: "Flashheart" })}
    />,
  );
}

const withActivity = () =>
  detail({
    comments: [
      comment(),
      comment({
        id: "c2",
        time: "2026-10-13T09:00:00Z",
        edited: "2026-10-13T09:05:00Z",
        quoteFrom: "Acceptance criteria 2",
        quote: "No tabs",
        body: "Not even on narrow screens.",
      }),
    ],
    history: [
      history(),
      history({
        id: "h2",
        time: "2026-10-12T12:40:00Z",
        by: "claude:aaaa1111-2222",
        byPerson: false,
        changes: [
          {
            what: "moved",
            from: "in-progress",
            to: "review",
            fields: [],
            criteria: [],
            steps: [],
          },
        ],
      }),
    ],
    activityHash: "a1",
  });

const activity = () => screen.getByRole("region", { name: "Activity" });
const box = () => screen.getByRole("textbox", { name: "Add a comment" });
const commentButton = () =>
  screen.getByRole("button", { name: "Comment" }) as HTMLButtonElement;
const items = () =>
  within(activity())
    .getAllByRole("listitem")
    .map((item) => item.textContent ?? "");
const outlineActivity = () =>
  within(
    screen.getAllByRole("navigation", { name: "Sections" })[0] as HTMLElement,
  ).getByRole("button", { name: /^Activity/ });

beforeEach(() => {
  server = fakeServer();
  vi.stubGlobal("IntersectionObserver", FakeObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe("the comment box (FH-125)", () => {
  it("ends Activity and posts a comment, then clears", async () => {
    const user = userEvent.setup();
    renderPage();
    expect(
      within(activity()).getByRole("textbox", { name: "Add a comment" }),
    ).toBeTruthy();
    await user.type(box(), "Looks right.");
    await user.click(commentButton());
    expect(server.requests).toEqual([
      {
        method: "POST",
        path: "/api/tickets/FH-65/comments",
        body: { body: "Looks right.", quote: "", quoteFrom: "" },
      },
    ]);
    await waitFor(() => expect((box() as HTMLTextAreaElement).value).toBe(""));
    expect(editing.notify).toHaveBeenCalledWith(
      expect.objectContaining({ text: "Commented on FH-65." }),
    );
  });

  it("posts with Ctrl or Cmd and Enter, and never posts an empty comment", async () => {
    const user = userEvent.setup();
    renderPage();
    expect(commentButton().disabled).toBe(true);
    await user.type(box(), "   ");
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(commentButton().disabled).toBe(true);
    expect(server.requests).toEqual([]);
    await user.type(box(), "Ship it");
    await user.keyboard("{Meta>}{Enter}{/Meta}");
    expect(server.requests.map((request) => request.body.body)).toEqual([
      "   Ship it",
    ]);
  });

  it("keeps the text with the error beside it when a post fails", async () => {
    const user = userEvent.setup();
    renderPage();
    server.replies.push(() =>
      Response.json(
        { error: { code: "write_failed", message: "The disk is full." } },
        { status: 500 },
      ),
    );
    await user.type(box(), "Keep me.");
    await user.click(commentButton());
    expect(
      (await within(activity()).findByRole("alert")).textContent,
    ).toContain("The disk is full.");
    expect((box() as HTMLTextAreaElement).value).toBe("Keep me.");
  });

  it("quotes a section, criterion or How to verify step chosen from the page", async () => {
    const user = userEvent.setup();
    renderPage();
    const picker = within(activity()).getByRole("combobox", {
      name: "Quote",
    });
    const options = within(picker)
      .getAllByRole("option")
      .map((option) => option.textContent);
    expect(options).toEqual(
      expect.arrayContaining([
        "No quote",
        "Description",
        "1. Three columns",
        "2. No tabs",
        "1. Open the page",
      ]),
    );
    await user.selectOptions(picker, "2. No tabs");
    const quote = within(activity()).getByRole("figure", {
      name: "Quoting Acceptance criteria 2",
    });
    expect(quote.textContent).toContain("No tabs");
    await user.type(box(), "Not on narrow screens either.");
    await user.click(commentButton());
    expect(server.requests[0]?.body).toEqual({
      body: "Not on narrow screens either.",
      quote: "No tabs",
      quoteFrom: "Acceptance criteria 2",
    });
    // Posted: the quote is cleared with the text.
    await waitFor(() =>
      expect(
        within(activity()).queryByRole("figure", { name: /^Quoting/ }),
      ).toBeNull(),
    );
  });

  it("asks before leaving the page with a comment not posted", async () => {
    const user = userEvent.setup();
    renderPage();
    const go = vi.fn();
    await user.type(box(), "Half a thought");
    act(() => leave(go));
    const ask = screen.getByRole("dialog", {
      name: "Unsaved changes to the comment",
    });
    await user.click(within(ask).getByRole("button", { name: "Keep editing" }));
    expect(go).not.toHaveBeenCalled();
    expect((box() as HTMLTextAreaElement).value).toBe("Half a thought");
    act(() => leave(go));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "Unsaved changes to the comment" }),
      ).getByRole("button", { name: "Discard changes" }),
    );
    expect(go).toHaveBeenCalledOnce();
    act(() => leave(go));
    expect(go).toHaveBeenCalledTimes(2);
  });

  it("is not offered on a read-only page", () => {
    renderPage(withActivity(), false);
    expect(within(activity()).queryByRole("textbox")).toBeNull();
    expect(
      within(activity()).queryByRole("button", { name: "Edit comment" }),
    ).toBeNull();
  });
});

describe("comments in Activity", () => {
  it("shows comments with their quote and whether they were edited, among the rest in time order", () => {
    renderPage(withActivity());
    const list = items();
    expect(list).toHaveLength(7);
    // A day's notes, which carry only their day, come before its timed
    // entries (D46).
    expect(list[0]).toContain("Mocked the layout.");
    expect(list[1]).toContain("Robert");
    expect(list[1]).toContain("created it in Backlog");
    expect(list[2]).toContain("Drop the line strip?");
    expect(list[3]).toContain("Keep the outline.");
    expect(list[4]).toContain("updated the handoff");
    expect(list[5]).toContain("claude:aaaa1111");
    expect(list[5]).toContain("moved it from In progress to Ready to review");
    expect(list[6]).toContain("edited");
    const quote = within(activity()).getByRole("figure", {
      name: "Quoting Acceptance criteria 2",
    });
    expect(quote.textContent).toContain("No tabs");
  });

  it("edits a person's comment in place, on the activity's hash", async () => {
    const user = userEvent.setup();
    renderPage(withActivity());
    server.replies.push(() =>
      Response.json({
        comment: comment({ body: "Keep the outline, left.", edited: "x" }),
        activityHash: "a2",
        revision: 3,
      }),
    );
    const [first] = within(activity()).getAllByRole("button", {
      name: "Edit comment",
    });
    await user.click(first as HTMLElement);
    const editor = screen.getByRole("textbox", { name: "Comment" });
    expect(document.activeElement).toBe(editor);
    await user.type(editor, ", left.");
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(server.requests).toEqual([
      {
        method: "PATCH",
        path: "/api/tickets/FH-65/comments/c1",
        body: { base: "a1", body: "Keep the outline., left." },
      },
    ]);
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Comment" })).toBeNull(),
    );
  });

  it("opens the conflict dialog when the comments changed since they were read", async () => {
    const user = userEvent.setup();
    renderPage(withActivity());
    server.replies.push(() =>
      Response.json(
        {
          error: { code: "conflict", message: "changed" },
          current: {
            hash: "a9",
            content: [
              "# Activity: FH-65",
              "",
              "### 2026-10-12T12:10:00Z · comment · Robert · c1",
              "",
              "state: open",
              "",
              "Theirs.",
              "",
              "### 2026-10-13T09:00:00Z · comment · Robert · c2",
              "",
              "state: open",
              "",
              "Other.",
              "",
            ].join("\n"),
          },
        },
        { status: 409 },
      ),
    );
    const [first] = within(activity()).getAllByRole("button", {
      name: "Edit comment",
    });
    await user.click(first as HTMLElement);
    await user.type(screen.getByRole("textbox", { name: "Comment" }), " Mine.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("dialog", {
      name: "FH-65 changed while you were editing",
    });
    expect(
      within(dialog).getByRole("region", { name: "Your version" }).textContent,
    ).toContain("Keep the outline. Mine.");
    const disk = within(dialog).getByRole("region", { name: "Now on disk" });
    expect(disk.textContent).toContain("Theirs.");
    expect(disk.textContent).not.toContain("Other.");
    await user.click(
      within(dialog).getByRole("button", { name: "Overwrite with mine" }),
    );
    expect(server.requests[1]?.body).toEqual({
      base: "a9",
      body: "Keep the outline. Mine.",
    });
  });

  it("offers no edit of a comment whose author is not a person", () => {
    renderPage(
      detail({
        comments: [comment({ by: "claude:aaaa1111-2222", byPerson: false })],
        activityHash: "a1",
      }),
    );
    expect(
      within(activity()).queryByRole("button", { name: "Edit comment" }),
    ).toBeNull();
  });
});

describe("filtering Activity", () => {
  it("shows All, Comments or History, and the outline's count follows", async () => {
    const user = userEvent.setup();
    renderPage(withActivity());
    expect(outlineActivity().textContent).toContain("7");
    await user.click(
      within(activity()).getByRole("radio", { name: "Comments" }),
    );
    expect(items()).toHaveLength(2);
    expect(items().every((text) => text.includes("Robert"))).toBe(true);
    expect(outlineActivity().textContent).toContain("2");
    await user.click(
      within(activity()).getByRole("radio", { name: "History" }),
    );
    expect(items().map((text) => /created|moved/.test(text))).toEqual([
      true,
      true,
    ]);
    expect(outlineActivity().textContent).toContain("2");
    await user.click(within(activity()).getByRole("radio", { name: "All" }));
    expect(items()).toHaveLength(7);
  });

  it("says when a filter has nothing to show", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(
      within(activity()).getByRole("radio", { name: "History" }),
    );
    expect(within(activity()).getByText("No history yet.")).toBeTruthy();
    expect(outlineActivity().textContent).toBe("Activity");
  });
});
