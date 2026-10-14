import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Card } from "../../api/board";
import type { Density } from "../../api/preferences";
import { type Live, noReason } from "../../api/runs";
import { TicketCard } from "./TicketCard";

// The card of docs/dev/specs/ui-layout.md §2 (FH-147): one set of markers,
// three layouts. Compact is the title led by its id with the attention and
// priority marks beside it; Normal adds one meta line of markers and a
// needs-you reason line; Detailed adds the words.
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
    pr: "",
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

const live: Live = {
  run: "claude:3f2a9c1e",
  short: "claude:3f2a9c1e",
  agent: "claude",
  state: "working",
  reason: noReason,
  lastActivity: "2026-10-06T11:58:00Z",
};

const needsYou: Live = {
  ...live,
  state: "needs-you",
  reason: { ...noReason, kind: "permission", permission: "Bash" },
};

function render(
  overrides: Partial<Card> = {},
  density: Density = "normal",
  paint?: { token: string; label: string; value: string },
  showProject?: string,
) {
  return renderToStaticMarkup(
    <TicketCard
      card={card(overrides)}
      line={{ colour: 3, initials: "BC" }}
      workstreamTitle="Board core"
      density={density}
      paint={paint}
      showProject={showProject}
      tabIndex={0}
      now={new Date("2026-10-06T12:00:00Z")}
      onOpen={() => undefined}
    />,
  );
}

// part is the markup of the element marked data-<name>="<value>", up to
// its closing tag at the same depth.
function part(markup: string, attribute: string): string {
  const start = markup.indexOf(attribute);
  if (start < 0) return "";
  const open = markup.lastIndexOf("<", start);
  const tag = markup.slice(open + 1).match(/^\w+/)?.[0] ?? "span";
  let depth = 0;
  const pattern = new RegExp(`<(/?)${tag}\\b[^>]*?(/?)>`, "g");
  pattern.lastIndex = open;
  for (let match = pattern.exec(markup); match; match = pattern.exec(markup)) {
    if (match[2]) continue;
    depth += match[1] ? -1 : 1;
    if (depth === 0) return markup.slice(open, pattern.lastIndex);
  }
  return markup.slice(open);
}

// text is what the markup shows, without its tags and attributes (which
// hold the tooltips and the accessible name).
const text = (markup: string) => markup.replace(/<[^>]*>/g, "");

// name is the card's accessible name, its covering button's label.
const name = (markup: string) =>
  markup.match(/<button[^>]*aria-label="([^"]*)"/)?.[1] ?? "";

// idLink is the ticket id as a link to its full page in a new tab (KEY-3).
const idLink = (id: string) =>
  new RegExp(
    `<a [^>]*href="#/ticket/${id}"[^>]*target="_blank"[^>]*>${id}</a>`,
  );
// linkInButton finds a link nested in a button, which is not allowed.
const linkInButton = /<button[^>]*>(?:(?!<\/button>)[\s\S])*<a /;

const densities = ["compact", "normal", "detailed"] as const;

describe("TicketCard, at every density", () => {
  it.each(densities)("links its id outside the card's button (%s)", (d) => {
    const markup = render({}, d);
    expect(markup).toMatch(idLink("FH-7"));
    expect(markup).not.toMatch(linkInButton);
    expect(markup).toContain('data-ticket="FH-7"');
  });

  it.each(densities)(
    "draws no workstream bullet, only the stripe (%s)",
    (d) => {
      const markup = render({}, d);
      expect(markup).not.toContain(">BC<");
      expect(markup).not.toContain("data-line-bullet");
      const stripes = markup.match(/var\(--fh-line-(\d)\)/g) ?? [];
      expect(new Set(stripes)).toEqual(new Set(["var(--fh-line-3)"]));
    },
  );

  it.each(densities)("shows no acceptance criteria count (%s)", (d) => {
    const markup = render({}, d);
    expect(markup).not.toContain("1/3");
    expect(markup).not.toContain('data-icon="criteria"');
  });

  it.each(densities)("shows the priority marker (%s)", (d) => {
    const marks = part(render({}, d), 'data-mark="priority"');
    expect(marks).toContain('data-priority="high"');
    expect(marks).toContain('title="High priority"');
  });

  it.each(densities)("leaves a priority without bars to its name (%s)", (d) => {
    const markup = render({ priority: "urgent" }, d);
    expect(markup).not.toContain('data-mark="priority"');
    expect(name(markup)).toContain("urgent priority");
  });

  it.each(densities)("names every marker it can show (%s)", (d) => {
    const label = name(
      render(
        {
          blocked: true,
          blockedBy: [blocker],
          live,
          changeRequests: { open: 2, addressed: 0 },
        },
        d,
      ),
    );
    for (const words of [
      "Locked atomic ticket writes, FH-7",
      "blocked",
      "Claude working",
      "2 changes requested",
      "feature",
      "high priority",
      "2 attachments",
      "review written",
      "changed 2h ago",
      "Board core workstream",
    ])
      expect(label, words).toContain(words);
    // A card never promises a plan (D51).
    expect(label).not.toMatch(/plan/i);
  });

  it.each(densities)(
    "tints the whole card with the Colour by paint (%s)",
    (d) => {
      const markup = render({}, d, {
        token: "type-feature",
        label: "feature",
        value: "feature",
      });
      const root = markup.slice(0, markup.indexOf(">"));
      expect(root).toContain('data-paint="type-feature"');
      expect(root).toContain("bg-(--paint-tint)");
      expect(markup).not.toContain('data-row="header"');
    },
  );

  it.each(densities)("never paints a card that needs repair (%s)", (d) => {
    const markup = render({ needsRepair: ["frontmatter does not parse"] }, d, {
      token: "type-feature",
      label: "feature",
      value: "feature",
    });
    expect(markup).not.toContain("data-paint");
    expect(markup).not.toContain("--paint-tint");
    expect(name(markup)).toContain("needs repair");
  });

  it.each(densities)("keeps the full title in its tooltip (%s)", (d) => {
    const markup = render({ title: "A".repeat(200) }, d);
    expect(markup).toContain(`title="${"A".repeat(200)}"`);
    expect(name(markup)).toContain("A".repeat(200));
  });

  it.each(densities)("names a rejected ticket in Done (%s, FH-58)", (d) => {
    const markup = render({ column: "done", rejected: true }, d);
    expect(name(markup)).toContain(", rejected");
    expect(render({ column: "done" }, d)).not.toContain("Rejected");
  });
});

