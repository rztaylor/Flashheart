import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Icon, iconNames } from "./Icon";

// Blocked has one symbol everywhere (FH-148, ui-layout.md §7): the
// no-entry sign, a filled disc with its bar cut out, so it reads at card
// size and the bar shows the surface behind it.
describe("Icon", () => {
  it("draws blocked as a no-entry sign: a filled disc with a bar cut out", () => {
    const markup = renderToStaticMarkup(<Icon name="blocked" />);
    expect(markup).toContain('data-icon="blocked"');
    expect(markup).toContain('fill="currentColor"');
    expect(markup).toContain('fill-rule="evenodd"');
    expect(markup).not.toContain('stroke="currentColor"');
  });

  it("names every icon it draws, for tests and inspection", () => {
    expect(renderToStaticMarkup(<Icon name="check" />)).toContain(
      'data-icon="check"',
    );
  });

  it("has no diamond, so nothing can mark blocked with it", () => {
    expect(iconNames).not.toContain("diamond");
    expect(iconNames).toContain("blocked");
  });
});
