import { describe, expect, it } from "vitest";

import { compareTimes, durationWords, runningTime } from "./time";

describe("compareTimes", () => {
  it("orders by time, not text, when the server drops trailing zeros", () => {
    // The server writes RFC 3339 with trimmed fractions: .3 is 300 ms,
    // before .315, though it sorts after it as text.
    const first = "2026-10-12T13:35:37.3Z";
    const second = "2026-10-12T13:35:37.315Z";
    expect(first.localeCompare(second)).toBeGreaterThan(0);
    expect(compareTimes(first, second)).toBeLessThan(0);
    expect([second, "2026-10-12T13:35:37Z", first].sort(compareTimes)).toEqual([
      "2026-10-12T13:35:37Z",
      first,
      second,
    ]);
  });

  it("puts missing or invalid times first, as equals", () => {
    expect(compareTimes("", "not a time")).toBe(0);
    expect(compareTimes("", "2026-10-12T13:35:37Z")).toBeLessThan(0);
  });
});

const now = new Date("2026-10-04T12:00:00Z");

describe("runningTime", () => {
  it("rounds to the largest sensible unit", () => {
    expect(runningTime("2026-10-04T11:59:40Z", now)).toBe("now");
    expect(runningTime("2026-10-04T11:55:00Z", now)).toBe("5m");
    expect(runningTime("2026-10-04T09:00:00Z", now)).toBe("3h");
    expect(runningTime("2026-10-01T12:00:00Z", now)).toBe("3d");
    expect(runningTime("2026-08-01T12:00:00Z", now)).toBe("9w");
    expect(runningTime("2025-08-01T12:00:00Z", now)).toBe("1y");
  });

  it("returns an empty string for missing or invalid input", () => {
    expect(runningTime("", now)).toBe("");
    expect(runningTime("yesterday", now)).toBe("");
  });

  it("treats future times as now", () => {
    expect(runningTime("2026-10-04T12:10:00Z", now)).toBe("now");
  });
});

describe("durationWords", () => {
  it("says how long ago in roomy words", () => {
    expect(durationWords("2026-10-04T11:59:40Z", now)).toBe("under a minute");
    expect(durationWords("2026-10-04T11:56:00Z", now)).toBe("4 min");
    expect(durationWords("2026-10-04T09:00:00Z", now)).toBe("3 h");
    expect(durationWords("2026-10-03T11:00:00Z", now)).toBe("1 day");
    expect(durationWords("2026-10-01T12:00:00Z", now)).toBe("3 days");
    expect(durationWords("2026-08-01T12:00:00Z", now)).toBe("9 weeks");
    expect(durationWords("2025-08-01T12:00:00Z", now)).toBe("1 year");
    expect(durationWords("", now)).toBe("");
  });
});
