import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { guardLeaving } from "../state/leaveGuard";
import type { Route } from "./route";
import { useTicketPanel } from "./useTicketPanel";

// Closing the card panel returns focus to what opened it (FH-128: also
// when closing waited for the person to drop a comment being written).
const route: Route = {
  scope: { kind: "all" },
  view: "board",
  ticket: { id: "FH-1" },
};

let release: (() => void) | undefined;

afterEach(() => {
  release?.();
  release = undefined;
  vi.useRealTimers();
  document.body.innerHTML = "";
});

function setUp() {
  const card = document.createElement("button");
  card.dataset.ticket = "FH-1";
  const other = document.createElement("button");
  document.body.append(card, other);
  const navigate = vi.fn();
  const { result } = renderHook(() => useTicketPanel(route, navigate));
  return { card, other, navigate, panel: result };
}

describe("closing the card panel", () => {
  it("returns focus to the ticket's card", () => {
    vi.useFakeTimers();
    const { card, navigate, panel } = setUp();
    act(() => panel.current.closeTicket());
    expect(navigate).toHaveBeenCalledWith({ ...route, ticket: undefined });
    act(() => vi.runAllTimers());
    expect(document.activeElement).toBe(card);
  });

  it("waits while unsaved work holds the page, and moves focus only once dropped", () => {
    vi.useFakeTimers();
    const { card, other, navigate, panel } = setUp();
    let proceed: (() => void) | undefined;
    release = guardLeaving((go) => {
      proceed = go;
    });
    other.focus();
    act(() => panel.current.closeTicket());
    act(() => vi.runAllTimers());
    expect(navigate).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(other);
    release();
    act(() => proceed?.());
    expect(navigate).toHaveBeenCalledOnce();
    act(() => vi.runAllTimers());
    expect(document.activeElement).toBe(card);
  });
});

describe("opening another ticket while unsaved work holds the page", () => {
  it("remembers the new opener only once the person drops the work", () => {
    vi.useFakeTimers();
    const { card, other, navigate, panel } = setUp();
    let proceed: (() => void) | undefined;
    release = guardLeaving((go) => {
      proceed = go;
    });
    // Kept: nothing opens, and closing later returns to the ticket's card.
    other.focus();
    act(() => panel.current.openTicket({ id: "FH-2" }));
    expect(navigate).not.toHaveBeenCalled();
    release();
    release = undefined;
    card.focus();
    other.focus();
    act(() => panel.current.closeTicket());
    act(() => vi.runAllTimers());
    expect(document.activeElement).toBe(card);
    expect(proceed).toBeDefined();
  });

  it("opens the ticket and remembers what opened it once dropped", () => {
    vi.useFakeTimers();
    const { other, navigate, panel } = setUp();
    let proceed: (() => void) | undefined;
    release = guardLeaving((go) => {
      proceed = go;
    });
    other.focus();
    act(() => panel.current.openTicket({ id: "FH-2" }));
    // The prompt takes focus while it asks.
    (document.activeElement as HTMLElement | null)?.blur();
    release();
    release = undefined;
    act(() => proceed?.());
    expect(navigate).toHaveBeenCalledWith({ ...route, ticket: { id: "FH-2" } });
    act(() => panel.current.closeTicket());
    act(() => vi.runAllTimers());
    expect(document.activeElement).toBe(other);
  });
});
