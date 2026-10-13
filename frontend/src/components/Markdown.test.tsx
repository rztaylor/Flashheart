import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { Markdown } from "./Markdown";

const context = { project: "flashheart", ticket: "FH-18" };
const html = (markdown: string) =>
  renderToStaticMarkup(
    <Markdown context={context} keys={new Set(["FH"])}>
      {markdown}
    </Markdown>,
  );

describe("Markdown", () => {
  it("links the ticket's own files, opening them in a new tab (FH-18)", () => {
    expect(html("[spec.pdf](files/20261013T1432-spec.pdf)")).toContain(
      '<a href="/api/projects/flashheart/tickets/FH-18/files/20261013T1432-spec.pdf" target="_blank" rel="noopener noreferrer"',
    );
  });

  it("shows unsafe and unknown links as their words", () => {
    for (const link of [
      "[run](javascript:alert(1))",
      "[data](data:text/html,hi)",
      "[up](../../secret.txt)",
    ]) {
      const out = html(link);
      expect(out, link).not.toContain("<a");
      expect(out, link).not.toContain("javascript:");
    }
  });
});
