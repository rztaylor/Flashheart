import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Button } from "./Button";

describe("Button", () => {
  it("is a non-submitting button by default", () => {
    const markup = renderToStaticMarkup(<Button>Quit</Button>);
    expect(markup).toContain('type="button"');
    expect(markup).toContain(">Quit</button>");
  });

  it("applies distinct variant styles", () => {
    const primary = renderToStaticMarkup(<Button variant="primary">Go</Button>);
    const quiet = renderToStaticMarkup(<Button variant="quiet">Go</Button>);
    expect(primary).not.toEqual(quiet);
  });

  it("marks actions that cannot be undone in red", () => {
    const danger = renderToStaticMarkup(
      <Button variant="danger">Delete permanently</Button>,
    );
    const quiet = renderToStaticMarkup(
      <Button variant="danger-quiet">Delete permanently</Button>,
    );
    expect(danger).toContain("bg-danger");
    expect(danger).toContain("text-on-danger");
    expect(quiet).toContain("text-danger");
    expect(quiet).not.toMatch(/ bg-danger( |")/);
  });

  it("passes through accessible attributes", () => {
    const markup = renderToStaticMarkup(
      <Button aria-label="Check connection" disabled>
        •
      </Button>,
    );
    expect(markup).toContain('aria-label="Check connection"');
    expect(markup).toContain("disabled");
  });
});