describe("a Compact card", () => {
  it("starts its title with the id and clamps it to two lines", () => {
    const markup = render({}, "compact");
    const title = part(markup, 'data-part="title"');
    expect(title).toContain("line-clamp-2");
    expect(text(title)).toBe("FH-7Locked atomic ticket writes");
  });

  it("has no other row: no meta line, words, type or age", () => {
    const markup = render({ live, excerpt: "Excerpt here" }, "compact");
    for (const absent of [
      'data-row="meta"',
      'data-row="needs-you"',
      'data-mark="type"',
      ">2h<",
      "Excerpt here",
      "Add the multi-process test",
      "Board core<",
      "data-run-state",
    ])
      expect(markup, absent).not.toContain(absent);
  });

  it("puts only attention and priority marks beside the title", () => {
    const quiet = part(render({}, "compact"), 'data-part="marks"');
    expect(quiet).toContain('data-mark="priority"');
    expect(quiet).not.toContain('data-mark="blocked"');
    expect(quiet).not.toContain('data-mark="requests"');
    expect(quiet).not.toContain('data-mark="needs-you"');

    const busy = part(
      render(
        {
          blocked: true,
          blockedBy: [blocker],
          live: needsYou,
          changeRequests: { open: 1, addressed: 0 },
        },
        "compact",
      ),
      'data-part="marks"',
    );
    const [needs, blocked, requests, priority] = [
      'data-mark="needs-you"',
      'data-mark="blocked"',
      'data-mark="requests"',
      'data-mark="priority"',
    ].map((mark) => busy.indexOf(mark));
    expect(needs).toBeGreaterThanOrEqual(0);
    expect(needs).toBeLessThan(blocked ?? 0);
    expect(blocked).toBeLessThan(requests ?? 0);
    expect(requests).toBeLessThan(priority ?? 0);
    expect(busy).toContain('data-icon="blocked"');
  });

  it("marks a waiting question as needing the user", () => {
    const marks = part(
      render({ openQuestions: 1 }, "compact"),
      'data-part="marks"',
    );
    expect(marks).toContain('data-mark="needs-you"');
  });

  it("leaves out the project name in All projects", () => {
    expect(render({}, "compact", undefined, "Flashheart")).not.toContain(
      ">Flashheart<",
    );
  });
});

