import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { useResource } from "./useResource";

interface Load {
  signal: AbortSignal;
  resolve(value: string): void;
  reject(error: unknown): void;
}

// fakeLoader records each load so a test decides when and how it ends.
function fakeLoader() {
  const loads: Load[] = [];
  const load = (signal: AbortSignal) =>
    new Promise<string>((resolve, reject) => {
      loads.push({ signal, resolve, reject });
    });
  return { load, loads };
}

// finish ends a load and lets the hook take its result.
async function finish(load: Load | undefined, value: string | Error) {
  await act(async () => {
    if (value instanceof Error) load?.reject(value);
    else load?.resolve(value);
  });
}

let visibility: DocumentVisibilityState = "visible";
Object.defineProperty(document, "visibilityState", {
  configurable: true,
  get: () => visibility,
});

function becomes(state: DocumentVisibilityState) {
  visibility = state;
  act(() => {
    document.dispatchEvent(new Event("visibilitychange"));
  });
}

// The load states of ui-layout.md §8 as the hook moves between them:
// loading, ready, error, and ready with the error of a failed reload.
describe("useResource", () => {
  afterEach(() => {
    visibility = "visible";
  });

  it("loads, then is ready with the value", async () => {
    const { load, loads } = fakeLoader();
    const { result } = renderHook(() => useResource(load, "board"));
    expect(result.current.status).toBe("loading");
    expect(loads).toHaveLength(1);
    await finish(loads[0], "cards");
    expect(result.current).toMatchObject({ status: "ready", data: "cards" });
    expect(result.current.error).toBeUndefined();
  });

  it("reports a first load's failure, and Try again loads it", async () => {
    const { load, loads } = fakeLoader();
    const { result } = renderHook(() => useResource(load, "board"));
    await finish(loads[0], new Error("permission denied"));
    expect(result.current).toMatchObject({
      status: "error",
      error: "permission denied",
    });
    act(() => result.current.reload());
    expect(loads).toHaveLength(2);
    await finish(loads[1], "cards");
    expect(result.current).toMatchObject({ status: "ready", data: "cards" });
  });

  it("keeps the good data when a reload fails, and drops the error once one succeeds", async () => {
    const { load, loads } = fakeLoader();
    const { result } = renderHook(() => useResource(load, "board"));
    await finish(loads[0], "cards");

    act(() => result.current.reload());
    // Reloading keeps showing the last good value.
    expect(result.current).toMatchObject({ status: "ready", data: "cards" });
    await finish(loads[1], new Error("disk full"));
    expect(result.current).toMatchObject({
      status: "ready",
      data: "cards",
      error: "disk full",
    });

    act(() => result.current.reload());
    await finish(loads[2], "more cards");
    expect(result.current).toMatchObject({
      status: "ready",
      data: "more cards",
    });
    expect(result.current.error).toBeUndefined();
  });

  it("loads afresh when the key changes, never showing the old key's data", async () => {
    const { load, loads } = fakeLoader();
    const { result, rerender } = renderHook(
      ({ key }) => useResource(load, key),
      { initialProps: { key: "alpha" } },
    );
    await finish(loads[0], "alpha cards");

    rerender({ key: "beta" });
    expect(result.current.status).toBe("loading");
    expect(result.current.data).toBeUndefined();
    expect(loads).toHaveLength(2);
    await finish(loads[1], "beta cards");
    expect(result.current).toMatchObject({
      status: "ready",
      data: "beta cards",
    });
  });

  it("aborts a superseded load and ignores its late result", async () => {
    const { load, loads } = fakeLoader();
    const { result, rerender } = renderHook(
      ({ key }) => useResource(load, key),
      { initialProps: { key: "alpha" } },
    );
    rerender({ key: "beta" });
    expect(loads[0]?.signal.aborted).toBe(true);
    expect(loads[1]?.signal.aborted).toBe(false);

    await finish(loads[1], "beta cards");
    await finish(loads[0], "alpha cards");
    expect(result.current).toMatchObject({
      status: "ready",
      data: "beta cards",
    });
    // A superseded load's failure is not reported either.
    rerender({ key: "gamma" });
    rerender({ key: "delta" });
    await finish(loads[2], new Error("aborted"));
    expect(result.current.status).toBe("loading");
    await finish(loads[3], "delta cards");
    expect(result.current).toMatchObject({
      status: "ready",
      data: "delta cards",
    });
  });

  it("reloads when the revision moves, aborting a load still running", async () => {
    const { load, loads } = fakeLoader();
    const { result, rerender } = renderHook(
      ({ revision }) => useResource(load, "board", revision),
      { initialProps: { revision: 1 } },
    );
    await finish(loads[0], "cards");
    rerender({ revision: 2 });
    expect(loads).toHaveLength(2);
    rerender({ revision: 3 });
    expect(loads[1]?.signal.aborted).toBe(true);
    await finish(loads[2], "newer cards");
    expect(result.current).toMatchObject({
      status: "ready",
      data: "newer cards",
    });
  });

  it("reloads when the page becomes visible again, not when it is hidden", async () => {
    const { load, loads } = fakeLoader();
    renderHook(() => useResource(load, "board"));
    await finish(loads[0], "cards");
    becomes("hidden");
    expect(loads).toHaveLength(1);
    becomes("visible");
    expect(loads).toHaveLength(2);
  });

  it("does not reload when only the loader changes", async () => {
    const first = fakeLoader();
    const second = fakeLoader();
    const { result, rerender } = renderHook(
      ({ load }) => useResource(load, "board"),
      { initialProps: { load: first.load } },
    );
    await finish(first.loads[0], "cards");
    rerender({ load: second.load });
    expect(second.loads).toHaveLength(0);
    // The next reload uses the current loader.
    act(() => result.current.reload());
    expect(first.loads).toHaveLength(1);
    expect(second.loads).toHaveLength(1);
  });

  it("waits while there is no loader", () => {
    const { result } = renderHook(() =>
      useResource<string>(undefined, "board"),
    );
    expect(result.current.status).toBe("loading");
  });
});
