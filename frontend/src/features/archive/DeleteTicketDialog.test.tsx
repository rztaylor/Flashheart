import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { DeletePlan } from "../../api/archive";
import { DeletePreview } from "./DeleteTicketDialog";

const plan: DeletePlan = {
  id: "AL-3",
  title: "Card panel",
  files: ["AL-3-card-panel.md", "review.md", "files/board.png"],
  tickets: [{ project: "beta", id: "BE-1", title: "Hello" }],
  workstreams: [{ slug: "board-ui", title: "Board UI" }],
  token: "t",
};

describe("delete preview", () => {
  it("lists the folder and every reference the delete removes", () => {
    const html = renderToStaticMarkup(<DeletePreview plan={plan} />);
    for (const text of [
      "AL-3-card-panel.md",
      "files/board.png",
      "BE-1",
      "Hello",
      "Board UI",
      "cannot be undone",
    ]) {
      expect(html).toContain(text);
    }
  });

  it("says when nothing else refers to the ticket", () => {
    const html = renderToStaticMarkup(
      <DeletePreview plan={{ ...plan, tickets: [], workstreams: [] }} />,
    );
    expect(html).toContain("No other ticket or workstream refers to AL-3");
  });
});
