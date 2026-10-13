import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Editing } from "../editing/useEditing";
import { CardPanel } from "./CardPanel";
import { TicketPage } from "./TicketPage";
import { detail } from "./ticketFixture";

// The ticket's useResource is mocked, so the page renders a loaded ticket
// without a server.
const resource = vi.hoisted(() => ({
  state: { status: "loading" } as Record<string, unknown>,
}));
vi.mock("../../state/useResource", () => ({
  useResource: () => ({ ...resource.state, reload: () => undefined }),
}));

// observers are the IntersectionObservers the page made, so a test can say
// which sections are in view as if the page had scrolled.
let observers: FakeObserver[] = [];
class FakeObserver {
  elements: Element[] = [];
  constructor(readonly callback: IntersectionObserverCallback) {
    observers.push(this);
  }
  observe(element: Element) {
    this.elements.push(element);
  }
  unobserve(element: Element) {
    this.elements = this.elements.filter((item) => item !== element);
  }
  disconnect() {
    this.elements = [];
  }
  takeRecords() {
    return [];
  }
}

// scrollTo reports the sections in view (by their headings' text) and the
// rest out of view.
function scrollTo(...titles: string[]) {
  act(() => {
    for (const observer of observers) {
      const entries = observer.elements.map((target) => ({
        target,
        isIntersecting: titles.some(
          (title) => target.querySelector("h2, h3")?.textContent === title,
        ),
      })) as unknown as IntersectionObserverEntry[];
      observer.callback(entries, observer as unknown as IntersectionObserver);
    }
  });
}

const editing = {
  move: vi.fn(),
  archive: vi.fn(),
  notify: vi.fn(),
  answer: vi.fn(),
} as unknown as Editing;

const props = {
  ticket: { id: "FH-65" },
  fetcher: () => new Promise<Response>(() => {}),
  lines: new Map(),
  workstreams: new Map(),
  keys: new Set(["FH"]),
  onOpen: () => undefined,
  onClose: () => undefined,
  revision: 0,
};

function renderPage(data = detail(), withEditing = true) {
  resource.state = { status: "ready", data };
  return render(
    <TicketPage
      {...props}
      editing={withEditing ? editing : undefined}
      boardLink={() => ({ href: "#/p/flashheart/board", label: "Flashheart" })}
    />,
  );
}

const outline = () =>
  screen.getAllByRole("navigation", { name: "Sections" })[0] as HTMLElement;

beforeEach(() => {
  observers = [];
  vi.stubGlobal("IntersectionObserver", FakeObserver);
});

afterEach(() => {
  vi.unstubAllGlobals();
  resource.state = { status: "loading" };
});

