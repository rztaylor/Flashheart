import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PriorityMarker, PriorityTag, TypeIcon, TypeTag } from "./TicketMarks";

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

// Where there is room, the marks stand beside their words (FH-154): the
// Table, the archive and the card panel's header show type and priority
// as tags with the card's icon or bars before the word.
describe("TypeTag and PriorityTag", () => {
  it("puts the type's icon before its word", () => {
    const markup = renderToStaticMarkup(<TypeTag type="bug" />);
    expect(markup).toMatch(/data-icon="bug"[\s\S]*<\/span>bug<\/span>$/);
  });

  it("puts the priority's bars before its word, set strong for high", () => {
    const markup = renderToStaticMarkup(<PriorityTag priority="high" />);
    expect(markup).toMatch(/data-priority="high"[\s\S]*<\/svg>High<\/span>$/);
    expect(markup).toContain("font-semibold");
    expect(renderToStaticMarkup(<PriorityTag priority="low" />)).not.toContain(
      "font-semibold",
    );
  });

  it("takes other words for the priority, as the panel's High priority", () => {
    const markup = renderToStaticMarkup(
      <PriorityTag priority="medium">Medium priority</PriorityTag>,
    );
    expect(markup).toContain('data-priority="medium"');
    expect(markup).toContain("Medium priority");
  });

  it("keeps the word alone for a priority without bars", () => {
    const markup = renderToStaticMarkup(<PriorityTag priority="urgent" />);
    expect(markup).not.toContain("data-priority");
    expect(markup).toContain(">urgent</span>");
  });
});
