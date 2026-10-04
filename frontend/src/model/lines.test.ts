import { describe, expect, it } from "vitest";

import { assignLines, LINE_COUNT, lineInitials } from "./lines";

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