describe("the ticket page (CARD-7, FH-65)", () => {
  it("shows the whole ticket as one page, with no tabs", () => {
    renderPage();
    expect(screen.queryAllByRole("tab")).toHaveLength(0);
    expect(screen.queryByRole("tabpanel")).toBeNull();
    expect(
      screen.getByRole("heading", { level: 1, name: "Outline the page" }),
    ).toBeTruthy();
  });

  it("orders the ticket's sections, then the review's, then Files and Activity", () => {
    renderPage();
    const headings = screen
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent);
    expect(headings).toEqual([
      "Properties",
      "Handoff",
      "Acceptance criteria1 of 2",
      "Description",
      "Test plan",
      "Review",
      "Files",
      "Activity",
    ]);
    const review = screen
      .getAllByRole("heading", { level: 3 })
      .map((heading) => heading.textContent);
    expect(review).toEqual([
      "Summary",
      "Evidence2 screenshots",
      "How to verify1 of 2",
      "Tests",
    ]);
  });

  it("leaves Review out without review.md", () => {
    renderPage(detail({ review: null }));
    const headings = screen
      .getAllByRole("heading", { level: 2 })
      .map((heading) => heading.textContent);
    expect(headings).not.toContain("Review");
    expect(headings).toContain("Files");
    expect(within(outline()).queryByText("Review")).toBeNull();
  });

  it("lists every section in the outline with its count, empty ones as empty", () => {
    renderPage();
    const nav = within(outline());
    expect(
      nav.getByRole("button", { name: "Acceptance criteria 1 of 2" }),
    ).toBeTruthy();
    expect(nav.getByRole("button", { name: "Evidence 2" })).toBeTruthy();
    expect(
      nav.getByRole("button", { name: "How to verify 1 of 2" }),
    ).toBeTruthy();
    expect(nav.getByRole("button", { name: "Files 1" })).toBeTruthy();
    expect(nav.getByRole("button", { name: "Activity 3" })).toBeTruthy();
    // Context is empty: listed, but not a place to go.
    expect(nav.queryByRole("button", { name: /Context/ })).toBeNull();
    expect(nav.getByText("Context, empty")).toBeTruthy();
  });

  it("highlights the section in view as the page scrolls", () => {
    renderPage();
    const nav = within(outline());
    const current = () =>
      nav
        .getAllByRole("button")
        .filter((button) => button.getAttribute("aria-current") === "location")
        .map((button) => button.textContent);
    expect(current()).toEqual(["Handoff"]);
    scrollTo("Description", "Test plan");
    expect(current()).toEqual(["Description"]);
    scrollTo("Tests");
    expect(current()).toEqual(["Tests"]);
    // Between sections the last one passed stays current.
    scrollTo();
    expect(current()).toEqual(["Tests"]);
  });

  it("moves focus to a section chosen by keyboard or mouse", async () => {
    const user = userEvent.setup();
    renderPage();
    const nav = within(outline());
    nav.getByRole("button", { name: "Test plan" }).focus();
    await user.keyboard("{Enter}");
    expect(document.activeElement?.textContent).toBe("Test plan");
    expect(document.activeElement?.tagName).toBe("H2");
    await user.click(nav.getByRole("button", { name: "Activity 3" }));
    expect(document.activeElement?.textContent).toBe("Activity");
    expect(
      nav
        .getByRole("button", { name: "Activity 3" })
        .getAttribute("aria-current"),
    ).toBe("location");
  });

  it("keeps a chosen section current until the page is moved by hand", async () => {
    const user = userEvent.setup();
    renderPage();
    const activity = within(outline()).getByRole("button", {
      name: "Activity 3",
    });
    await user.click(activity);
    // Activity is short and last: the scroll leaves Files in the top third.
    scrollTo("Files");
    expect(activity.getAttribute("aria-current")).toBe("location");
    fireEvent.wheel(window);
    scrollTo("Files");
    expect(activity.getAttribute("aria-current")).toBeNull();
    expect(
      within(outline())
        .getByRole("button", { name: "Files 1" })
        .getAttribute("aria-current"),
    ).toBe("location");
  });

  it("shows each screenshot once: Evidence as a gallery, the rest under Files", async () => {
    const user = userEvent.setup();
    renderPage();
    const evidence = screen.getByRole("region", { name: /^Evidence/ });
    const shots = within(evidence).getAllByRole("button", { name: /^Open / });
    expect(shots.map((shot) => shot.getAttribute("aria-label"))).toEqual([
      "Open Light, 1440",
      "Open Dark, 1440",
    ]);
    const filesSection = screen.getByRole("region", { name: "Files" });
    expect(
      within(filesSection)
        .getAllByRole("button", { name: /^Open / })
        .map((shot) => shot.getAttribute("aria-label")),
    ).toEqual(["Open Caption of extra.png"]);
    expect(
      document.querySelectorAll('img[src="/api/files/light.png"]'),
    ).toHaveLength(1);

    await user.click(shots[1] as HTMLElement);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("heading").textContent).toBe("Dark, 1440");
  });

  it("shows Activity as a timeline, oldest first, ending with the comment box", () => {
    renderPage();
    const activity = screen.getByRole("region", { name: "Activity" });
    const items = within(activity)
      .getAllByRole("listitem")
      .map((item) => item.textContent ?? "");
    expect(items).toHaveLength(3);
    expect(items[0]).toContain("Mocked the layout.");
    expect(items[1]).toContain("Drop the line strip?");
    expect(items[1]).toContain("Yes, drop it");
    expect(items[2]).toContain("updated the handoff");
    expect(items[2]).toContain("Verify it");
    // Comments are written below the timeline (FH-125); notes stay the
    // agents'.
    expect(
      within(activity).getByRole("textbox", { name: "Add a comment" }),
    ).toBeTruthy();
  });

  it("keeps the properties in one panel with Move to and Archive", () => {
    renderPage();
    const properties = screen.getByRole("region", { name: "Properties" });
    const panel = within(properties);
    expect(panel.getByRole("combobox", { name: "Move to" })).toBeTruthy();
    expect(panel.getByRole("button", { name: "Archive" })).toBeTruthy();
    for (const term of [
      "Priority",
      "Type",
      "Workstream",
      "Branch",
      "Depends on",
      "Tags",
      "Agent",
      "Created",
      "Changed",
    ]) {
      expect(panel.getByText(term, { selector: "dt" })).toBeTruthy();
    }
    expect(panel.getByRole("link", { name: "FH-99" })).toBeTruthy();
  });

  it("marks Priority and Type with the card's bars and icon beside the words (FH-154)", () => {
    renderPage();
    const properties = screen.getByRole("region", { name: "Properties" });
    const value = (term: string) =>
      within(properties).getByText(term, { selector: "dt" })
        .nextElementSibling as HTMLElement;
    const priority = value("Priority");
    expect(priority.querySelector('[data-priority="medium"]')).toBeTruthy();
    expect(priority.textContent).toContain("Medium");
    const type = value("Type");
    expect(type.querySelector('[data-type="feature"]')).toBeTruthy();
    expect(type.textContent).toContain("Feature");
  });

  it("shows the status without Move to or Archive when read-only", () => {
    renderPage(detail(), false);
    const panel = within(screen.getByRole("region", { name: "Properties" }));
    expect(panel.queryByRole("combobox")).toBeNull();
    expect(panel.queryByRole("button", { name: "Archive" })).toBeNull();
    expect(panel.getByText("Ready to review")).toBeTruthy();
  });

  it("leaves the board's card panel with its tabs", () => {
    resource.state = { status: "ready", data: detail() };
    render(<CardPanel {...props} editing={editing} />);
    const tabs = screen.getAllByRole("tab").map((tab) => tab.textContent);
    expect(tabs).toEqual([
      "Ticket",
      "Comments",
      "Edit",
      "Runs",
      "Attachments 3",
      "Review",
    ]);
    expect(screen.queryByRole("navigation", { name: "Sections" })).toBeNull();
  });
});
