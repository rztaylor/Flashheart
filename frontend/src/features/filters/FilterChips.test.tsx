import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Card, WorkstreamBrief } from "../../api/board";
import { emptyFilters, type Filters } from "../../model/filters";
import { FilterChips } from "./FilterChips";

const brief = (slug: string, done: number, total: number): WorkstreamBrief => ({
  slug,
  title: slug.toUpperCase(),
  created: "",
  status: "active",
  done,
  total,
});

const card = (workstream: string, column: Card["column"]) =>
  ({ workstream, column }) as Card;

const render = (filters: Filters = emptyFilters) =>
  renderToStaticMarkup(
    <FilterChips
      workstreams={[
        brief("quiet", 0, 1),
        brief("busy", 0, 2),
        brief("over", 2, 2),
      ]}
      lines={new Map()}
      cards={[card("busy", "in-progress"), card("busy", "review")]}
      paint="type"
      paints={[
        { token: "type-feature", label: "feature", value: "feature" },
        { token: "type-bug", label: "bug", value: "bug" },
      ]}
      filters={filters}
      onChange={() => undefined}
    />,
  );

describe("FilterChips", () => {
  it("lists open workstreams busiest first and leaves out finished ones", () => {
    const markup = render();
    expect(markup.indexOf("BUSY")).toBeLessThan(markup.indexOf("QUIET"));
    expect(markup).not.toContain("OVER");
  });

  it("makes the colour key a row of filter chips", () => {
    const markup = render({
      ...emptyFilters,
      type: { include: ["bug"], exclude: [] },
    });
    expect(markup).toContain('aria-label="Filter by type"');
    expect(markup).toMatch(/aria-pressed="true"[^>]*>(<[^>]*>)*bug/);
    expect(markup).toMatch(/aria-pressed="false"[^>]*>(<[^>]*>)*feature/);
  });

  it("marks each type chip with the type's own card icon (FH-147)", () => {
    const markup = render();
    expect(markup).toMatch(/data-type="feature"[^>]*>[\s\S]*?feature/);
    expect(markup).toContain('data-icon="star"');
    expect(markup).toContain('data-icon="bug"');
    expect(markup).toContain("color:var(--fh-paint-type-bug)");
  });

  it("keeps the round paint bullet for other Colour by values", () => {
    const markup = renderToStaticMarkup(
      <FilterChips
        workstreams={[]}
        lines={new Map()}
        cards={[]}
        paint="priority"
        paints={[{ token: "priority-high", label: "High", value: "high" }]}
        filters={emptyFilters}
        onChange={() => undefined}
      />,
    );
    expect(markup).toContain("--paint:var(--fh-paint-priority-high)");
    expect(markup).not.toContain("data-icon");
  });

  it("shows an excluded workstream as hidden", () => {
    const markup = render({
      ...emptyFilters,
      workstream: { include: [], exclude: ["quiet"] },
    });
    expect(markup).toMatch(/QUIET(<[^>]*>|[^<])*, hidden/);
  });

  it("never wraps the workstream row", () => {
    expect(render()).toContain("flex-nowrap");
  });
});
