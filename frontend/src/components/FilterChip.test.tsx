import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { FilterChip, wantsExclude } from "./FilterChip";

const chip = (state: "included" | "excluded" | "idle", hidden = false) =>
  renderToStaticMarkup(
    <FilterChip
      name="bug"
      bullet={<i data-bullet />}
      state={state}
      hidden={hidden}
      onToggle={() => undefined}
    />,
  );

describe("FilterChip", () => {
  it("shows a bullet and the name, with no count", () => {
    expect(chip("idle")).toMatch(
      /<i data-bullet="true"><\/i><span[^>]*>bug<\/span><\/button>/,
    );
  });

  it("reports whether its value is shown only", () => {
    expect(chip("included")).toContain('aria-pressed="true"');
    expect(chip("idle")).toContain('aria-pressed="false"');
    expect(chip("excluded")).toContain('aria-pressed="false"');
  });

  it("names an excluded value as hidden and looks different", () => {
    expect(chip("excluded")).toContain(", hidden");
    expect(chip("idle")).not.toContain(", hidden");
    const looks = new Set(
      (["included", "excluded", "idle"] as const).map(
        (state) => chip(state).match(/class="([^"]*)"/)?.[1],
      ),
    );
    expect(looks.size).toBe(3);
  });

  it("hints at both clicks", () => {
    expect(chip("idle")).toMatch(
      /title="Show only bug\. (⌘|Ctrl)-click hides it\."/,
    );
    expect(chip("included")).toContain(
      'title="Showing only bug. Click to restore."',
    );
    expect(chip("excluded")).toContain('title="Hiding bug. Click to restore."');
  });

  it("takes a chip that does not fit out of view and the tab order", () => {
    const markup = chip("idle", true);
    expect(markup).toContain("invisible");
    expect(markup).toContain('tabindex="-1"');
    expect(markup).toContain('aria-hidden="true"');
  });
});

describe("wantsExclude", () => {
  it("is a click with Cmd or Ctrl held", () => {
    expect(wantsExclude({ metaKey: true, ctrlKey: false })).toBe(true);
    expect(wantsExclude({ metaKey: false, ctrlKey: true })).toBe(true);
    expect(wantsExclude({ metaKey: false, ctrlKey: false })).toBe(false);
  });
});
