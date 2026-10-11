import { describe, expect, it } from "vitest";

import { dayAndTime, durationWords, runningTime } from "./time";

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

describe("dayAndTime", () => {
  // Local times, so the day words hold in any time zone.
  const evening = new Date(2026, 9, 9, 20, 30);
  const clock = (date: Date) =>
    date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

  it("names today and yesterday in words", () => {
    const morning = new Date(2026, 9, 9, 9, 5);
    const yesterday = new Date(2026, 9, 8, 18, 0);
    expect(dayAndTime(morning.toISOString(), evening)).toBe(
      `today ${clock(morning)}`,
    );
    expect(dayAndTime(yesterday.toISOString(), evening)).toBe(
      `yesterday ${clock(yesterday)}`,
    );
  });

  it("names a weekday within the week, then the date", () => {
    const monday = new Date(2026, 9, 5, 9, 12);
    const earlier = new Date(2026, 8, 20, 9, 12);
    expect(dayAndTime(monday.toISOString(), evening)).toBe(
      `${monday.toLocaleDateString([], { weekday: "short" })} ${clock(monday)}`,
    );
    expect(dayAndTime(earlier.toISOString(), evening)).toBe(
      `${earlier.toLocaleDateString([], { day: "numeric", month: "short" })} ${clock(earlier)}`,
    );
  });

  it("returns an empty string for missing or invalid input", () => {
    expect(dayAndTime("", evening)).toBe("");
    expect(dayAndTime("soon", evening)).toBe("");
  });
});
