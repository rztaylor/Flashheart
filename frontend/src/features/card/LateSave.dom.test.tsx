import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AuthenticatedFetch } from "../../api/client";
import type { ToastMessage } from "../../components/Toast";
import { leave } from "../../state/leaveGuard";
import { type EditPlace, LateSavesProvider } from "../editing/lateSaves";
import type { Editing } from "../editing/useEditing";
import { CardPanel } from "./CardPanel";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// A save still waiting for its answer is not unsaved work (FH-156, D70):
// leaving does not ask about it, and when it fails or conflicts after the
// person has moved on, a toast says so with Back to the edit, which goes
// back and reopens the edit with their text. The harness stands for the
// shell: it shows the ticket's full page, its card panel or neither, and
// goes back to a place as the shell's route would. The ticket's
// useResource is mocked, so each view renders a loaded ticket.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({ ...resource.state, reload: () => undefined }),
}));

// The formatted editor is stood in for by a text area, as on the page's
// other interaction tests.
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

class FakeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

interface Write {
  path: string;
  body: Record<string, unknown>;
  answer(response: Response): void;
}

// fakeServer holds every write until the test answers it; reads never
// settle (useResource is mocked).
function fakeServer() {
  const writes: Write[] = [];
  const fetcher = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    if (method === "GET") return new Promise<Response>(() => {});
    return new Promise<Response>((resolve) => {
      writes.push({
        path: String(input),
        body: JSON.parse(String(init?.body ?? "{}")),
        answer: resolve,
      });
    });
  }) as unknown as AuthenticatedFetch;
  return { fetcher, writes };
}

const busy = () =>
  Response.json(
    { error: { code: "busy", message: "The project is busy" } },
    { status: 503 },
  );

const conflict = () =>
  Response.json(
    {
      error: { code: "conflict", message: "This ticket changed." },
      current: {
        hash: "newer",
        content: "## Description\n\nTheirs.\n",
      },
    },
    { status: 409 },
  );

const editing = {
  move: vi.fn(),
  archive: vi.fn(),
  notify: vi.fn(),
  answer: vi.fn(),
} as unknown as Editing;

type Shown =
  | { view: "page" }
  | { view: "panel"; tab: "comments" | "edit" }
  | { view: "board" };

let server: ReturnType<typeof fakeServer>;
let toasts: Omit<ToastMessage, "id">[];
let show: (shown: Shown) => void;

function Shell({ initial }: { initial: Shown }) {
  const [shown, setShown] = useState<Shown>(initial);
  show = setShown;
  const ticket = detail();
  const back = (place: EditPlace, ready: () => void) =>
    leave(() => {
      ready();
      setShown(
        place.kind === "panel"
          ? { view: "panel", tab: place.tab }
          : { view: "page" },
      );
    });
  return (
    <LateSavesProvider notify={(toast) => toasts.push(toast)} back={back}>
      {shown.view === "page" ? (
        <TicketPage
          ticket={{ id: ticket.id }}
          fetcher={server.fetcher}
          lines={new Map()}
          workstreams={new Map()}
          keys={new Set(["FH"])}
          onOpen={() => undefined}
          onClose={() => undefined}
          revision={0}
          editing={editing}
          boardLink={() => ({
            href: "#/p/flashheart/board",
            label: "Flashheart",
          })}
        />
      ) : shown.view === "panel" ? (
        <CardPanel
          key={shown.tab}
          ticket={{ id: ticket.id }}
          fetcher={server.fetcher}
          lines={new Map()}
          workstreams={new Map()}
          keys={new Set(["FH"])}
          onOpen={() => undefined}
          onClose={() => undefined}
          revision={0}
          editing={editing}
          initialTab={shown.tab}
        />
      ) : (
        <p>The board</p>
      )}
    </LateSavesProvider>
  );
}

function renderShell(initial: Shown = { view: "page" }) {
  resource.state = { status: "ready", data: detail() };
  return render(<Shell initial={initial} />);
}

// goTo leaves for another view as the shell's route does: through the
// leave guard, which asks first while anything is unsaved.
const goTo = (shown: Shown) => act(() => leave(() => show(shown)));
const asked = () => screen.queryByRole("dialog", { name: /^Unsaved changes/ });
const pencil = (name: string) =>
  screen.getByRole("button", { name: `Edit ${name}` });
const description = () =>
  screen.getByRole("textbox", { name: "Description" }) as HTMLTextAreaElement;
const answer = async (response: Response) => {
  const write = server.writes.shift();
  if (!write) throw new Error("no write waiting");
  await act(async () => write.answer(response));
};
// back presses the last toast's Back to the edit.
const back = () => {
  const action = toasts.at(-1)?.action;
  expect(action?.label).toBe("Back to the edit");
  act(() => action?.run());
};

beforeEach(() => {
  server = fakeServer();
  toasts = [];
  vi.stubGlobal("IntersectionObserver", FakeObserver);
});

