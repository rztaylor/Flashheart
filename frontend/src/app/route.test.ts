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

  it("routes the Overview, and lands an old Agents route there", () => {
    expect(parseRoute("#/p/alpha/overview")).toEqual({
      scope: { kind: "project", project: "alpha" },
      view: "overview",
    });
    expect(formatRoute({ scope: { kind: "all" }, view: "overview" })).toBe(
      "#/all/overview",
    );
    expect(parseRoute("#/p/alpha/agents?t=AL-3")).toEqual({
      scope: { kind: "project", project: "alpha" },
      view: "overview",
      ticket: { id: "AL-3" },
    });
  });

  it("ignores unknown views and malformed ticket ids", () => {
    expect(parseRoute("#/p/alpha/runs")).toEqual({
      scope: { kind: "project", project: "alpha" },
      view: "board",
    });
    expect(parseRoute("#/all/board?t=../x")).toEqual({
      scope: { kind: "all" },
      view: "board",
    });
  });

  it("routes a workstream's page by project and slug (FH-139)", () => {
    const route = {
      scope: { kind: "all" as const },
      view: "workstream" as const,
      workstream: { project: "my project", slug: "board-ui" },
    };
    expect(formatRoute(route)).toBe("#/workstream/my%20project/board-ui");
    expect(parseRoute("#/workstream/my%20project/board-ui")).toEqual(route);
    // A slug or project that cannot be one lands on the board.
    for (const hash of [
      "#/workstream/alpha",
      "#/workstream/alpha/Board%20UI",
      "#/workstream/../board-ui",
    ])
      expect(parseRoute(hash)).toEqual({
        scope: { kind: "all" },
        view: "board",
      });
    expect(parseRoute("#/all/workstream")).toEqual({
      scope: { kind: "all" },
      view: "board",
    });
  });

  it("routes a ticket's full page by id (KEY-3)", () => {
    expect(parseRoute("#/ticket/FH-42")).toEqual({
      scope: { kind: "all" },
      view: "ticket",
      ticket: { id: "FH-42" },
    });
    expect(
      formatRoute({
        scope: { kind: "project", project: "flashheart" },
        view: "ticket",
        ticket: { id: "FH-42" },
      }),
    ).toBe("#/ticket/FH-42");
  });

  it("falls back to the board for a full page without a valid id", () => {
    expect(parseRoute("#/ticket/../x")).toEqual({
      scope: { kind: "all" },
      view: "board",
    });
    expect(parseRoute("#/ticket")).toEqual({
      scope: { kind: "all" },
      view: "board",
    });
  });
});
