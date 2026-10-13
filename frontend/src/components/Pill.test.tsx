import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BlockerPill, Pill, StatusPill, Tag } from "./Pill";

describe("StatusPill", () => {
  it("says the column's word beside its colour and icon", () => {
    const markup = renderToStaticMarkup(<StatusPill column="done" />);
    expect(markup).toContain("Done");
    expect(markup).toContain('data-tone="done"');
    expect(markup).toContain("<svg");
  });

  it("keeps backlog neutral", () => {
    expect(renderToStaticMarkup(<StatusPill column="backlog" />)).toContain(
      'data-tone="neutral"',
    );
  });
});

describe("BlockerPill", () => {
  it("pairs the no-entry sign with the reason in words", () => {
    const markup = renderToStaticMarkup(
      <BlockerPill>Depends on FH-4</BlockerPill>,
    );
    expect(markup).toContain('data-tone="blocked"');
    expect(markup).toContain('data-icon="blocked"');
    expect(markup).toContain("Blocked: </span>Depends on FH-4");
  });

  it("gives the blocked tone the no-entry sign on any pill", () => {
    const markup = renderToStaticMarkup(<Pill tone="blocked">Blocked</Pill>);
    expect(markup).toContain('data-icon="blocked"');
    expect(markup).toContain("Blocked");
  });
});

describe("Tag", () => {
  it("draws a tag as a neutral outlined chip", () => {
    const markup = renderToStaticMarkup(<Tag>feature</Tag>);
    expect(markup).not.toContain("--paint");
    expect(markup).toContain("border-rule");
  });
});
