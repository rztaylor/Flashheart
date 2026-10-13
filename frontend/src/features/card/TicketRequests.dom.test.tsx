import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Comment, TicketDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import type { Editing } from "../editing/useEditing";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// Change requests on the ticket's full page (REV-6, FH-126): Request
// changes on a ticket in review, the list of requests at the top of the
// page with Resolve and Reopen, and a request's state on its comment. The
// ticket's useResource is mocked; writes go to a fake server that records
// them.
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
    body: "The header wraps.",
    ...overrides,
  };
}

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
      return reply
        ? reply()
        : Response.json({
            comment: comment(),
            activityHash: "a2",
            revision: 3,
          });
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

function renderPage(data: TicketDetail) {
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
      editing={editing}
      boardLink={() => ({ href: "#/p/flashheart/board", label: "Flashheart" })}
    />,
  );
}

const dialog = () =>
  screen.getByRole("dialog", { name: "Request changes on FH-65" });
const properties = () => screen.getByRole("region", { name: "Properties" });

beforeEach(() => {
  server = fakeServer();
  vi.stubGlobal("IntersectionObserver", FakeObserver);
  vi.mocked(editing.notify).mockClear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe("Request changes (REV-6)", () => {
  it("sends chosen comments and a new one back, and says where they went", async () => {
    const user = userEvent.setup();
    renderPage(
      detail({
        comments: [
          comment({ id: "c1", body: "The header wraps." }),
          comment({
            id: "c2",
            state: "change-requested",
            body: "Rename the button.",
          }),
        ],
        activityHash: "a1",
      }),
    );
    await user.click(
      within(properties()).getByRole("button", { name: "Request changes" }),
    );
    const boxes = within(dialog()).getAllByRole("checkbox");
    // Newest first; the open request comes checked.
    expect(boxes.map((box) => (box as HTMLInputElement).checked)).toEqual([
      true,
      false,
    ]);
    await user.click(
      within(dialog()).getByRole("checkbox", { name: /The header wraps/ }),
    );
    await user.type(
      within(dialog()).getByRole("textbox", { name: "New change request" }),
      "Keep the old label.",
    );
    server.replies.push(() =>
      Response.json({
        comments: [comment(), comment({ id: "c2" }), comment({ id: "c3" })],
        hash: "h2",
        activityHash: "a2",
        revision: 4,
        sentTo: "codex:019a7c00-0000-7000-8000-000000000001",
        sentToShort: "codex:00000001",
        warnings: [],
      }),
    );
    await user.click(
      within(dialog()).getByRole("button", { name: "Request changes" }),
    );
    expect(server.requests).toEqual([
      {
        method: "POST",
        path: "/api/tickets/FH-65/request-changes",
        body: {
          comments: ["c1", "c2"],
          comment: { body: "Keep the old label.", quote: "", quoteFrom: "" },
        },
      },
    ]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(editing.notify).toHaveBeenCalledWith(
      expect.objectContaining({
        text: "Requested 3 changes on FH-65; sent to codex:00000001.",
      }),
    );
  });

  it("needs a comment or new text, and sends with Ctrl and Enter", async () => {
    const user = userEvent.setup();
    renderPage(detail({ comments: [] }));
    await user.click(
      within(properties()).getByRole("button", { name: "Request changes" }),
    );
    await user.click(
      within(dialog()).getByRole("button", { name: "Request changes" }),
    );
    expect(within(dialog()).getByRole("alert").textContent).toBe(
      "Choose a comment or write a change to request.",
    );
    expect(server.requests).toEqual([]);

    server.replies.push(() =>
      Response.json({ comments: [comment()], revision: 4, sentTo: "" }),
    );
    await user.type(
      within(dialog()).getByRole("textbox", { name: "New change request" }),
      "Fix it.{Control>}{Enter}{/Control}",
    );
    expect(server.requests[0]?.body).toEqual({
      comments: [],
      comment: { body: "Fix it.", quote: "", quoteFrom: "" },
    });
    await waitFor(() =>
      expect(editing.notify).toHaveBeenCalledWith(
        expect.objectContaining({
          text: "Requested 1 change on FH-65; the next session on it will see it.",
        }),
      ),
    );
  });

  it("keeps the dialog open with the reason when the request fails", async () => {
    const user = userEvent.setup();
    renderPage(detail({ comments: [] }));
    await user.click(
      within(properties()).getByRole("button", { name: "Request changes" }),
    );
    server.replies.push(() =>
      Response.json(
        {
          error: {
            code: "not_in_review",
            message: "This ticket is no longer in review.",
          },
        },
        { status: 409 },
      ),
    );
    await user.type(
      within(dialog()).getByRole("textbox", { name: "New change request" }),
      "Fix it.",
    );
    await user.click(
      within(dialog()).getByRole("button", { name: "Request changes" }),
    );
    await waitFor(() =>
      expect(within(dialog()).getByRole("alert").textContent).toContain(
        "no longer in review",
      ),
    );
  });

  it("is offered only on a ticket in review", () => {
    renderPage(detail({ column: "in-progress", comments: [] }));
    expect(
      screen.queryByRole("button", { name: "Request changes" }),
    ).toBeNull();
  });
});

describe("the change requests list (REV-6)", () => {
  const requests = () =>
    detail({
      column: "in-progress",
      comments: [
        comment({ id: "c1", state: "change-requested", body: "Rename it." }),
        comment({ id: "c2", state: "addressed", body: "Fix the header." }),
        comment({ id: "c3", state: "resolved", body: "Keep the label." }),
        comment({ id: "c4", body: "Nice work." }),
      ],
      activityHash: "a1",
    });
  const list = () => screen.getByRole("region", { name: /^Change requests/ });

  it("heads the page with the requests and their counts", () => {
    renderPage(requests());
    const region = list();
    expect(region.textContent).toContain("1 open · 1 addressed");
    const rows = within(region)
      .getAllByRole("listitem")
      .map((row) => row.textContent ?? "");
    expect(rows).toHaveLength(3);
    expect(rows[0]).toContain("Change requested");
    expect(rows[0]).toContain("Waiting for the agent");
    expect(rows[1]).toContain("Addressed");
    expect(rows[2]).toContain("Resolved");
    // The page's first section, before the criteria.
    const sections = [...document.querySelectorAll("[data-section]")].map(
      (section) => section.getAttribute("data-section"),
    );
    expect(sections[0]).toBe("requests");
    // Each request's comment in Activity shows its state too; a plain
    // comment shows none.
    const activity = screen.getByRole("region", { name: "Activity" });
    expect(within(activity).getAllByText("Change requested")).toHaveLength(1);
    expect(within(activity).getAllByText("Addressed")).toHaveLength(1);
  });

  it("resolves and reopens requests, each saying so", async () => {
    const user = userEvent.setup();
    renderPage(requests());
    server.replies.push(() =>
      Response.json({
        comment: comment({ id: "c2", state: "change-requested" }),
        activityHash: "a2",
        revision: 4,
        sentTo: "claude:3f2a9c1e-x",
        sentToShort: "claude:3f2a9c1e",
      }),
    );
    await user.click(
      within(list()).getByRole("button", { name: "Reopen “Fix the header.”" }),
    );
    expect(server.requests.at(-1)).toEqual({
      method: "PATCH",
      path: "/api/tickets/FH-65/comments/c2",
      body: { state: "change-requested" },
    });
    await waitFor(() =>
      expect(editing.notify).toHaveBeenCalledWith(
        expect.objectContaining({
          text: "Reopened the change request on FH-65; sent to claude:3f2a9c1e.",
        }),
      ),
    );
    await user.click(
      within(list()).getByRole("button", { name: "Resolve “Rename it.”" }),
    );
    expect(server.requests.at(-1)?.body).toEqual({ state: "resolved" });
    await waitFor(() =>
      expect(editing.notify).toHaveBeenCalledWith(
        expect.objectContaining({
          text: "Resolved the change request on FH-65.",
        }),
      ),
    );
    await user.click(
      within(list()).getByRole("button", { name: "Reopen “Keep the label.”" }),
    );
    expect(server.requests.at(-1)).toMatchObject({
      path: "/api/tickets/FH-65/comments/c3",
      body: { state: "change-requested" },
    });
  });

  it("keeps a failed change's reason beside it", async () => {
    const user = userEvent.setup();
    renderPage(requests());
    server.replies.push(() =>
      Response.json(
        { error: { code: "write_failed", message: "Disk full" } },
        { status: 500 },
      ),
    );
    const resolve = within(list()).getByRole("button", {
      name: "Resolve “Rename it.”",
    });
    await user.click(resolve);
    await waitFor(() =>
      expect(within(list()).getByRole("alert").textContent).toContain(
        "Disk full",
      ),
    );
    expect(document.activeElement).toBe(resolve);
  });
});
