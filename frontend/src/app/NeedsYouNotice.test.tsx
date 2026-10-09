import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NeedsYouNotice } from "./NeedsYouNotice";

const render = (count: number) =>
  renderToStaticMarkup(
    <NeedsYouNotice count={count} onOpenOverview={() => undefined} />,
  );

describe("NeedsYouNotice (FH-44)", () => {
  it("is absent while every agent that needs you has a ticket shown", () => {
    expect(render(0)).toBe("");
  });

  it("names agents that need you with no ticket, and opens the Overview", () => {
    expect(render(1)).toContain(
      "1 agent needs you with no ticket on the board",
    );
    expect(render(2)).toContain(
      "2 agents need you with no ticket on the board",
    );
    expect(render(1)).toMatch(/<button[^>]*>Open Overview<\/button>/);
  });
});
