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

const card = (workstream: string, column: Card["column"], needsYou = false) =>
  ({ workstream, column, needsYou }) as Card;

const render = (
  filters: Filters = emptyFilters,
  cards = [card("busy", "in-progress"), card("busy", "review")],
) =>
  renderToStaticMarkup(
    <FilterChips
      workstreams={[
        brief("quiet", 0, 1),
        brief("busy", 0, 2),
        brief("over", 2, 2),
      ]}
      lines={new Map()}
      cards={cards}
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

  it("leads with a Needs you chip while a ticket needs you (FH-44)", () => {
    expect(render()).not.toContain("Needs you");
    const waiting = [card("busy", "in-progress", true), card("busy", "review")];
    const markup = render(emptyFilters, waiting);
    expect(markup).toContain('aria-label="Filter by tickets that need you"');
    expect(markup.indexOf("Needs you")).toBeLessThan(markup.indexOf("BUSY"));
    expect(markup).toMatch(/aria-pressed="false"[^>]*>(<[^>]*>)*Needs you/);
    expect(render({ ...emptyFilters, needsYou: "included" }, waiting)).toMatch(
      /aria-pressed="true"[^>]*>(<[^>]*>)*Needs you/,
    );
  });

  it("keeps a chosen Needs you chip to restore once nothing needs you", () => {
    expect(render({ ...emptyFilters, needsYou: "excluded" })).toContain(
      'aria-label="Needs you, hidden"',
    );
  });
});
