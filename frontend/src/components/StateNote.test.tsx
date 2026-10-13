import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { StateNote } from "./StateNote";

describe("StateNote", () => {
  it("marks a blocker with the no-entry sign and says Blocked in words", () => {
    for (const compact of [false, true]) {
      const markup = renderToStaticMarkup(
        <StateNote kind="blocked" compact={compact}>
          Depends on FH-4
        </StateNote>,
      );
      expect(markup).toContain('data-icon="blocked"');
      expect(markup).toContain("Blocked: </span>Depends on FH-4");
    }
  });
});
