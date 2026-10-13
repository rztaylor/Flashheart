import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PriorityMarker, TypeIcon } from "./TicketMarks";

// The card's priority and type marks (FH-147, ui-layout.md §2): priority is
// three ascending bars, filled by level in its own colour role; type is an
// icon in the type's paint colour, a dot for a type without one.
describe("PriorityMarker", () => {
  it.each([
    ["high", 3],
    ["medium", 2],
    ["low", 1],
  ])("fills %s's bars in its colour", (level, filled) => {
    const markup = renderToStaticMarkup(<PriorityMarker priority={level} />);
    expect(markup).toContain(`data-priority="${level}"`);
    expect(markup.match(/data-bar="on"/g)).toHaveLength(filled);
    expect(markup.match(/data-bar="off"/g) ?? []).toHaveLength(3 - filled);
    expect(markup).toContain(`--fh-priority-${level}`);
    if (filled < 3) expect(markup).toContain("--fh-priority-off");
  });

  it("draws nothing for an unknown or missing priority", () => {
    expect(renderToStaticMarkup(<PriorityMarker priority="" />)).toBe("");
    expect(renderToStaticMarkup(<PriorityMarker priority="urgent" />)).toBe("");
  });

  it("is a drawing; its word is for the tooltip and the name around it", () => {
    expect(renderToStaticMarkup(<PriorityMarker priority="high" />)).toContain(
      'aria-hidden="true"',
    );
  });
});

describe("TypeIcon", () => {
  it.each([
    ["feature", "star"],
    ["bug", "bug"],
    ["infra", "server"],
    ["test", "beaker"],
    ["refactor", "swap"],
    ["docs", "doc"],
    ["spike", "bulb"],
  ])("draws %s as its icon in the type's colour", (type, icon) => {
    const markup = renderToStaticMarkup(<TypeIcon type={type} />);
    expect(markup).toContain(`data-icon="${icon}"`);
    expect(markup).toContain(`color:var(--fh-paint-type-${type})`);
    expect(markup).toContain(`data-type="${type}"`);
  });

  it("falls back to a dot in the other colour for a type without an icon", () => {
    const markup = renderToStaticMarkup(<TypeIcon type="chore" />);
    expect(markup).toContain('data-type="chore"');
    expect(markup).toContain("data-type-dot");
    expect(markup).toContain("color:var(--fh-paint-type-other)");
    expect(markup).not.toContain("data-icon");
  });
});
