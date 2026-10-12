import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { LoadError } from "./LoadError";
import { StaleNote } from "./StaleNote";

// The Error state of ui-layout.md §8, shared by every view: the error in
// words and Try again, or a note while the last good copy stays visible.
describe("LoadError", () => {
  it("says what failed and why, as an alert, with Try again", () => {
    const html = renderToStaticMarkup(
      <LoadError
        what="The archive could not be loaded"
        error="disk full"
        onRetry={vi.fn()}
      />,
    );
    expect(html).toMatch(
      /<p [^>]*role="alert"[^>]*>The archive could not be loaded: disk full<\/p>/,
    );
    expect(html).toMatch(/<button[^>]*>.*Try again<\/button>/);
    expect(html).toContain("text-danger");
  });

  it("ties Try again to the failure it retries", () => {
    const html = renderToStaticMarkup(
      <LoadError
        what="Workstreams could not be loaded"
        error="x"
        onRetry={vi.fn()}
      />,
    );
    const id = /<p id="([^"]+)"/.exec(html)?.[1];
    expect(id).toBeTruthy();
    expect(html).toContain(`aria-describedby="${id}"`);
  });

  it("shows the error alone when there is nothing to name", () => {
    const html = renderToStaticMarkup(
      <LoadError error="Request failed (500)" onRetry={vi.fn()} />,
    );
    expect(html).toMatch(/role="alert"[^>]*>Request failed \(500\)<\/p>/);
  });
});

describe("StaleNote", () => {
  it("says the last good copy is showing and why, politely", () => {
    const html = renderToStaticMarkup(<StaleNote error="disk full" />);
    expect(html).toMatch(
      /<p [^>]*role="status"[^>]*>Showing the last good copy: disk full<\/p>/,
    );
  });
});
