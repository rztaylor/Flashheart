import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Question } from "../api/runs";
import { QuestionCard } from "./QuestionCard";

const now = new Date("2026-10-06T10:05:00Z");
const question: Question = {
  id: "q-1",
  run: "claude:s",
  runShort: "claude:s",
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
    const asked = {
      ...question,
      run: "codex:mcp-0a1b2c3d4e5f",
      mcpOnly: true,
    };
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

  it("does not promise a resumption to an ended run without hooks", () => {
    const asked = {
      ...question,
      run: "codex:mcp-0a1b2c3d4e5f",
      mcpOnly: true,
      sessionEnded: true,
    };
    const open = renderToStaticMarkup(
      <QuestionCard
        question={asked}
        asker="codex:mcp-0a1b"
        now={now}
        onAnswer={send}
      />,
    );
    expect(open).toContain("This session has ended and will not resume");
    expect(open).not.toContain("when it resumes");
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
    expect(answered).toContain(
      "The session has ended; the answer stays on the board.",
    );
  });

  // A Codex session without hooks has a run named by its own session id
  // (FH-74): its answer goes with its next tool call, and an ended one can
  // be resumed.
  it("promises a resumption to a session without hooks named by its id", () => {
    const asked = {
      ...question,
      run: "codex:019a7c00-0000-7000-8000-000000000001",
      mcpOnly: true,
    };
    const open = renderToStaticMarkup(
      <QuestionCard
        question={asked}
        asker="codex:019a7c00"
        now={now}
        onAnswer={send}
      />,
    );
    expect(open).toContain("with its next Flashheart tool call");
    const ended = renderToStaticMarkup(
      <QuestionCard
        question={{ ...asked, sessionEnded: true }}
        asker="codex:019a7c00"
        now={now}
        onAnswer={send}
      />,
    );
    expect(ended).toContain("it gets your answer when it resumes");
    expect(ended).not.toContain("will not resume");
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
