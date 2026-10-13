import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Comment, TicketDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import type { Editing } from "../editing/useEditing";
import { CardPanel } from "./CardPanel";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// Deleting a comment posted by mistake (FH-140), on the full page and the
// card panel's Comments tab: the Delete control, its confirmation, and the
// reason a comment its session has read or an open change request stays.
// The ticket's useResource is mocked; writes go to a fake server that
// records them.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
  reloads: 0,
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({
    ...resource.state,
    reload: () => {
      resource.reloads++;
    },
  }),
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

function comment(overrides: Partial<Comment> = {}): Comment {
  return {
    id: "c1",
    time: "2026-10-12T12:10:00Z",
    by: "Robert",
    byShort: "Robert",
    byPerson: true,
    state: "open",
    edited: "",
    quoteFrom: "",
    quote: "",
    body: "Posted on the wrong ticket.",
    sentTo: "",
    sentToShort: "",
    delivery: "",
    delivered: "",
    ...overrides,
  };
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
        : Response.json({ comment: "c1", activityHash: "a2", revision: 3 });
    },
  ) as unknown as AuthenticatedFetch;
  return { fetcher, requests, replies };
}

const editing = {
  move: vi.fn(),
  archive: vi.fn(),
  notify: vi.fn(),
  answer: vi.fn(),
} as unknown as Editing;

let server: ReturnType<typeof fakeServer>;

const props = (data: TicketDetail) => ({
  ticket: { id: data.id },
  fetcher: server.fetcher,
  lines: new Map(),
  workstreams: new Map(),
  keys: new Set(["FH"]),
  onOpen: () => undefined,
  onClose: () => undefined,
  revision: 0,
  editing,
});

interface Surface {
  name: string;
  render(data: TicketDetail): void;
  comments(): HTMLElement;
}

const surfaces: Surface[] = [
  {
    name: "the full page",
    render: (data) => {
      resource.state = { status: "ready", data };
      render(
        <TicketPage
          {...props(data)}
          boardLink={() => ({ href: "#/p/flashheart/board", label: "x" })}
        />,
      );
    },
    comments: () => screen.getByRole("region", { name: "Activity" }),
  },
  {
    name: "the card panel",
    render: (data) => {
      resource.state = { status: "ready", data };
      render(<CardPanel {...props(data)} initialTab="comments" />);
    },
    comments: () => screen.getByRole("tabpanel", { name: /^Comments/ }),
  },
];

const withComments = (comments: Comment[]) =>
  detail({ activityHash: "a1", comments });

beforeEach(() => {
  server = fakeServer();
  resource.reloads = 0;
  vi.mocked(editing.notify).mockClear();
  vi.stubGlobal("IntersectionObserver", FakeObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe.each(surfaces)("deleting a comment on $name (FH-140)", (surface) => {
  const item = (text: string) =>
    within(surface.comments())
      .getAllByRole("listitem")
      .find((li) => li.textContent?.includes(text)) as HTMLElement;
  const deleteIn = (text: string) =>
    within(item(text)).getByRole("button", { name: "Delete comment" });

  it("asks first, then deletes on the activity's hash and says so", async () => {
    const user = userEvent.setup();
    surface.render(
      withComments([
        comment({
          sentTo: "claude:aaaa1111-2222",
          sentToShort: "claude:aaaa1111",
          delivery: "waiting",
        }),
      ]),
    );
    await user.click(deleteIn("Posted on the wrong ticket."));
    const dialog = screen.getByRole("dialog", { name: "Delete this comment?" });
    expect(dialog.textContent).toContain("Posted on the wrong ticket.");
    expect(dialog.textContent).toContain(
      "claude:aaaa1111 has not taken it yet and never will.",
    );
    expect(server.requests).toEqual([]);
    await user.click(
      within(dialog).getByRole("button", { name: "Delete comment" }),
    );
    expect(server.requests).toEqual([
      {
        method: "DELETE",
        path: "/api/tickets/FH-65/comments/c1",
        body: { base: "a1" },
      },
    ]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(editing.notify).toHaveBeenCalledWith({
      text: "Deleted your comment on FH-65.",
    });
    expect(resource.reloads).toBeGreaterThan(0);
  });

  it("works from the keyboard, and Keep it or Escape deletes nothing", async () => {
    const user = userEvent.setup();
    surface.render(withComments([comment()]));
    deleteIn("Posted on the wrong ticket.").focus();
    await user.keyboard("{Enter}");
    const dialog = screen.getByRole("dialog", { name: "Delete this comment?" });
    expect(document.activeElement).toBe(
      within(dialog).getByRole("button", { name: "Keep it" }),
    );
    await user.keyboard("{Enter}");
    expect(screen.queryByRole("dialog")).toBeNull();
    deleteIn("Posted on the wrong ticket.").focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(server.requests).toEqual([]);
  });

  it("says why a comment its session read, or an open request, stays", async () => {
    const user = userEvent.setup();
    surface.render(
      withComments([
        comment({
          id: "c1",
          body: "Read already.",
          sentTo: "claude:aaaa1111-2222",
          sentToShort: "claude:aaaa1111",
          delivery: "delivered",
          delivered: "2026-10-12T12:11:00Z",
        }),
        comment({ id: "c2", body: "Rename it.", state: "change-requested" }),
        comment({ id: "c3", body: "Done now.", state: "addressed" }),
        comment({ id: "c4", body: "Was fine.", state: "resolved" }),
      ]),
    );
    const reasons: Record<string, string> = {
      "Read already.": "It can't be deleted: claude:aaaa1111 has read it.",
      "Rename it.":
        "It can't be deleted while the change it asks for is open. Resolve it first.",
      "Done now.":
        "It can't be deleted while the change request waits for you. Resolve it first.",
    };
    for (const [text, reason] of Object.entries(reasons)) {
      const control = deleteIn(text);
      expect(control.getAttribute("aria-disabled")).toBe("true");
      const described = document.getElementById(
        control.getAttribute("aria-describedby") ?? "",
      );
      expect(described?.textContent).toBe(reason);
      expect(described?.hidden).toBe(true);
      await user.click(control);
      expect(described?.hidden).toBe(false);
      expect(control.getAttribute("aria-expanded")).toBe("true");
      expect(screen.queryByRole("dialog")).toBeNull();
    }
    // A resolved request can go.
    expect(deleteIn("Was fine.").getAttribute("aria-disabled")).toBeNull();
    expect(server.requests).toEqual([]);
  });

  it("shows the server's refusal in the dialog and reloads", async () => {
    const user = userEvent.setup();
    surface.render(withComments([comment()]));
    server.replies.push(() =>
      Response.json(
        {
          error: {
            code: "delivered",
            message:
              "This comment was delivered to its session, which has read it, so it can no longer be deleted.",
          },
        },
        { status: 409 },
      ),
    );
    await user.click(deleteIn("Posted on the wrong ticket."));
    const dialog = screen.getByRole("dialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Delete comment" }),
    );
    expect((await within(dialog).findByRole("alert")).textContent).toContain(
      "can no longer be deleted",
    );
    expect(resource.reloads).toBeGreaterThan(0);
    expect(editing.notify).not.toHaveBeenCalled();
  });

  it("offers no Delete without a way to save", () => {
    surface.render(detail({ activityHash: "", comments: [comment()] }));
    expect(
      within(surface.comments()).queryByRole("button", {
        name: "Delete comment",
      }),
    ).toBeNull();
  });
});
