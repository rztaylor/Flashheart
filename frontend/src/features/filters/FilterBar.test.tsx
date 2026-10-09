import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { emptyFilters, type Filters } from "../../model/filters";
import { FilterBar } from "./FilterBar";

const options = {
  types: ["bug", "feature"],
  priorities: ["high", "low"],
  workstreams: [{ value: "board-ui", label: "Board UI" }],
};

const render = (filters: Filters = emptyFilters, board = true) =>
  renderToStaticMarkup(
    <FilterBar
      filters={filters}
      options={options}
      onChange={() => undefined}
      {...(board
        ? {
            view: {
              density: "normal",
              onDensity: () => undefined,
              paint: "type",
              onPaint: () => undefined,
              hiddenColumns: [],
              onHiddenColumns: () => undefined,
            },
          }
        : {})}
    />,
  );

describe("FilterBar", () => {
  it("names each filter on its own button, with no separate label or select", () => {
    const markup = render();
    for (const name of ["Type", "Priority", "Workstream", "State"]) {
      expect(markup).toMatch(
        new RegExp(
          `<button[^>]*aria-expanded="false"[^>]*aria-label="${name}"`,
        ),
      );
    }
    expect(markup).not.toContain("<select");
    expect(markup).not.toContain("<label");
  });

  it("has no Hide later filter or New ticket button", () => {
    const markup = render();
    expect(markup).not.toContain("Hide later");
    expect(markup).not.toContain("New ticket");
  });

  it("badges an active filter with its count and offers Clear filters", () => {
    expect(render()).not.toContain("Clear filters");
    const markup = render({
      ...emptyFilters,
      type: { include: ["bug"], exclude: ["feature"] },
      state: "blocked",
    });
    expect(markup).toMatch(/Type<\/span><span[^>]*>2<\/span>/);
    expect(markup).toContain('aria-label="Type: 2 chosen"');
    expect(markup).toContain("State: Blocked");
    expect(markup).toContain("Clear filters");
  });

  it("counts chip-only filters toward Clear filters", () => {
    expect(
      render({ ...emptyFilters, age: { include: ["today"], exclude: [] } }),
    ).toContain("Clear filters");
  });

  it("puts the board's view settings behind one menu button", () => {
    const markup = render();
    expect(markup).toMatch(
      /aria-label="View options"[^>]*aria-expanded="false"|aria-expanded="false"[^>]*aria-label="View options"/,
    );
    expect(markup).not.toContain("Colour by");
    expect(markup).not.toContain("Compact");
    expect(render(emptyFilters, false)).not.toContain("View options");
  });
});
