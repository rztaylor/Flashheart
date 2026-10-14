import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NeedsYouEmpty, NeedsYouNotice } from "./NeedsYouNotice";

const notice = (count: number) =>
  renderToStaticMarkup(
    <NeedsYouNotice count={count} onAnswer={() => undefined} />,
  );

const empty = (count: number, otherFilters = false) =>
  renderToStaticMarkup(
    <NeedsYouEmpty
      count={count}
      otherFilters={otherFilters}
      onAnswer={() => undefined}
    />,
  );

// The primary button's whole markup, by its words.
const primary = (words: string) =>
  new RegExp(`<button[^>]*class="[^"]*bg-action[^"]*"[^>]*>${words}</button>`);

describe("NeedsYouNotice (FH-44, FH-157)", () => {
  it("is absent while every agent that needs you has a ticket shown", () => {
    expect(notice(0)).toBe("");
  });

  it("says why the other agents have no card, and where to answer them", () => {
    expect(notice(1)).toContain("1 more agent needs you, not about a ticket.");
    expect(notice(1)).toContain(
      "It asked about the project or a workstream, so no card can show it.",
    );
    expect(notice(2)).toContain("2 more agents need you, not about a ticket.");
    expect(notice(2)).toContain(
      "They asked about a project or a workstream, so no card can show them.",
    );
  });

  // The label says what pressing it gets, without the text around it.
  it("offers a primary button that says what it does", () => {
    expect(notice(1)).toMatch(primary("Answer it in the Overview"));
    expect(notice(2)).toMatch(primary("Answer them in the Overview"));
    expect(notice(1)).not.toContain("Open Overview");
  });
});

describe("NeedsYouEmpty (FH-157)", () => {
  it("explains what waits, why no card shows it and where to answer it", () => {
    const html = empty(1);
    expect(html).toContain('aria-label="Agents without a ticket"');
    expect(html).toContain("1 agent needs you, not about a ticket");
    expect(html).toContain(
      "It asked about the project or a workstream, so no card can show it.",
    );
    expect(html).toContain(
      "It is waiting in the Overview, under Needs your decision.",
    );
    expect(html).toMatch(primary("Answer it in the Overview"));
    expect(html).not.toContain("Clear a filter");
  });

  it("words several agents in the plural", () => {
    const html = empty(3);
    expect(html).toContain("3 agents need you, not about a ticket");
    expect(html).toContain(
      "They asked about a project or a workstream, so no card can show them.",
    );
    expect(html).toContain(
      "They are waiting in the Overview, under Needs your decision.",
    );
    expect(html).toMatch(primary("Answer them in the Overview"));
  });

  it("says other filters may hide tickets that need you", () => {
    expect(empty(1, true)).toContain(
      "Other filters are on too: clear them to see any tickets that need you.",
    );
    expect(empty(1)).not.toContain("Other filters");
  });

  it("says nothing needs you when no agent is waiting, without filter advice", () => {
    const html = empty(0);
    expect(html).toContain("Nothing needs you right now");
    expect(html).toContain(
      "When an agent asks you about a ticket, its card shows here.",
    );
    expect(html).not.toContain("Clear a filter");
    expect(html).not.toContain("<button");
  });
});
