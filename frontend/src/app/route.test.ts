import { describe, expect, it } from "vitest";

import { formatRoute, parseRoute } from "./route";

describe("routes", () => {
  it("defaults to all projects on the board", () => {
    expect(parseRoute("")).toEqual({ scope: { kind: "all" }, view: "board" });
    expect(parseRoute("#/nonsense")).toEqual({
      scope: { kind: "all" },
      view: "board",
    });
  });

  it("round-trips projects, views and open tickets by id", () => {
    const route = {
      scope: { kind: "project" as const, project: "my project" },
      view: "workstreams" as const,
      ticket: { id: "FH-42" },
    };
    expect(parseRoute(formatRoute(route))).toEqual(route);
    expect(formatRoute({ scope: { kind: "all" }, view: "table" })).toBe(
      "#/all/table",
    );
    expect(formatRoute(route)).toBe("#/p/my%20project/workstreams?t=FH-42");
  });

  it("ignores unknown views and malformed ticket ids", () => {
    expect(parseRoute("#/p/alpha/agents")).toEqual({
      scope: { kind: "project", project: "alpha" },
      view: "board",
    });
    expect(parseRoute("#/all/board?t=../x")).toEqual({
      scope: { kind: "all" },
      view: "board",
    });
  });
});
