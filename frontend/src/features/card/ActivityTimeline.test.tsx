import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { QuestionEntry } from "../../model/activity";
import { ActivityTimeline } from "./ActivityTimeline";

function question(overrides: Partial<QuestionEntry> = {}): QuestionEntry {
  return {
    kind: "question",
    key: "question:q1",
    day: "2026-10-12",
    time: "2026-10-12T12:00:00Z",
    by: "claude:aaaa1111",
    question: "Ship it?",
    answer: "",
    answeredBy: "",
    answeredAt: "",
    state: "open",
    ...overrides,
  };
}

const render = (entry: QuestionEntry) =>
  renderToStaticMarkup(
    <ActivityTimeline
      entries={[entry]}
      context={{ project: "flashheart", ticket: "FH-1" }}
      keys={new Set()}
    />,
  );

// What became of a question its history keeps (FH-190) is said beneath it.
describe("ActivityTimeline questions", () => {
  it("says what became of each question", () => {
    expect(render(question({ state: "open" }))).toContain(
      "Waiting for your answer, above.",
    );
    expect(render(question({ state: "in-session" }))).toContain(
      "Answered in the session&#x27;s own chat.",
    );
    expect(render(question({ state: "ended" }))).toContain(
      "Not answered before its session ended.",
    );
    expect(render(question({ state: "unanswered" }))).toContain(
      "No answer was recorded.",
    );
    const answered = render(
      question({ state: "answered", answer: "Yes", answeredBy: "Robert" }),
    );
    expect(answered).toContain("Robert");
    expect(answered).toContain("Yes");
  });
});
