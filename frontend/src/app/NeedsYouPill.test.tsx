import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NeedsYouPill } from "./NeedsYouPill";

const render = (count: number, on: boolean) =>
  renderToStaticMarkup(
    <NeedsYouPill count={count} on={on} onToggle={() => undefined} />,
  );

describe("NeedsYouPill", () => {
  it("is absent while nothing needs you and the filter is off", () => {
    expect(render(0, false)).toBe("");
  });

  it("is a toggle, off until pressed (FH-44)", () => {
    const markup = render(2, false);
    expect(markup).toContain('aria-pressed="false"');
    expect(markup).toContain("Show only tickets that need you");
    expect(markup).toMatch(/2(<!-- -->)?<span[^>]*> need you/);
  });

  it("stays while on, so the filter can be switched off", () => {
    const markup = render(0, true);
    expect(markup).toContain('aria-pressed="true"');
    expect(markup).toContain("Showing only tickets that need you");
  });
});
