import { useEffect, useId, useRef, useState } from "react";

import { ApiError } from "../../api/client";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { Icon } from "../../components/Icon";
import type { CommentEntry } from "../../model/activity";
import { deleteNote, deleteRefusal } from "../../model/comments";
import { messageOf } from "../../model/errors";

// excerptLimit is how much of the comment the confirmation shows.
const excerptLimit = 160;

const excerpt = (text: string) => {
  const line = text.replace(/\s+/g, " ").trim();
  return line.length > excerptLimit
    ? `${line.slice(0, excerptLimit - 1).trimEnd()}…`
    : line;
};

// CommentDelete is a person's comment's Delete control (FH-140), beside its
// pencil: it asks before deleting, on base, the hash of the activity the
// comment was read from. A comment its session has read, or a change
// request not resolved, cannot be deleted: the control stays focusable,
// says why as its description, and shows the reason when pressed.
export function CommentDelete({
  entry,
  base,
  remove,
}: {
  entry: CommentEntry;
  base: string;
  remove(comment: string, base: string): Promise<void>;
}) {
  const [confirming, setConfirming] = useState(false);
  const [why, setWhy] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const reasonId = useId();
  const reason = deleteRefusal(entry);
  // Keep it takes focus once the dialog has opened (its effect runs
  // before this one), so Enter right away deletes nothing.
  const keep = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (confirming) keep.current?.focus();
  }, [confirming]);

  const close = () => {
    setConfirming(false);
    setError("");
  };
  const confirm = async () => {
    if (busy || reason) return;
    setBusy(true);
    setError("");
    try {
      await remove(entry.id, base);
      setConfirming(false);
    } catch (failure) {
      setError(
        failure instanceof ApiError && failure.code === "conflict"
          ? "The ticket's comments changed since you opened them. They are up to date now; delete it again if you still mean to."
          : messageOf(failure, "The comment was not deleted."),
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <span className="relative">
      <button
        type="button"
        aria-label="Delete comment"
        title={reason ?? "Delete comment"}
        aria-disabled={reason ? true : undefined}
        aria-describedby={reason ? reasonId : undefined}
        aria-expanded={reason ? why : undefined}
        onClick={() => (reason ? setWhy(!why) : setConfirming(true))}
        onBlur={() => setWhy(false)}
        onKeyDown={(event) => {
          if (event.key === "Escape" && why) {
            event.preventDefault();
            event.stopPropagation();
            setWhy(false);
          }
        }}
        className={`grid size-7 shrink-0 place-items-center rounded-control text-ink-muted opacity-0 transition-opacity group-focus-within/edit:opacity-100 group-hover/edit:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:opacity-100 ${
          reason
            ? "aria-disabled:cursor-not-allowed hover:bg-well"
            : "hover:bg-danger-surface hover:text-danger"
        }`}
      >
        <Icon name="trash" size={14} />
      </button>
      {reason ? (
        <span
          id={reasonId}
          hidden={!why}
          className="absolute top-full right-0 z-10 mt-1 w-64 rounded-card border border-rule bg-card px-3 py-2 text-xs text-ink shadow-card"
        >
          {reason}
        </span>
      ) : null}
      {confirming ? (
        <Dialog
          title={reason ? "This comment stays" : "Delete this comment?"}
          onClose={close}
          actions={
            reason ? (
              <Button onClick={close}>Close</Button>
            ) : (
              <>
                <Button ref={keep} onClick={close}>
                  Keep it
                </Button>
                <Button
                  variant="danger"
                  aria-disabled={busy || undefined}
                  onClick={confirm}
                  className="aria-disabled:cursor-wait aria-disabled:opacity-50"
                >
                  {busy ? "Deleting…" : "Delete comment"}
                </Button>
              </>
            )
          }
        >
          <blockquote className="rounded-card border border-rule bg-well px-3 py-2 text-ink-muted">
            {excerpt(entry.markdown)}
          </blockquote>
          <p className="mt-3">{reason ?? deleteNote(entry)}</p>
          {error ? (
            <p role="alert" className="mt-3 text-danger">
              {error}
            </p>
          ) : null}
        </Dialog>
      ) : null}
    </span>
  );
}
