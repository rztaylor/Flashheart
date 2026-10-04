import { describe, expect, it } from "vitest";

import { runningTime } from "./time";

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
