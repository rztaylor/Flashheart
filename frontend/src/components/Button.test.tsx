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
