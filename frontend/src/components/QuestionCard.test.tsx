import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Question } from "../api/runs";
import { QuestionCard } from "./QuestionCard";

const now = new Date("2026-10-06T10:05:00Z");
const question: Question = {
  id: "q-1",
  run: "claude:s",
  ticket: "FH-4",
  kind: "decision",
  text: "Keep <b>the</b> old schema?",
  options: ["Yes", "No"],
  asked: "2026-10-06T10:00:00Z",
};
const send = async () => undefined;

describe("QuestionCard", () => {
  it("shows the question as text with its choices and an answer box", () => {
    const markup = renderToStaticMarkup(
      <QuestionCard
        question={question}
        asker="claude:5b0c7e2a"
        now={now}
        onAnswer={send}
      />,
    );
    expect(markup).toContain("Needs a decision");
    expect(markup).toContain("from claude:5b0c7e2a");
    expect(markup).toContain("Keep &lt;b&gt;the&lt;/b&gt; old schema?");
    expect(markup).toContain('aria-pressed="false"');
    expect(markup).toContain(">Yes</button>");
    expect(markup).toContain(">Send answer</button>");
  });

  it("says an answer is on its way instead of asking again", () => {
    const markup = renderToStaticMarkup(
      <QuestionCard
        question={{
          ...question,
          answer: "No",
          answeredAt: "2026-10-06T10:04:00Z",
        }}
        asker="claude:5b0c7e2a"
        now={now}
        onAnswer={send}
      />,
    );
    expect(markup).toContain("Answer waits for its next prompt");
    expect(markup).toContain("“No”");
    expect(markup).not.toContain("Send answer");
  });

  it("says a run without hooks gets its answer with its next tool call", () => {
    const asked = { ...question, run: "codex:mcp-0a1b2c3d4e5f" };
    const open = renderToStaticMarkup(
      <QuestionCard
        question={asked}
        asker="codex:mcp-0a1b"
        now={now}
        onAnswer={send}
      />,
    );
    expect(open).toContain("with its next Flashheart tool call");
    expect(open).not.toContain("next prompt");
    const answered = renderToStaticMarkup(
      <QuestionCard
        question={{
          ...asked,
          answer: "No",
          answeredAt: "2026-10-06T10:04:00Z",
        }}
        asker="codex:mcp-0a1b"
        now={now}
        onAnswer={send}
      />,
    );
    expect(answered).toContain("Answer waits for its next tool call");
    expect(answered).toContain("with its next Flashheart tool call");
    expect(answered).not.toContain("next prompt");
  });

  it("explains a read-only board", () => {
    const markup = renderToStaticMarkup(
      <QuestionCard question={question} asker="claude:5b0c7e2a" now={now} />,
    );
    expect(markup).toContain("read-only");
    expect(markup).not.toContain("<form");
  });

  it("says a question from an ended session waits for it to resume", () => {
    const markup = renderToStaticMarkup(
      <QuestionCard
        question={{ ...question, sessionEnded: true }}
        asker="claude:5b0c7e2a"
        now={now}
        onAnswer={send}
      />,
    );
    expect(markup).toContain("This session has ended");
    expect(markup).toContain("when it resumes");
    expect(markup).toContain(">Send answer</button>");
    expect(markup).not.toContain("Needs you");
  });
});
