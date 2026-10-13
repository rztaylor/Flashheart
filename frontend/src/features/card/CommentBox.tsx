import {
  type FormEvent,
  type KeyboardEvent,
  type ReactNode,
  useId,
  useState,
} from "react";

import type { NewComment } from "../../api/edit";
import { Button } from "../../components/Button";
import { SelectField, TextArea } from "../../components/Field";
import { Icon } from "../../components/Icon";
import type { QuoteGroup } from "../../model/comments";
import { messageOf } from "../../model/errors";
import { sendKeys, useHeldDraft } from "../editing/InPlace";

// QuoteFigure is the part of the ticket a comment quotes, named by where
// it is from: on a well, with a full rule border (no side stripe).
export function QuoteFigure({
  from,
  children,
}: {
  from: string;
  children: ReactNode;
}) {
  return (
    <figure
      aria-label={`Quoting ${from}`}
      className="mb-2 min-w-0 rounded-card border border-rule bg-well px-3 py-2"
    >
      <figcaption className="text-xs font-semibold text-ink-muted">
        {from}
      </figcaption>
      <blockquote className="mt-1 text-sm text-ink">{children}</blockquote>
    </figure>
  );
}

// CommentBox ends a ticket page's Activity (FH-125): a person writes a
// comment, optionally quoting a section, criterion or How to verify step
// chosen from the page, and posts it with Comment or Ctrl or Cmd and
// Enter. An empty comment cannot be posted; a failed post keeps the text
// with the reason beside it. While it holds text, leaving the page asks
// first (the page's in-place editing holds it).
export function CommentBox({
  choices,
  onPost,
  renderQuote,
}: {
  choices: QuoteGroup[];
  onPost(comment: NewComment): Promise<void>;
  // renderQuote shows a quote's text as the page shows the ticket's.
  renderQuote(text: string): ReactNode;
}) {
  const id = useId();
  const [text, setText] = useState("");
  const [quoteKey, setQuoteKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const quote = choices
    .flatMap((group) => group.choices)
    .find((choice) => choice.key === quoteKey);
  const empty = !text.trim();
  useHeldDraft("comment-box", "comment", !empty, () => {
    setText("");
    setQuoteKey("");
    setError("");
  });

  const post = async () => {
    if (busy || empty) return;
    setBusy(true);
    setError("");
    try {
      await onPost({
        body: text,
        quote: quote?.text ?? "",
        quoteFrom: quote?.from ?? "",
      });
      setText("");
      setQuoteKey("");
    } catch (failure) {
      setError(messageOf(failure, "Not posted"));
    } finally {
      setBusy(false);
    }
  };
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void post();
    }
  };
  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    void post();
  };

  return (
    // Ctrl or Cmd and Enter posts from anywhere in the box.
    <form
      aria-label="New comment"
      onSubmit={onSubmit}
      onKeyDown={onKeyDown}
      className="mt-5 grid grid-cols-[1.75rem_minmax(0,1fr)] gap-3"
    >
      <span
        aria-hidden="true"
        className="grid size-7 place-items-center rounded-full border border-select/30 bg-select-surface text-ink"
      >
        <Icon name="comment" size={13} />
      </span>
      <div className="flex min-w-0 flex-col gap-2">
        {quote ? (
          <QuoteFigure from={quote.from}>{renderQuote(quote.text)}</QuoteFigure>
        ) : null}
        <TextArea
          aria-label="Add a comment"
          value={text}
          onChange={setText}
          placeholder="Add a comment…"
          rows={Math.min(12, Math.max(3, text.split("\n").length + 1))}
          aria-describedby={error ? `${id}-error` : `${id}-keys`}
        />
        {error ? (
          <p id={`${id}-error`} role="alert" className="text-sm text-danger">
            {error}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-2">
          {/* While posting the button stays focusable (aria-disabled), so
              focus is not lost when the post fails. */}
          <Button
            type="submit"
            variant="primary"
            disabled={empty && !busy}
            aria-disabled={busy || undefined}
            className="h-8 py-0 text-xs aria-disabled:cursor-wait aria-disabled:opacity-50"
          >
            {busy ? "Posting…" : "Comment"}
          </Button>
          {choices.length > 0 ? (
            <SelectField
              label="Quote"
              plain
              value={quoteKey}
              onChange={setQuoteKey}
              className="w-64 max-w-full min-w-0"
            >
              <option value="">No quote</option>
              {choices.map((group) => (
                <optgroup key={group.label} label={group.label}>
                  {group.choices.map((choice) => (
                    <option key={choice.key} value={choice.key}>
                      {choice.label}
                    </option>
                  ))}
                </optgroup>
              ))}
            </SelectField>
          ) : null}
          <span id={`${id}-keys`} className="text-xs text-ink-muted">
            {sendKeys} posts
          </span>
        </div>
      </div>
    </form>
  );
}
