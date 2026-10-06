import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BlockerPill, StatusPill, Tag } from "./Pill";

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
  it("pairs the diamond with the reason in words", () => {
    const markup = renderToStaticMarkup(
      <BlockerPill>Depends on FH-4</BlockerPill>,
    );
    expect(markup).toContain('data-tone="blocked"');
    expect(markup).toContain("Blocked: </span>Depends on FH-4");
  });
});

describe("Tag", () => {
  it("fills a painted tag with its paint and names the value", () => {
    const markup = renderToStaticMarkup(
      <Tag paint={{ token: "type-bug", label: "bug" }}>bug</Tag>,
    );
    expect(markup).toContain("--paint:var(--fh-paint-type-bug)");
    expect(markup).toContain(">bug</span>");
  });

  it("draws an unpainted tag as a neutral outlined chip", () => {
    const markup = renderToStaticMarkup(<Tag>feature</Tag>);
    expect(markup).not.toContain("--paint");
    expect(markup).toContain("border-rule");
  });
});
