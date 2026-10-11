import { describe, expect, it, vi } from "vitest";

import { fetchMetrics, isMetrics } from "./metrics";

const metrics = {
  revision: 4,
  since: "2026-10-08T18:00:00Z",
  lastChange: true,
  done: 0,
  review: 3,
  created: 6,
  criteriaTicked: 9,
};

describe("isMetrics", () => {
  it("accepts the headline metrics", () => {
    expect(isMetrics(metrics)).toBe(true);
    expect(isMetrics({ ...metrics, lastChange: false })).toBe(true);
  });

  it("rejects a missing or malformed field", () => {
    expect(isMetrics({ ...metrics, criteriaTicked: undefined })).toBe(false);
    expect(isMetrics({ ...metrics, done: -1 })).toBe(false);
    expect(isMetrics({ ...metrics, review: "3" })).toBe(false);
    expect(isMetrics({ ...metrics, lastChange: "yes" })).toBe(false);
    expect(isMetrics(null)).toBe(false);
  });
});

describe("fetchMetrics", () => {
  it("asks for a project's metrics, or every project's", async () => {
    const fetcher = vi.fn(
      async (_input: RequestInfo | URL, _init?: RequestInit) =>
        new Response(JSON.stringify(metrics), { status: 200 }),
    );
    await expect(fetchMetrics(fetcher, "alpha")).resolves.toEqual(metrics);
    await fetchMetrics(fetcher, "");
    expect(fetcher.mock.calls.map((call) => call[0])).toEqual([
      "/api/metrics?project=alpha",
      "/api/metrics",
    ]);
  });
});