afterEach(async () => {
  // A save left in flight would hold the browser's prompt into the next
  // test.
  for (const write of server.writes.splice(0)) write.answer(busy());
  await act(() => new Promise<void>((resolve) => setTimeout(resolve, 0)));
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe("an in-place save waiting for its answer (FH-156)", () => {
  it("does not ask on leaving the page or opening another item", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(server.writes).toHaveLength(1);

    await user.click(pencil("priority"));
    expect(asked()).toBeNull();
    expect(screen.getByRole("combobox", { name: "Priority" })).toBeTruthy();

    goTo({ view: "board" });
    expect(asked()).toBeNull();
    expect(screen.getByText("The board")).toBeTruthy();
  });

  it("says in a toast that it was not saved when it fails after the page was left", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    goTo({ view: "board" });
    await answer(busy());
    expect(toasts.map((toast) => toast.text)).toEqual([
      "Your edit to the description of FH-65 was not saved: The project is busy",
    ]);
  });

  it("goes back to the page with the text and the reason on Back to the edit", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    goTo({ view: "board" });
    await answer(busy());
    back();
    expect(description().value).toBe(
      "Lay the ticket out in three columns. Then a fourth.",
    );
    expect(screen.getByRole("alert").textContent).toBe("The project is busy");
    // The edit is unsaved again: leaving asks.
    goTo({ view: "board" });
    expect(asked()).toBeTruthy();
  });

  it("reopens the item on the same page, after another was opened", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await user.click(pencil("branch"));
    await answer(busy());
    expect(toasts).toHaveLength(1);
    // The branch is untouched, so going back asks nothing.
    back();
    expect(description().value).toBe(
      "Lay the ticket out in three columns. Then a fourth.",
    );
    expect(screen.queryByRole("textbox", { name: "Branch" })).toBeNull();
  });

  it("opens the conflict dialog on Back to the edit after a conflict", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    goTo({ view: "board" });
    await answer(conflict());
    expect(toasts.map((toast) => toast.text)).toEqual([
      "Your edit to the description of FH-65 was not saved: FH-65 changed on disk",
    ]);
    back();
    const dialog = screen.getByRole("dialog", { name: /changed/ });
    expect(within(dialog).getByText(/Theirs\./)).toBeTruthy();
    expect(within(dialog).getByText(/Then a fourth\./)).toBeTruthy();
  });

  it("gives the held edit back when the person opens the item again", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    goTo({ view: "board" });
    await answer(busy());
    act(() => show({ view: "page" }));
    await user.click(pencil("description"));
    expect(description().value).toBe(
      "Lay the ticket out in three columns. Then a fourth.",
    );
  });

  it("still shows a failure beside the item while it is open", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await answer(busy());
    expect(toasts).toEqual([]);
    expect(screen.getByRole("alert").textContent).toBe("The project is busy");
    goTo({ view: "board" });
    expect(asked()).toBeTruthy();
  });

  it("keeps the browser's prompt on reload while the save is in flight", async () => {
    const user = userEvent.setup();
    renderShell();
    await user.click(pencil("description"));
    await user.type(description(), " Then a fourth.");
    await user.click(screen.getByRole("button", { name: "Save" }));
    goTo({ view: "board" });
    const unload = () => {
      const event = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(event);
      return event.defaultPrevented;
    };
    expect(unload()).toBe(true);
    await answer(Response.json({ hash: "saved", revision: 2, warnings: [] }));
    expect(unload()).toBe(false);
    expect(toasts).toEqual([]);
  });
});

describe("the card panel's Edit tab saving (FH-156)", () => {
  it("closes without asking, and Back to the edit reopens the tab with the change", async () => {
    const user = userEvent.setup();
    renderShell({ view: "panel", tab: "edit" });
    const title = () =>
      screen.getByRole("textbox", { name: "Title" }) as HTMLInputElement;
    await user.clear(title());
    await user.type(title(), "Outline the whole page");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    goTo({ view: "board" });
    expect(asked()).toBeNull();
    await answer(busy());
    expect(toasts.map((toast) => toast.text)).toEqual([
      "Your changes to FH-65 were not saved: The project is busy",
    ]);
    back();
    expect(
      screen.getByRole("tab", { name: "Edit" }).getAttribute("aria-selected"),
    ).toBe("true");
    expect(title().value).toBe("Outline the whole page");
    expect(screen.getByRole("alert").textContent).toBe("The project is busy");
  });
});

describe("the comment box posting (FH-156)", () => {
  it("leaves without asking, and Back to the edit puts the comment back", async () => {
    const user = userEvent.setup();
    renderShell();
    const box = () =>
      screen.getByRole("textbox", {
        name: "Add a comment",
      }) as HTMLTextAreaElement;
    await user.type(box(), "Keep the outline.");
    await user.click(screen.getByRole("button", { name: "Comment" }));
    goTo({ view: "board" });
    expect(asked()).toBeNull();
    await answer(busy());
    expect(toasts.map((toast) => toast.text)).toEqual([
      "Your comment on FH-65 was not posted: The project is busy",
    ]);
    back();
    await waitFor(() => expect(box().value).toBe("Keep the outline."));
    expect(screen.getByRole("alert").textContent).toBe("The project is busy");
  });
});
