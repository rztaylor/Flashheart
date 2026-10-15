import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { narrowQuery, useStartOnWiden } from "./useStartOnWiden";

// fakeMedia stands for window.matchMedia: the test fires the width change
// itself, since happy-dom has no viewport to resize.
function fakeMedia() {
  const listeners = new Set<(event: MediaQueryListEvent) => void>();
  const list = {
    matches: true,
    media: narrowQuery,
    addEventListener: vi.fn(
      (_: "change", listener: (event: MediaQueryListEvent) => void) => {
        listeners.add(listener);
      },
    ),
    removeEventListener: vi.fn(
      (_: "change", listener: (event: MediaQueryListEvent) => void) => {
        listeners.delete(listener);
      },
    ),
  };
  const matchMedia = vi.fn((query: string) => {
    expect(query).toBe(narrowQuery);
    return list as unknown as MediaQueryList;
  });
  vi.stubGlobal("matchMedia", matchMedia);
  return {
    list,
    resize: (matches: boolean) => {
      for (const listener of listeners)
        listener({ matches } as MediaQueryListEvent);
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

// FH-155: the strip starts again when the window leaves the phone layout.
describe("useStartOnWiden", () => {
  it("scrolls the strip to its start when the window widens past the phone layout", () => {
    const media = fakeMedia();
    const scroller = { current: { scrollLeft: 381 } as HTMLElement };
    renderHook(() => useStartOnWiden(scroller));
    media.resize(false);
    expect(scroller.current.scrollLeft).toBe(0);
  });

  it("leaves the strip where the phone's Column picker put it while narrow", () => {
    const media = fakeMedia();
    const scroller = { current: { scrollLeft: 104 } as HTMLElement };
    renderHook(() => useStartOnWiden(scroller));
    media.resize(true);
    expect(scroller.current.scrollLeft).toBe(104);
  });

  it("stops listening when the board unmounts", () => {
    const media = fakeMedia();
    const scroller = { current: { scrollLeft: 381 } as HTMLElement };
    const { unmount } = renderHook(() => useStartOnWiden(scroller));
    unmount();
    expect(media.list.removeEventListener).toHaveBeenCalledTimes(1);
    media.resize(false);
    expect(scroller.current.scrollLeft).toBe(381);
  });
});
