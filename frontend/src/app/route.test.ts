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

  it("round-trips projects, views and open tickets", () => {
    const route = {
      scope: { kind: "project" as const, project: "my project" },
      view: "workstreams" as const,
      ticket: { project: "my project", slug: "feat--x" },
    };
    expect(parseRoute(formatRoute(route))).toEqual(route);
    expect(formatRoute({ scope: { kind: "all" }, view: "table" })).toBe(
      "#/all/table",
    );
    expect(formatRoute(route)).toBe(
      "#/p/my%20project/workstreams?t=my%20project%2Ffeat--x",
    );
  });

  it("ignores unknown views", () => {
    expect(parseRoute("#/p/alpha/agents")).toEqual({
      scope: { kind: "project", project: "alpha" },
      view: "board",
    });
  });
});
