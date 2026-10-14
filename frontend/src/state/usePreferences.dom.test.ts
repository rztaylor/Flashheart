import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AuthenticatedFetch } from "../api/client";
import {
  defaultPreferences,
  type Preferences,
  type PreferencesPatch,
  type ScopePreferences,
} from "../api/preferences";
import { SAVE_DELAY_MS, usePreferences } from "./usePreferences";

const none = { include: [], exclude: [] };
const hiding = (slug: string): ScopePreferences => ({
  view: "board",
  type: none,
  priority: none,
  workstream: { include: [], exclude: [slug] },
  age: none,
  state: "",
});

// fakeServer stands for /api/preferences: reads answer its copy, held
// until the test lets them through when hold is set; a patch changes it
// as the server's PATCH does.
function fakeServer(initial: Preferences) {
  let copy = structuredClone(initial);
  const patches: PreferencesPatch[] = [];
  const held: (() => void)[] = [];
  let hold = false;
  const fetcher: AuthenticatedFetch = async (_input, init) => {
    if (init?.method === "PATCH") {
      const patch = JSON.parse(String(init.body)) as PreferencesPatch;
      patches.push(patch);
      const { scopes, expandedWorkstreams, ...rest } = patch;
      copy = { ...copy, ...rest };
      for (const [project, scope] of Object.entries(scopes ?? {})) {
        if (scope === null) delete copy.scopes[project];
        else copy.scopes[project] = scope;
      }
      for (const [project, slugs] of Object.entries(
        expandedWorkstreams ?? {},
      )) {
        if (slugs === null) delete copy.expandedWorkstreams[project];
        else copy.expandedWorkstreams[project] = slugs;
      }
      return new Response(null, { status: 204 });
    }
    const answer = JSON.stringify(copy);
    if (hold) await new Promise<void>((resolve) => held.push(resolve));
    return new Response(answer, {
      headers: { "Content-Type": "application/json" },
    });
  };
  return {
    fetcher,
    patches,
    get copy() {
      return copy;
    },
    // set changes the copy as another writer (an agent's rename) does.
    set(next: Preferences) {
      copy = structuredClone(next);
    },
    hold(value: boolean) {
      hold = value;
    },
    release() {
      for (const resolve of held.splice(0)) resolve();
    },
  };
}

const settle = () => act(() => vi.advanceTimersByTimeAsync(0));
const saveDelay = () => act(() => vi.advanceTimersByTimeAsync(SAVE_DELAY_MS));

const before: Preferences = {
  ...defaultPreferences,
  scopes: { alpha: hiding("board-ui") },
  expandedWorkstreams: { alpha: ["board-ui"] },
};
// renamed is the copy after an agent renamed board-ui to board (FH-170).
const renamed: Preferences = {
  ...defaultPreferences,
  scopes: { alpha: hiding("board") },
  expandedWorkstreams: { alpha: ["board"] },
};

// FH-173: an open tab hears that its saved preferences changed on the
// server, takes the server's copy when nothing of its own is waiting, and
// never loses a change it just made.
describe("usePreferences refresh", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  async function loaded(server: ReturnType<typeof fakeServer>) {
    const hook = renderHook(() => usePreferences(server.fetcher, true));
    await settle();
    expect(hook.result.current.loaded).toBe(true);
    return hook;
  }

  it("takes the server's copy on a refresh, and later saves keep it", async () => {
    const server = fakeServer(before);
    const { result } = await loaded(server);
    expect(result.current.refreshed).toBe(0);

    server.set(renamed);
    act(() => result.current.refresh());
    await settle();
    expect(result.current.preferences).toEqual(renamed);
    expect(result.current.refreshed).toBe(1);

    act(() =>
      result.current.update((current) => ({ ...current, theme: "dark" })),
    );
    await saveDelay();
    expect(server.patches).toEqual([{ theme: "dark" }]);
    expect(server.copy).toEqual({ ...renamed, theme: "dark" });

    // A refresh that finds the copy unchanged changes nothing.
    act(() => result.current.refresh());
    await settle();
    expect(result.current.refreshed).toBe(1);
  });

  it("keeps a change made while the refresh was being read", async () => {
    const server = fakeServer(before);
    const { result } = await loaded(server);
    server.set(renamed);
    server.hold(true);
    act(() => result.current.refresh());
    await settle();
    act(() =>
      result.current.update((current) => ({ ...current, density: "compact" })),
    );
    server.hold(false);
    server.release();
    await settle();
    // The read predates the change: the page keeps its own copy.
    expect(result.current.preferences.density).toBe("compact");

    // Once the change is saved the server's copy is read again, with both.
    await saveDelay();
    await settle();
    expect(server.patches).toEqual([{ density: "compact" }]);
    expect(result.current.preferences).toEqual({
      ...renamed,
      density: "compact",
    });
  });

  it("waits for a pending save before it reads the server's copy", async () => {
    const server = fakeServer(before);
    const { result } = await loaded(server);
    act(() =>
      result.current.update((current) => ({ ...current, theme: "light" })),
    );
    server.set({ ...renamed });
    act(() => result.current.refresh());
    await settle();
    expect(result.current.preferences.scopes).toEqual(before.scopes);

    await saveDelay();
    await settle();
    expect(server.patches).toEqual([{ theme: "light" }]);
    expect(result.current.preferences).toEqual({ ...renamed, theme: "light" });
  });

  it("does not refresh before the preferences have loaded", async () => {
    const server = fakeServer(before);
    const { result } = renderHook(() => usePreferences(server.fetcher, false));
    act(() => result.current.refresh());
    await settle();
    expect(result.current.loaded).toBe(false);
    expect(result.current.preferences).toEqual(defaultPreferences);
  });
});