describe("a Normal card", () => {
  it("clamps its title to three lines", () => {
    expect(part(render(), 'data-part="title"')).toContain("line-clamp-3");
  });

  it("holds every marker on one meta line, in order", () => {
    const meta = part(
      render({
        blocked: true,
        blockedBy: [blocker],
        live,
        changeRequests: { open: 2, addressed: 0 },
      }),
      'data-row="meta"',
    );
    const at = (mark: string) => {
      const index = meta.indexOf(mark);
      expect(index, mark).toBeGreaterThanOrEqual(0);
      return index;
    };
    const order = [
      at('data-mark="type"'),
      at(">FH-7<"),
      at('data-mark="blocked"'),
      at('data-mark="requests"'),
      at('data-mark="live"'),
      at('data-mark="age"'),
      at('data-mark="review"'),
      at('data-mark="attachments"'),
      at('data-mark="priority"'),
    ];
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    expect(meta).not.toContain(">2/5<");
    expect(meta).toContain(">2h<");
    expect(meta).toContain('data-icon="blocked"');
    expect(meta).toContain('data-icon="star"');
  });

  it("drops what does not apply", () => {
    const meta = part(
      render({ attachments: 0, hasReview: false }),
      'data-row="meta"',
    );
    for (const mark of ["blocked", "requests", "live", "review", "attachments"])
      expect(meta, mark).not.toContain(`data-mark="${mark}"`);
  });

  it("shows a run that needs the user with its reason on its own line", () => {
    const line = part(render({ live: needsYou }), 'data-row="needs-you"');
    expect(line).toContain("Needs you");
    expect(line).toContain("Bash");
    const question = part(render({ openQuestions: 2 }), 'data-row="needs-you"');
    expect(question).toContain("2 questions waiting");
    expect(render({ live })).not.toContain('data-row="needs-you"');
  });

  it("leaves the words to Detailed, in tooltips and the name meanwhile", () => {
    const markup = render({ blocked: true, blockedBy: [blocker], live });
    for (const absent of [
      "Write through a temp file",
      "Add the multi-process test",
      "which is in Backlog",
      "Fix header",
      "Board core",
      "Claude",
    ])
      expect(text(markup), absent).not.toContain(absent);
    expect(part(markup, 'data-mark="blocked"')).toContain(
      'title="Blocked: Depends on FH-4, which is in Backlog"',
    );
    expect(part(markup, 'data-mark="live"')).toContain(
      'title="Claude: Working"',
    );
  });

  it("leaves out the project name in All projects", () => {
    expect(render({}, "normal", undefined, "Flashheart")).not.toContain(
      ">Flashheart<",
    );
  });
});

describe("a Detailed card", () => {
  const markup = render(
    { blocked: true, blockedBy: [blocker, blocker], live: needsYou },
    "detailed",
    undefined,
    "Flashheart",
  );

  it("shows everything a Normal card does", () => {
    expect(markup).toContain('data-row="meta"');
    expect(markup).toContain('data-row="needs-you"');
    expect(part(markup, 'data-part="title"')).toContain("line-clamp-3");
  });

  it("adds the blocker's reason, the agent, and the next step", () => {
    expect(markup).toContain("Depends on FH-4, which is in Backlog (+1 more)");
    expect(markup).toContain('data-tone="blocked"');
    const run = part(markup, 'data-row="run"');
    expect(run).toContain("Claude");
    expect(run).not.toContain("2/5");
    expect(run).not.toContain("Fix header");
    expect(markup).toContain("Add the multi-process test");
  });

  it("adds a two-line excerpt and the workstream and project names", () => {
    const excerpt = part(markup, 'data-row="excerpt"');
    expect(excerpt).toContain("line-clamp-2");
    expect(excerpt).toContain("Write through a temp file");
    const where = part(markup, 'data-row="workstream"');
    expect(where).toContain("Board core");
    expect(where).toContain("Flashheart");
  });

  // The markers end a Detailed card (FH-168): the title leads straight into
  // the words, and the needs-you line stays under the title where Normal
  // has it. Normal keeps the meta line under the title.
  it("ends on its meta line, with Needs you still under the title", () => {
    const rows = markup.match(/data-row="[\w-]+"/g) ?? [];
    expect(rows).toEqual([
      'data-row="needs-you"',
      'data-row="run"',
      'data-row="excerpt"',
      'data-row="workstream"',
      'data-row="meta"',
    ]);
    const normal = render({ live: needsYou }, "normal");
    expect(normal.match(/data-row="[\w-]+"/g)).toEqual([
      'data-row="meta"',
      'data-row="needs-you"',
    ]);
  });
});

describe("change requests on a card (REV-6)", () => {
  it.each(densities)("counts them, with the words in the name (%s)", (d) => {
    const markup = render({ changeRequests: { open: 2, addressed: 1 } }, d);
    const mark = part(markup, 'data-mark="requests"');
    expect(mark).toContain(">2<");
    expect(mark).toContain('title="2 open · 1 addressed"');
    expect(name(markup)).toContain(", 2 open · 1 addressed");
  });

  it("colours addressed-only requests as waiting for review", () => {
    const mark = part(
      render({ changeRequests: { open: 0, addressed: 1 } }),
      'data-mark="requests"',
    );
    expect(mark).toContain("--fh-state-review-ink");
    expect(mark).toContain(">1<");
  });

  it("shows nothing when none waits", () => {
    expect(render({ changeRequests: { open: 0, addressed: 0 } })).not.toMatch(
      /data-mark="requests"|requested|addressed/,
    );
  });
});
