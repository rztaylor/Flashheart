import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Comment, TellSession, TicketDetail } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import type { Editing } from "../editing/useEditing";
import { CardPanel } from "./CardPanel";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// Tell a session now (FH-127) in the comment box and a sent comment's
// delivery in the timeline, on the full page and the card panel's
// Comments tab. The ticket's useResource is mocked; writes go to a fake
// server that records them.
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
    body: "Use the blue one.",
    sentTo: "",
    sentToShort: "",
    delivery: "",
    delivered: "",
    ...overrides,
  };
}

const session: TellSession = {
  run: "claude:aaaa1111-2222",
  short: "claude:aaaa1111",
  agent: "claude",
  state: "working",
  parent: "",
  agentType: "",
};
const subagent: TellSession = {
  run: "claude:aaaa1111-2222/a1",
  short: "claude:aaaa1111",
  agent: "claude",
  state: "waiting",
  parent: session.run,
  agentType: "Explore",
};

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
        : Response.json(
            {
              comment: comment(),
              activityHash: "a2",
              revision: 3,
              sentTo: session.run,
              sentToShort: session.short,
            },
            { status: 201 },
          );
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

const tellBox = () =>
  screen.getByRole("checkbox", {
    name: "Tell a session now",
  }) as HTMLInputElement;
const box = () => screen.getByRole("textbox", { name: "Add a comment" });

beforeEach(() => {
  server = fakeServer();
  resource.reloads = 0;
  vi.stubGlobal("IntersectionObserver", FakeObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe.each(surfaces)(
  "telling a session now on $name (FH-127)",
  (surface) => {
    it("is disabled, saying why, when no live session holds the ticket", () => {
      surface.render(detail({ tellSessions: [] }));
      expect(tellBox().disabled).toBe(true);
      expect(tellBox().getAttribute("aria-describedby")).toBeTruthy();
      const reason = document.getElementById(
        tellBox().getAttribute("aria-describedby") ?? "",
      );
      expect(reason?.textContent).toBe("No live session holds FH-65 now.");
      expect(screen.queryByRole("combobox", { name: "Session" })).toBeNull();
    });

    it("sends the comment to the session chosen, by keyboard, then resets", async () => {
      const user = userEvent.setup();
      surface.render(detail({ tellSessions: [session, subagent] }));
      await user.type(box(), "Use the blue one.");
      // Tab to the option and check it with Space.
      tellBox().focus();
      await user.keyboard(" ");
      expect(tellBox().checked).toBe(true);
      const picker = within(surface.comments()).getByRole("combobox", {
        name: "Session",
      });
      const groups = Array.from(picker.querySelectorAll("optgroup")).map(
        (group) => [
          group.label,
          ...Array.from(group.querySelectorAll("option")).map(
            (option) => option.textContent,
          ),
        ],
      );
      expect(groups).toEqual([
        [
          "Session claude:aaaa1111",
          "claude:aaaa1111 · Working",
          "Explore subagent · Waiting",
        ],
      ]);
      expect((picker as HTMLSelectElement).value).toBe(session.run);
      await user.selectOptions(picker, "Explore subagent · Waiting");
      await user.click(
        screen.getByRole("button", { name: "Comment and tell" }),
      );
      expect(server.requests).toEqual([
        {
          method: "POST",
          path: "/api/tickets/FH-65/comments",
          body: {
            body: "Use the blue one.",
            quote: "",
            quoteFrom: "",
            sendTo: subagent.run,
          },
        },
      ]);
      expect(editing.notify).toHaveBeenCalledWith(
        expect.objectContaining({
          text: "Commented on FH-65 and sent it to claude:aaaa1111.",
        }),
      );
      // Per comment: the next one is posted plainly unless chosen again.
      await waitFor(() => expect(tellBox().checked).toBe(false));
      expect(screen.getByRole("button", { name: "Comment" })).toBeTruthy();
    });

    it("keeps the comment and reloads when the session is no longer live", async () => {
      const user = userEvent.setup();
      surface.render(detail({ tellSessions: [session] }));
      server.replies.push(() =>
        Response.json(
          {
            error: {
              code: "not_live",
              message:
                "claude:aaaa1111 is not a live session holding FH-65 now.",
            },
          },
          { status: 409 },
        ),
      );
      await user.type(box(), "Too late?");
      await user.click(tellBox());
      await user.click(
        screen.getByRole("button", { name: "Comment and tell" }),
      );
      expect(
        (await within(surface.comments()).findByRole("alert")).textContent,
      ).toContain("not a live session");
      expect((box() as HTMLTextAreaElement).value).toBe("Too late?");
      expect(resource.reloads).toBeGreaterThan(0);
    });

    it("shows a sent comment waiting, delivered and locked, or missed", () => {
      surface.render(
        detail({
          activityHash: "a1",
          comments: [
            comment({
              id: "c1",
              sentTo: session.run,
              sentToShort: session.short,
              delivery: "waiting",
            }),
            comment({
              id: "c2",
              body: "Delivered one.",
              sentTo: session.run,
              sentToShort: session.short,
              delivery: "delivered",
              delivered: "2026-10-12T12:11:00Z",
            }),
            comment({
              id: "c3",
              body: "Missed one.",
              sentTo: "claude:bbbb2222",
              sentToShort: "claude:bbbb2222",
              delivery: "missed",
            }),
          ],
        }),
      );
      const item = (text: string) =>
        within(surface.comments())
          .getAllByRole("listitem")
          .find((li) => li.textContent?.includes(text));
      const waiting = item("Use the blue one.");
      const delivered = item("Delivered one.");
      const missed = item("Missed one.");
      expect(waiting?.textContent).toContain(
        "Waiting for claude:aaaa1111 to take it with its next prompt or tool call.",
      );
      expect(delivered?.textContent).toMatch(
        /Delivered to claude:aaaa1111 at .+\. It can no longer be edited\./,
      );
      expect(
        within(delivered as HTMLElement).queryByRole("button", {
          name: "Edit comment",
        }),
      ).toBeNull();
      expect(
        within(waiting as HTMLElement).getByRole("button", {
          name: "Edit comment",
        }),
      ).toBeTruthy();
      expect(missed?.textContent).toContain(
        "Not delivered: claude:bbbb2222 ended before taking it. It stays on the ticket.",
      );
      expect(
        within(missed as HTMLElement).getByRole("button", {
          name: "Edit comment",
        }),
      ).toBeTruthy();
    });
  },
);
