import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  type Mock,
  vi,
} from "vitest";

import type { AuthenticatedFetch } from "../../api/client";
import { leave } from "../../state/leaveGuard";
import type { Editing } from "../editing/useEditing";
import { CardPanel } from "./CardPanel";
import { detail } from "./ticketFixture";

// The card panel's Edit tab holds unsaved work like its Comments tab
// (FH-138): its draft stays while another tab is shown, and closing the
// panel, Escape or opening another ticket asks first. Closing and opening
// go through the leave guard as the board's useTicketPanel does; the
// ticket's useResource is mocked, so the panel renders a loaded ticket
// without a server.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({ ...resource.state, reload: () => undefined }),
}));

const editing = {
  move: vi.fn(),
  archive: vi.fn(),
  notify: vi.fn(),
  answer: vi.fn(),
} as unknown as Editing;

const raw = "---\nid: FH-65\npriority: medium\n---\n# Outline the page\n";

// closed and opened stand for the board closing the panel and opening
// another ticket: each runs only once the guard lets it.
let closed: Mock<() => void>;
let opened: Mock<() => void>;

// renderPanel opens FH-65 on its Edit tab. A write waits for reply, when
// given; reads never settle (useResource is mocked).
function renderPanel(reply?: Promise<Response>) {
  resource.state = { status: "ready", data: detail({ raw }) };
  const fetcher = vi.fn((_input: RequestInfo | URL, init?: RequestInit) =>
    init?.method && init.method !== "GET" && reply
      ? reply
      : new Promise<Response>(() => {}),
  ) as unknown as AuthenticatedFetch;
  return render(
    <CardPanel
      ticket={{ id: "FH-65" }}
      fetcher={fetcher}
      lines={new Map()}
      workstreams={new Map()}
      keys={new Set(["FH"])}
      onOpen={() => undefined}
      onClose={() => leave(closed)}
      revision={0}
      editing={editing}
      initialTab="edit"
    />,
  );
}

const tab = (name: string | RegExp) => screen.getByRole("tab", { name });
const editPanel = () => screen.getByRole("tabpanel", { name: "Edit" });
const title = () =>
  screen.getByRole("textbox", { name: "Title" }) as HTMLInputElement;
const priority = () =>
  screen.getByRole("combobox", { name: "Priority" }) as HTMLSelectElement;
const file = () =>
  screen.getByRole("textbox", { name: "Ticket file" }) as HTMLTextAreaElement;
const ask = () =>
  screen.getByRole("dialog", { name: "Unsaved changes to the ticket" });
const choose = (user: ReturnType<typeof userEvent.setup>, name: string) =>
  user.click(within(ask()).getByRole("button", { name }));

beforeEach(() => {
  closed = vi.fn();
  opened = vi.fn();
});

afterEach(() => {
  resource.state = { status: "loading" };
});

describe("the card panel's Edit tab with unsaved changes (FH-138)", () => {
  it("keeps the draft while another tab is shown", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.selectOptions(priority(), "high");
    await user.click(screen.getByRole("radio", { name: "Raw file" }));
    await user.type(file(), "More.");
    await user.click(tab("Ticket"));
    expect(screen.queryByRole("textbox", { name: "Ticket file" })).toBeNull();
    await user.click(tab("Edit"));
    expect(file().value).toBe(`${raw}More.`);
    await user.click(screen.getByRole("radio", { name: "Fields" }));
    expect(priority().value).toBe("high");
  });

  it("asks before closing the panel, and Keep editing returns to the Edit tab with focus in it", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.clear(title());
    await user.type(title(), "Outline the whole page");
    await user.click(tab("Ticket"));
    await user.click(screen.getByRole("button", { name: "Close ticket" }));
    ask();
    await choose(user, "Keep editing");
    expect(closed).not.toHaveBeenCalled();
    expect(tab("Edit").getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(title());
    expect(title().value).toBe("Outline the whole page");

    // Discard drops the draft and lets the panel close.
    await user.click(screen.getByRole("button", { name: "Close ticket" }));
    await choose(user, "Discard changes");
    expect(closed).toHaveBeenCalledOnce();
    expect(title().value).toBe("Outline the page");
    act(() => leave(closed));
    expect(closed).toHaveBeenCalledTimes(2);
  });

  it("asks on Escape, keeping focus where it was, and Discard drops the draft", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(screen.getByRole("radio", { name: "Raw file" }));
    await user.type(file(), "More.");
    await user.keyboard("{Escape}");
    ask();
    await choose(user, "Keep editing");
    expect(closed).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(file());
    expect(file().value).toBe(`${raw}More.`);

    await user.keyboard("{Escape}");
    await choose(user, "Discard changes");
    expect(closed).toHaveBeenCalledOnce();
    expect(file().value).toBe(raw);
  });

  it("asks before opening another ticket", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.selectOptions(priority(), "high");
    await user.click(tab("Runs"));
    act(() => leave(opened));
    await choose(user, "Keep editing");
    expect(opened).not.toHaveBeenCalled();
    expect(
      within(editPanel()).getByRole("combobox", { name: "Priority" }),
    ).toBeTruthy();
    expect(editPanel().contains(document.activeElement)).toBe(true);
    expect(priority().value).toBe("high");

    act(() => leave(opened));
    await choose(user, "Discard changes");
    expect(opened).toHaveBeenCalledOnce();
    expect(priority().value).toBe("medium");
  });

  it("does not ask while the Edit tab has no changes", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.selectOptions(priority(), "high");
    await user.selectOptions(priority(), "medium");
    act(() => leave(opened));
    expect(opened).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("names both drafts, and Keep editing leaves focus in the comment being written", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.selectOptions(priority(), "high");
    await user.click(tab("Comments"));
    const box = screen.getByRole("textbox", { name: "Add a comment" });
    await user.type(box, "Half a thought");
    await user.keyboard("{Escape}");
    const both = screen.getByRole("dialog", {
      name: "Unsaved changes to the ticket and the comment",
    });
    await user.click(
      within(both).getByRole("button", { name: "Keep editing" }),
    );
    expect(tab(/^Comments/).getAttribute("aria-selected")).toBe("true");
    expect(document.activeElement).toBe(box);
  });

  it("shows the Edit tab again when a save made before a tab switch meets a conflict", async () => {
    const user = userEvent.setup();
    let conflict: (response: Response) => void = () => undefined;
    renderPanel(
      new Promise<Response>((resolve) => {
        conflict = resolve;
      }),
    );
    await user.selectOptions(priority(), "high");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await user.click(tab("Ticket"));
    await act(async () =>
      conflict(
        Response.json(
          {
            error: { code: "conflict", message: "This ticket changed." },
            current: { hash: "newer", content: "Theirs." },
          },
          { status: 409 },
        ),
      ),
    );
    expect(
      await screen.findByRole("dialog", {
        name: "FH-65 changed while you were editing",
      }),
    ).toBeTruthy();
    expect(tab("Edit").getAttribute("aria-selected")).toBe("true");
  });
});
