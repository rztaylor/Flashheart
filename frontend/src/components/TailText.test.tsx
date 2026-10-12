import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BranchLabel } from "./BranchLabel";
import { TailText } from "./TailText";

const branch = "feature/a-very-long-branch-name-that-overflows-the-panel";

describe("TailText", () => {
  it("cuts from the start, keeps the value's order and its full title", () => {
    const markup = renderToStaticMarkup(<TailText>{branch}</TailText>);
    expect(markup).toContain("[direction:rtl]");
    expect(markup).toContain("truncate");
    expect(markup).toContain(`title="${branch}"`);
    expect(markup).toContain(`<bdi>${branch}</bdi>`);
    expect(markup).toMatch(/^<span /);
  });

  it("renders as a description in a list", () => {
    const markup = renderToStaticMarkup(
      <TailText as="dd" className="font-mono">
        /Users/example/src/flashheart
      </TailText>,
    );
    expect(markup).toMatch(/^<dd class="[^"]*font-mono/);
  });
});

describe("BranchLabel", () => {
  it("shows the branch through TailText, named for assistive technology", () => {
    const markup = renderToStaticMarkup(<BranchLabel branch={branch} />);
    expect(markup).toContain("<svg");
    expect(markup).toContain('<span class="sr-only">Branch </span>');
    expect(markup).toContain(
      renderToStaticMarkup(<TailText className="font-mono">{branch}</TailText>),
    );
  });

  it("renders nothing without a branch", () => {
    expect(renderToStaticMarkup(<BranchLabel branch="" />)).toBe("");
  });
});
