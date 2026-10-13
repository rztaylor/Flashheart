import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { guardLeaving } from "../state/leaveGuard";
import { useRoute } from "./route";

let release: (() => void) | undefined;
let asks: (() => void)[] = [];

beforeEach(() => {
  window.history.replaceState(null, "", "#/ticket/FH-66");
  asks = [];
});
afterEach(() => release?.());

const hashChange = (hash: string) =>
  act(() => {
    window.history.replaceState(null, "", hash);
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });

// Leaving a page with an unsaved edit asks first (FH-66): the router holds
// both its own navigation and a link or Back that already changed the hash.
describe("useRoute with the leave guard", () => {
  it("holds a navigation until the guard's owner lets it go", () => {
    const { result } = renderHook(() => useRoute());
    release = guardLeaving((proceed) => asks.push(proceed));
    act(() => result.current[1]({ scope: { kind: "all" }, view: "board" }));
    expect(result.current[0].view).toBe("ticket");
    expect(window.location.hash).toBe("#/ticket/FH-66");
    act(() => {
      release?.();
      asks[0]?.();
    });
    expect(result.current[0].view).toBe("board");
    expect(window.location.hash).toBe("#/all/board");
  });

  it("puts a link's hash back while it asks, and follows it once allowed", () => {
    const { result } = renderHook(() => useRoute());
    release = guardLeaving((proceed) => asks.push(proceed));
    hashChange("#/ticket/FH-65");
    expect(window.location.hash).toBe("#/ticket/FH-66");
    expect(result.current[0].ticket?.id).toBe("FH-66");
    expect(asks).toHaveLength(1);
    act(() => {
      release?.();
      asks[0]?.();
    });
    // Setting the hash fires hashchange in the browser; the test fires it.
    hashChange(window.location.hash);
    expect(result.current[0].ticket?.id).toBe("FH-65");
  });

  it("follows links at once with nothing unsaved", () => {
    const { result } = renderHook(() => useRoute());
    hashChange("#/p/flashheart/board");
    expect(result.current[0]).toMatchObject({
      scope: { kind: "project", project: "flashheart" },
      view: "board",
    });
  });
});
