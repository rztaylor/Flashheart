import { describe, expect, it } from "vitest";

import {
  assignLines,
  LINE_COUNT,
  lineInitials,
  takenWorkstream,
  workstreamTitle,
  workstreamTitleKey,
} from "./lines";

describe("assignLines", () => {
  it("gives every workstream in a project a distinct line while colours last", () => {
    const lines = assignLines([
      { slug: "board-ui", created: "2026-10-01" },
      { slug: "storage", created: "2026-10-02" },
      { slug: "agents", created: "2026-10-03" },
    ]);
    const colours = [...lines.values()].map((line) => line.colour);
    expect(new Set(colours).size).toBe(3);
    for (const colour of colours) {
      expect(colour).toBeGreaterThanOrEqual(0);
      expect(colour).toBeLessThan(LINE_COUNT);
    }
  });

  it("keeps existing colours when a newer workstream is added", () => {
    const before = assignLines([
      { slug: "board-ui", created: "2026-10-01" },
      { slug: "storage", created: "2026-10-02" },
    ]);
    const after = assignLines([
      { slug: "board-ui", created: "2026-10-01" },
      { slug: "storage", created: "2026-10-02" },
      { slug: "zeta", created: "2026-10-05" },
    ]);
    expect(after.get("board-ui")?.colour).toBe(before.get("board-ui")?.colour);
    expect(after.get("storage")?.colour).toBe(before.get("storage")?.colour);
  });

  it("reuses colours only once every line is taken", () => {
    const many = Array.from({ length: LINE_COUNT + 2 }, (_, n) => ({
      slug: `ws-${n}`,
      created: "2026-10-01",
    }));
    const colours = [...assignLines(many).values()].map((line) => line.colour);
    expect(new Set(colours).size).toBe(LINE_COUNT);
  });

  it("is deterministic regardless of input order", () => {
    const a = assignLines([
      { slug: "x", created: "2026-10-01" },
      { slug: "y", created: "2026-10-01" },
    ]);
    const b = assignLines([
      { slug: "y", created: "2026-10-01" },
      { slug: "x", created: "2026-10-01" },
    ]);
    expect(a.get("x")).toEqual(b.get("x"));
  });
});

describe("lineInitials", () => {
  it("uses up to two word initials", () => {
    expect(lineInitials("board-ui")).toBe("BU");
    expect(lineInitials("storage")).toBe("S");
    expect(lineInitials("ofqual-regulatory-layer")).toBe("OR");
    expect(lineInitials("")).toBe("?");
  });
});

describe("workstreamTitle", () => {
  const brief = (slug: string, title: string) => ({
    slug,
    title,
    created: "2026-10-01",
    status: "active" as const,
    done: 0,
    total: 1,
  });
  const workstreams = new Map([
    ["flashheart", [brief("board-ui", "Board UI"), brief("sync", "Sync")]],
    ["other", [brief("board-ui", "Another board")]],
  ]);

  it("names a workstream by its project's title for the slug", () => {
    expect(workstreamTitle(workstreams, "flashheart", "board-ui")).toBe(
      "Board UI",
    );
    expect(workstreamTitle(workstreams, "other", "board-ui")).toBe(
      "Another board",
    );
  });

  it("falls back to the slug for a workstream or project it does not know", () => {
    expect(workstreamTitle(workstreams, "flashheart", "gone")).toBe("gone");
    expect(workstreamTitle(workstreams, "missing", "sync")).toBe("sync");
    expect(workstreamTitle(workstreams, "flashheart", "")).toBe("");
  });
});

// A project's workstream titles are unique, compared ignoring case and
// runs of spaces, as the server compares them (FH-163, D76).
describe("takenWorkstream", () => {
  const brief = (slug: string, title: string) => ({
    slug,
    title,
    created: "2026-10-01",
    status: "active" as const,
    done: 0,
    total: 1,
  });
  const workstreams = [
    brief("board-ui", "Board UI"),
    brief("workstream-editing", "Workstream editing"),
  ];

  it("finds the workstream that already has a title, in any case or spacing", () => {
    expect(takenWorkstream(workstreams, "Board UI")?.slug).toBe("board-ui");
    expect(takenWorkstream(workstreams, "  workstream \t EDITING ")?.slug).toBe(
      "workstream-editing",
    );
  });

  it("finds none for a new title, an empty one, or the workstream's own", () => {
    expect(takenWorkstream(workstreams, "Board UI!")).toBeUndefined();
    expect(takenWorkstream(workstreams, "Workstream-editing")).toBeUndefined();
    expect(takenWorkstream(workstreams, "   ")).toBeUndefined();
    expect(
      takenWorkstream(workstreams, "board ui", "board-ui"),
    ).toBeUndefined();
  });

  it("compares titles as the server does", () => {
    expect(workstreamTitleKey("  Équipe   Board ")).toBe("équipe board");
  });
});
