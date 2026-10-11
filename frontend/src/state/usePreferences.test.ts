import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AuthenticatedFetch } from "../api/client";
import { defaultPreferences, type Preferences } from "../api/preferences";
import {
  type PageVisibility,
  preferenceSaver,
  SAVE_DELAY_MS,
} from "./usePreferences";

// The page and document are stand-ins: the hook passes window and document.
function harness() {
  const fetcher = vi.fn<AuthenticatedFetch>(
    async () => new Response(null, { status: 204 }),
  );
  const page = new EventTarget();
  const visibility = Object.assign(new EventTarget(), {
    visibilityState: "visible" as DocumentVisibilityState,
  }) satisfies PageVisibility;
  const saver = preferenceSaver(fetcher);
  const stop = saver.watch(page, visibility);
  const hide = () => {
    visibility.visibilityState = "hidden";
    visibility.dispatchEvent(new Event("visibilitychange"));
  };
  return { fetcher, page, visibility, saver, stop, hide };
}

const withColour = (colourBy: Preferences["colourBy"]): Preferences => ({
  ...defaultPreferences,
  colourBy,
});

function sent(fetcher: ReturnType<typeof harness>["fetcher"]) {
  return fetcher.mock.calls.map(([path, init]) => ({
    path,
    method: init?.method,
    keepalive: init?.keepalive ?? false,
    body: JSON.parse(String(init?.body)) as Preferences,
  }));
}

describe("preferenceSaver", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("sends a pending save at once on pagehide, kept alive (FH-48)", () => {
    const { fetcher, page, saver, stop } = harness();
    saver.schedule(withColour("priority"));
    vi.advanceTimersByTime(SAVE_DELAY_MS - 1);
    expect(fetcher).not.toHaveBeenCalled();

    page.dispatchEvent(new Event("pagehide"));
    expect(sent(fetcher)).toEqual([
      {
        path: "/api/preferences",
        method: "PUT",
        keepalive: true,
        body: withColour("priority"),
      },
    ]);

    // The timer is spent: the save is not sent twice.
    vi.advanceTimersByTime(SAVE_DELAY_MS);
    expect(fetcher).toHaveBeenCalledTimes(1);
    stop();
  });

  it("sends a pending save when the page becomes hidden", () => {
    const { fetcher, page, saver, stop, hide } = harness();
    saver.schedule(withColour("age"));
    hide();
    expect(sent(fetcher)).toEqual([
      expect.objectContaining({ keepalive: true, body: withColour("age") }),
    ]);

    // Hidden and then unloaded: nothing is left to send.
    page.dispatchEvent(new Event("pagehide"));
    vi.advanceTimersByTime(SAVE_DELAY_MS);
    expect(fetcher).toHaveBeenCalledTimes(1);
    stop();
  });

  it("does not flush when the page becomes visible", () => {
    const { fetcher, visibility, saver, stop } = harness();
    saver.schedule(withColour("age"));
    visibility.dispatchEvent(new Event("visibilitychange"));
    expect(fetcher).not.toHaveBeenCalled();
    vi.advanceTimersByTime(SAVE_DELAY_MS);
    expect(sent(fetcher)).toEqual([
      expect.objectContaining({ keepalive: false, body: withColour("age") }),
    ]);
    stop();
  });

  it("sends nothing on pagehide or hidden when nothing is pending", () => {
    const { fetcher, page, saver, stop, hide } = harness();
    page.dispatchEvent(new Event("pagehide"));
    hide();
    vi.advanceTimersByTime(SAVE_DELAY_MS * 2);
    expect(fetcher).not.toHaveBeenCalled();

    // A save already sent leaves nothing pending either.
    saver.schedule(withColour("none"));
    vi.advanceTimersByTime(SAVE_DELAY_MS);
    expect(fetcher).toHaveBeenCalledTimes(1);
    page.dispatchEvent(new Event("pagehide"));
    expect(fetcher).toHaveBeenCalledTimes(1);
    stop();
  });

  it("coalesces a burst of changes into one save of the last value", () => {
    const { fetcher, saver, stop } = harness();
    saver.schedule(withColour("priority"));
    vi.advanceTimersByTime(SAVE_DELAY_MS - 100);
    saver.schedule(withColour("age"));
    vi.advanceTimersByTime(SAVE_DELAY_MS - 100);
    saver.schedule(withColour("none"));
    vi.advanceTimersByTime(SAVE_DELAY_MS - 1);
    expect(fetcher).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(sent(fetcher)).toEqual([
      {
        path: "/api/preferences",
        method: "PUT",
        keepalive: false,
        body: withColour("none"),
      },
    ]);
    stop();
  });

  it("retries a refused kept-alive save as an ordinary one", async () => {
    const { fetcher, saver, stop, hide } = harness();
    // Browsers refuse keepalive bodies over 64 KiB; the page is still alive
    // when it is only hidden.
    fetcher.mockImplementation(async (_path, init) => {
      if (init?.keepalive) throw new TypeError("keepalive body too large");
      return new Response(null, { status: 204 });
    });
    saver.schedule(withColour("age"));
    hide();
    await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
    expect(sent(fetcher)).toEqual([
      expect.objectContaining({ keepalive: true, body: withColour("age") }),
      expect.objectContaining({ keepalive: false, body: withColour("age") }),
    ]);
    stop();
  });

  it("stops listening and drops a pending save when stopped", () => {
    const { fetcher, page, saver, stop, hide } = harness();
    saver.schedule(withColour("priority"));
    stop();
    page.dispatchEvent(new Event("pagehide"));
    hide();
    vi.advanceTimersByTime(SAVE_DELAY_MS);
    expect(fetcher).not.toHaveBeenCalled();
  });
});
