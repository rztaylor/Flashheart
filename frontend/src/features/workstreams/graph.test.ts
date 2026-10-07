import { describe, expect, it } from "vitest";

import { layoutGraph } from "./graph";

const t = (id: string, ...dependsOn: string[]) => ({ id, dependsOn });

const at = (layout: ReturnType<typeof layoutGraph>, id: string) => {
  const station = layout.stations.find((item) => item.id === id);
  return station ? [station.layer, station.lane] : undefined;
};

describe("layoutGraph", () => {
  it("draws a chain of dependencies as one straight line", () => {
    const layout = layoutGraph([t("A"), t("B", "A"), t("C", "B")]);
    expect(layout.stations.map((s) => [s.id, s.layer, s.lane])).toEqual([
      ["A", 0, 0],
      ["B", 1, 0],
      ["C", 2, 0],
    ]);
    expect(layout.edges.map((e) => [e.from, e.to, e.route])).toEqual([
      ["A", "B", "straight"],
      ["B", "C", "straight"],
    ]);
    expect(layout.free).toEqual([]);
    expect([layout.layers, layout.lanes]).toEqual([3, 1]);
  });

  it("branches independent work onto parallel tracks", () => {
    // B and C both need A, and nothing else orders them.
    const layout = layoutGraph([t("A"), t("B", "A"), t("C", "A")]);
    expect(at(layout, "B")).toEqual([1, 0]);
    expect(at(layout, "C")).toEqual([1, 1]);
    expect(
      layout.edges.find((e) => e.from === "A" && e.to === "C")?.route,
    ).toBe("branch");
  });

  it("merges every track a ticket depends on into it", () => {
    // D depends on both B and C, which run in parallel after A.
    const layout = layoutGraph([
      t("A"),
      t("B", "A"),
      t("C", "A"),
      t("D", "B", "C"),
    ]);
    expect(at(layout, "D")).toEqual([2, 0]);
    expect(
      layout.edges.filter((e) => e.to === "D").map((e) => [e.from, e.route]),
    ).toEqual([
      ["B", "straight"],
      ["C", "merge"],
    ]);
  });

  it("keeps the trunk on the top track when tracks join", () => {
    // D needs C and B equally late; the top track (B's) carries on.
    const layout = layoutGraph([
      t("A"),
      t("B", "A"),
      t("C", "A"),
      t("D", "C", "B"),
      t("E", "D"),
    ]);
    expect(at(layout, "D")).toEqual([2, 0]);
    expect(at(layout, "E")).toEqual([3, 0]);
  });

  it("places a ticket after the longest chain it depends on", () => {
    // D needs A directly and C, which comes after B after A.
    const layout = layoutGraph([
      t("A"),
      t("B", "A"),
      t("C", "B"),
      t("D", "A", "C"),
    ]);
    expect(at(layout, "D")).toEqual([3, 0]);
    const skip = layout.edges.find((e) => e.from === "A" && e.to === "D");
    // A, B, C and D share a track, so the long edge from A takes a detour
    // on a free track rather than running through B and C.
    expect([skip?.route, skip?.via]).toEqual(["detour", 1]);
    expect(layout.lanes).toBe(2);
  });

  it("starts unrelated chains on their own tracks", () => {
    const layout = layoutGraph([t("A"), t("B", "A"), t("X"), t("Y", "X")]);
    expect(at(layout, "A")).toEqual([0, 0]);
    expect(at(layout, "X")).toEqual([0, 1]);
    expect(at(layout, "Y")).toEqual([1, 1]);
  });

  it("leaves tickets with no dependencies on the line as free stations", () => {
    const layout = layoutGraph([t("A"), t("F"), t("B", "A"), t("G", "Z")]);
    expect(layout.stations.map((s) => s.id)).toEqual(["A", "B"]);
    // G depends only on Z, which is not on this line.
    expect(layout.free).toEqual(["F", "G"]);
  });

  it("survives a dependency cycle and a repeated id", () => {
    const layout = layoutGraph([t("A", "B"), t("B", "A"), t("A")]);
    expect(layout.stations).toHaveLength(2);
    expect(layout.edges).toHaveLength(1);
    expect(layout.free).toEqual([]);
  });
});
