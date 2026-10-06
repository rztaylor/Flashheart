import { type FormEvent, useEffect, useId, useRef, useState } from "react";
import type { Question } from "../api/runs";
import { questionReason } from "../model/runs";
import { absoluteTime, runningTime } from "../model/time";
import { Button } from "./Button";
import { TextArea } from "./Field";
import { RunStateLabel } from "./RunState";
import { StateNote } from "./StateNote";

// QuestionCard is an agent's question to the human (CARD-6): what it asks,
// who asked and when, and an answer box with the agent's options as
// choices. The question is the agent's words, shown as data. Once
// answered it says the answer is on its way to the session's next prompt.
// MAX_ANSWER matches the server's limit on an answer.
const MAX_ANSWER = 1000;

export function QuestionCard({
  question,
  asker,
  now,
  onAnswer,
  headingLevel = 4,
}: {
  question: Question;
  // asker is the asking run's short id.
  asker: string;
  now: Date;
  // onAnswer sends an answer and resolves to an error message, or
  // undefined once sent; absent when this board cannot record answers.
  onAnswer?(answer: string): Promise<string | undefined>;
  // headingLevel places the question's heading under the caller's.
  headingLevel?: 4 | 5;
}) {
  const id = useId();
  const [answer, setAnswer] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const sent = useRef(false);
  const status = useRef<HTMLParagraphElement>(null);
  const reason = questionReason(question);
  const answered = Boolean(question.answeredAt);
  const options = [...new Set(question.options ?? [])];
  const Heading = headingLevel === 4 ? "h4" : "h5";

  // The form goes once the answer is recorded; keep focus with the
  // question by moving it to the status that replaces the form.
  useEffect(() => {
    if (answered && sent.current) status.current?.focus();
  }, [answered]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!onAnswer || !answer.trim() || sending) return;
    setSending(true);
    setError("");
    const problem = await onAnswer(answer.trim());
    setSending(false);
    sent.current = !problem;
    if (problem) setError(problem);
  };

  return (
    <section
      aria-labelledby={`${id}-heading`}
      aria-describedby={`${id}-text`}
      className="flex flex-col gap-2.5 rounded-panel bg-well p-3"
      data-question={question.id}
    >
      <header className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        <RunStateLabel state="needs-you" />
        <Heading id={`${id}-heading`} className="font-semibold text-ink">
          {reason}
        </Heading>
        <span className="text-ink-muted">
          from {asker} ·{" "}
          <time dateTime={question.asked} title={absoluteTime(question.asked)}>
            {runningTime(question.asked, now)}
          </time>
        </span>
      </header>
      <p
        id={`${id}-text`}
        className="text-sm break-words whitespace-pre-wrap text-ink"
      >
        {question.text}
      </p>
      {answered ? (
        <p
          ref={status}
          role="status"
          tabIndex={-1}
          className="text-xs text-ink-muted"
        >
          Answered{" "}
          <span className="font-semibold text-ink">“{question.answer}”</span>.
          The session gets it with its next prompt.
        </p>
      ) : onAnswer ? (
        <form className="flex flex-col gap-2" onSubmit={submit}>
          {options.length > 0 ? (
            <fieldset className="flex flex-wrap gap-1.5">
              <legend className="sr-only">Choices</legend>
              {options.map((option) => (
                <Button
                  key={option}
                  aria-pressed={answer === option}
                  className={
                    answer === option ? "border-ink ring-1 ring-ink" : undefined
                  }
                  onClick={() => setAnswer(option)}
                  disabled={sending}
                >
                  {option}
                </Button>
              ))}
            </fieldset>
          ) : null}
          <label htmlFor={`${id}-answer`} className="sr-only">
            Your answer
          </label>
          <TextArea
            id={`${id}-answer`}
            rows={2}
            value={answer}
            onChange={setAnswer}
            maxLength={MAX_ANSWER}
            placeholder={
              options.length
                ? "Choose above or write your own answer"
                : "Your answer"
            }
          />
          <div className="flex items-center gap-2">
            <Button
              type="submit"
              variant="primary"
              disabled={!answer.trim() || sending}
            >
              {sending ? "Sending…" : "Send answer"}
            </Button>
            <span className="text-2xs text-ink-muted">
              It reaches the session with its next prompt.
            </span>
          </div>
          <div role="alert">
            {error ? <StateNote kind="warning">{error}</StateNote> : null}
          </div>
        </form>
      ) : (
        <p className="text-xs text-ink-muted">
          This board is read-only; answer in a running Flashheart.
        </p>
      )}
    </section>
  );
}
