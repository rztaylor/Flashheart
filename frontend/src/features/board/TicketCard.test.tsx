import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Card } from "../../api/board";
import type { Density } from "../filters/FilterBar";
import { TicketCard } from "./TicketCard";

// The card anatomy of docs/dev/specs/ui-layout.md §2: one order of rows at
// every density, a density only dropping rows from the end.
function card(overrides: Partial<Card> = {}): Card {
  return {
    project: "flashheart",
    id: "FH-7",
    slug: "x",
    column: "up-next",
    title: "Locked atomic ticket writes",
    type: "feature",
    priority: "high",
    workstream: "board-core",
    tags: [],
    created: "2026-10-01",
    updated: "",
    modified: "2026-10-06T10:00:00Z",
    branch: "",
    dependsOn: [],
    criteria: { done: 1, total: 3 },
    excerpt: "Write through a temp file and rename.",
    handoffNext: "Add the multi-process test",
    attachments: 2,
    hasReview: true,
    blocked: false,
    blockedBy: [],
    needsRepair: [],
    warnings: [],
    needsYou: false,
    agentWorking: false,
    openQuestions: 0,
    ...overrides,
  };
}

const blocker = {
  kind: "ticket" as const,
  text: "Depends on FH-4, which is in Backlog",
  missing: false,
};

function render(
  overrides: Partial<Card> = {},
  density: Density = "normal",
  paint?: { token: string; label: string },
) {
  return renderToStaticMarkup(
    <TicketCard
      card={card(overrides)}
      line={{ colour: 3, initials: "BC" }}
      workstreamTitle="Board core"
      density={density}
      paint={paint}
      tabIndex={0}
      now={new Date("2026-10-06T12:00:00Z")}
      onOpen={() => undefined}
    />,
  );
}

const order = (markup: string, ...parts: string[]) =>
  parts.map((part) => {
    const at = markup.indexOf(part);
    expect(at, part).toBeGreaterThanOrEqual(0);
    return at;
  });

describe("TicketCard", () => {
  it("opens with the id and running time, then the title, then the tags", () => {
    const markup = render();
    const [id, time, title, tag] = order(
      markup,
      ">FH-7<",
      ">2h<",
      ">Locked atomic ticket writes<",
      ">feature<",
    );
    expect(id).toBeLessThan(time ?? 0);
    expect(time).toBeLessThan(title ?? 0);
    expect(title).toBeLessThan(tag ?? 0);
  });

  it("ends with criteria left and priority right in the footer", () => {
    const markup = render();
    const [criteria, priority] = order(markup, ">1/3", ">High<");
    expect(criteria).toBeLessThan(priority ?? 0);
    expect(markup).toContain('data-row="footer"');
  });

  it("keeps type and priority in Compact, and drops criteria", () => {
    const markup = render({}, "compact");
    expect(markup).toContain(">feature<");
    expect(markup).toContain(">High<");
    expect(markup).not.toContain(">1/3");
  });

  it("adds excerpt, next step, attachments and review only in Detailed", () => {
    const normal = render();
    const detailed = render({}, "detailed");
    for (const part of [
      "Write through a temp file",
      "Add the multi-process test",
      'title="Attachments"',
      ">Review<",
    ]) {
      expect(normal).not.toContain(part);
      expect(detailed).toContain(part);
    }
  });

  it("shows a real blocker as the blocked pill with its reason", () => {
    const markup = render({ blocked: true, blockedBy: [blocker] });
    expect(markup).toContain('data-tone="blocked"');
    expect(markup).toContain("Depends on FH-4, which is in Backlog");
    const compact = render({ blocked: true, blockedBy: [blocker] }, "compact");
    expect(compact).toContain('data-tone="blocked"');
    expect(compact).not.toContain("which is in Backlog");
  });

  it("draws the stripe in the bullet's own line colour", () => {
    const markup = render();
    const stripes = markup.match(/var\(--fh-line-(\d)\)/g) ?? [];
    expect(new Set(stripes)).toEqual(new Set(["var(--fh-line-3)"]));
    expect(stripes.length).toBeGreaterThanOrEqual(2);
  });

  it("paints the header and fills the painted tag, naming its value", () => {
    const markup = render({}, "normal", {
      token: "type-feature",
      label: "feature",
    });
    expect(markup).toContain('data-paint="type-feature"');
    expect(markup).toContain("bg-(--paint-tint)");
  });

  it("wraps a long title to at most four lines", () => {
    expect(render()).toContain("line-clamp-4");
  });
});
