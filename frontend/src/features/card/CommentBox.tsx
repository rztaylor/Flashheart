import {
  type FormEvent,
  type KeyboardEvent,
  type ReactNode,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";

import type { TellSession } from "../../api/board";
import type { NewComment } from "../../api/edit";
import { Button } from "../../components/Button";
import { SelectField, TextArea } from "../../components/Field";
import { Icon } from "../../components/Icon";
import type { QuoteGroup } from "../../model/comments";
import { messageOf } from "../../model/errors";
import { noSessionReason, tellGroups, tellLabel } from "../../model/tell";
import { guardUnloading } from "../../state/leaveGuard";
import { sendKeys, useHeldDraft, useLateDraft } from "../editing/InPlace";
import { notSaved } from "../editing/lateSaves";

// Left is a comment whose post failed after the box was left (FH-156), as
// it was, for Back to the edit to put back.
interface Left {
  text: string;
  quoteKey: string;
  telling: boolean;
  target: string;
  error: string;
}

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

// Tell is the comment box's Tell a session now option (FH-127): the runs
// the server offers for ticket, live sessions that have claimed it with
// their subagents; none disables the option, saying why.
export interface Tell {
  ticket: string;
  sessions: TellSession[];
}

// CommentBox ends a ticket page's Activity (FH-125): a person writes a
// comment, optionally quoting a section, criterion or How to verify step
// chosen from the page, and posts it with Comment or Ctrl or Cmd and
// Enter. With tell, the comment can also be sent to a live session that
// has claimed the ticket, chosen per comment (FH-127). An empty comment
// cannot be posted; a failed post keeps the text with the reason beside
// it. While it holds text, leaving the page asks first (the page's
// in-place editing holds it), except while it is posting (FH-156): a post
// that fails after the page or panel was left is held with a toast, and
// Back to the edit puts the comment back.
export function CommentBox({
  ticket,
  choices,
  tell,
  onPost,
  renderQuote,
}: {
  // ticket is the ticket commented on, for a late failure's words.
  ticket: string;
  choices: QuoteGroup[];
  tell?: Tell;
  onPost(comment: NewComment): Promise<void>;
  // renderQuote shows a quote's text as the page shows the ticket's.
  renderQuote(text: string): ReactNode;
}) {
  const id = useId();
  const [text, setText] = useState("");
  const [quoteKey, setQuoteKey] = useState("");
  const [telling, setTelling] = useState(false);
  const [target, setTarget] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const quote = choices
    .flatMap((group) => group.choices)
    .find((choice) => choice.key === quoteKey);
  const sessions = tell?.sessions ?? [];
  // The run chosen, or the holder while the choice is no longer offered.
  const chosen =
    sessions.find((session) => session.run === target) ?? sessions[0];
  const sending = telling && chosen !== undefined;
  const empty = !text.trim();
  useHeldDraft("comment-box", "comment", !empty && !busy, () => {
    setText("");
    setQuoteKey("");
    setTelling(false);
    setError("");
  });
  const holdLate = useLateDraft<Left>("comment-box", "comment", (left) => {
    setText(left.text);
    setQuoteKey(left.quoteKey);
    setTelling(left.telling);
    setTarget(left.target);
    setError(left.error);
  });
  const mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const post = async () => {
    if (busy || empty) return;
    setBusy(true);
    setError("");
    // Closing or reloading the tab while posting asks (state/leaveGuard).
    const saving = guardUnloading();
    try {
      await onPost({
        body: text,
        quote: quote?.text ?? "",
        quoteFrom: quote?.from ?? "",
        ...(sending ? { sendTo: chosen.run } : {}),
      });
      setText("");
      setQuoteKey("");
      setTelling(false);
    } catch (failure) {
      const error = messageOf(failure, "Not posted");
      if (mounted.current) setError(error);
      else
        holdLate(
          notSaved(`Your comment on ${ticket} was not posted`, ticket, {
            error,
          }),
          { text, quoteKey, telling, target, error },
        );
    } finally {
      saving();
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
        {tell ? (
          <TellChoice
            id={id}
            tell={tell}
            telling={sending}
            onTelling={setTelling}
            chosen={chosen?.run ?? ""}
            onChoose={setTarget}
          />
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
            {sending
              ? busy
                ? "Sending…"
                : "Comment and tell"
              : busy
                ? "Posting…"
                : "Comment"}
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

// TellChoice is the Tell a session now option (FH-127): a checkbox and,
// once checked, the picker of the runs offered, each session with its
// subagents under it. With none it is disabled and says why.
function TellChoice({
  id,
  tell,
  telling,
  onTelling,
  chosen,
  onChoose,
}: {
  id: string;
  tell: Tell;
  telling: boolean;
  onTelling(telling: boolean): void;
  chosen: string;
  onChoose(run: string): void;
}) {
  const none = tell.sessions.length === 0;
  const reason = `${id}-tell-reason`;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <label
        className={`flex items-center gap-1.5 text-sm ${
          none ? "cursor-not-allowed text-ink-muted" : "cursor-pointer text-ink"
        }`}
      >
        <input
          type="checkbox"
          checked={telling}
          disabled={none}
          aria-describedby={reason}
          onChange={(event) => onTelling(event.target.checked)}
          className="size-4 accent-[var(--fh-select)] disabled:cursor-not-allowed"
        />
        Tell a session now
      </label>
      {telling ? (
        <SelectField
          label="Session"
          plain
          value={chosen}
          onChange={onChoose}
          className="w-72 max-w-full min-w-0"
        >
          {tellGroups(tell.sessions).map((group) => (
            <optgroup key={group.label} label={group.label}>
              {group.session ? (
                <option value={group.session.run}>
                  {tellLabel(group.session)}
                </option>
              ) : null}
              {group.subagents.map((run) => (
                <option key={run.run} value={run.run}>
                  {tellLabel(run)}
                </option>
              ))}
            </optgroup>
          ))}
        </SelectField>
      ) : null}
      <span id={reason} className="text-xs text-ink-muted">
        {none
          ? noSessionReason(tell.ticket)
          : telling
            ? "It gets your words with its next prompt or tool call."
            : `Also hands it to the session working on ${tell.ticket}.`}
      </span>
    </div>
  );
}
