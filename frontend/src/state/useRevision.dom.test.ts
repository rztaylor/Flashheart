import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AuthenticatedFetch } from "../api/client";
import { RETRY_MS, useRevision } from "./useRevision";

interface Poll {
  since: number;
  signal: AbortSignal | undefined;
  answer(revision: number): void;
  fail(): void;
}

// fakeChanges stands for /api/changes: each long poll waits until the test
// answers or fails it.
function fakeChanges() {
  const polls: Poll[] = [];
  const fetcher: AuthenticatedFetch = (input, init) =>
    new Promise<Response>((resolve, reject) => {
      const url = new URL(String(input), "http://board.test");
      polls.push({
        since: Number(url.searchParams.get("since")),
        signal: init?.signal ?? undefined,
        answer: (revision) =>
          resolve(
            new Response(JSON.stringify({ revision }), {
              headers: { "Content-Type": "application/json" },
            }),
          ),
        fail: () => reject(new TypeError("Failed to fetch")),
      });
    });
  return { fetcher, polls };
}

// settle lets the hook's loop run through the answered poll.
const settle = () => act(() => vi.advanceTimersByTimeAsync(0));

// LIFE-3: the board revision by long poll, retried after an error, and
// followed wherever the server says it is.
describe("useRevision", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("polls from 0 and returns each revision the server reports", async () => {
    const { fetcher, polls } = fakeChanges();
    const { result } = renderHook(() => useRevision(fetcher, true));
    await settle();
    expect(result.current).toBe(0);
    expect(polls.map((poll) => poll.since)).toEqual([0]);

    polls[0]?.answer(7);
    await settle();
    expect(result.current).toBe(7);
    expect(polls.map((poll) => poll.since)).toEqual([0, 7]);

    // A wait that ends with no change polls again from the same revision.
    polls[1]?.answer(7);
    await settle();
    expect(result.current).toBe(7);
    expect(polls.map((poll) => poll.since)).toEqual([0, 7, 7]);
  });

  it("retries after an error, once the retry delay has passed", async () => {
    const { fetcher, polls } = fakeChanges();
    const { result } = renderHook(() => useRevision(fetcher, true));
    await settle();
    polls[0]?.answer(3);
    await settle();

    polls[1]?.fail();
    await settle();
    expect(polls).toHaveLength(2);
    await act(() => vi.advanceTimersByTimeAsync(RETRY_MS - 1));
    expect(polls).toHaveLength(2);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(polls).toHaveLength(3);
    expect(polls[2]?.since).toBe(3);
    expect(result.current).toBe(3);

    polls[2]?.answer(4);
    await settle();
    expect(result.current).toBe(4);
  });

  it("follows a revision that goes backwards, as after a server restart", async () => {
    const { fetcher, polls } = fakeChanges();
    const { result } = renderHook(() => useRevision(fetcher, true));
    await settle();
    polls[0]?.answer(12);
    await settle();
    expect(result.current).toBe(12);

    polls[1]?.answer(2);
    await settle();
    expect(result.current).toBe(2);
    expect(polls[2]?.since).toBe(2);
  });

  it("does not poll while disabled, and aborts its poll on unmount", async () => {
    const { fetcher, polls } = fakeChanges();
    const { rerender, unmount } = renderHook(
      ({ enabled }) => useRevision(fetcher, enabled),
      { initialProps: { enabled: false } },
    );
    await settle();
    expect(polls).toHaveLength(0);

    rerender({ enabled: true });
    await settle();
    expect(polls).toHaveLength(1);
    unmount();
    expect(polls[0]?.signal?.aborted).toBe(true);
    // An aborted poll does not retry.
    polls[0]?.fail();
    await act(() => vi.advanceTimersByTimeAsync(RETRY_MS * 2));
    expect(polls).toHaveLength(1);
  });
});
