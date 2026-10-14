import type { ReactNode } from "react";
import type { Comment, CommentDelivery } from "../../api/board";
import type { Saved } from "../../api/edit";
import { TextArea } from "../../components/Field";
import { Icon, type IconName } from "../../components/Icon";
import { Markdown } from "../../components/Markdown";
import { LinkedText } from "../../components/TicketLink";
import {
  type ActivityEntry,
  type ActivityFilter,
  andList,
  type CommentEntry,
} from "../../model/activity";
import { commentInFile } from "../../model/comments";
import type { LinkContext } from "../../model/markdown";
import { deliveryWords } from "../../model/tell";
import {
  EditPencil,
  editGroup,
  InPlaceEditor,
  useInPlaceItem,
} from "../editing/InPlace";
import { RequestPill } from "./ChangeRequests";
import { QuoteFigure } from "./CommentBox";
import { CommentDelete } from "./CommentDelete";

// when is an entry's date, and its time when it has one, where the
// browser is.
function when(entry: ActivityEntry): { text: string; dateTime: string } {
  const time = entry.kind === "note" ? "" : entry.time;
  if (time) {
    const date = new Date(time);
    if (!Number.isNaN(date.getTime()))
      return {
        dateTime: time,
        text: date.toLocaleString([], {
          day: "numeric",
          month: "short",
          hour: "2-digit",
          minute: "2-digit",
        }),
      };
  }
  const [year, month, day] = entry.day.split("-").map(Number);
  if (!year || !month || !day) return { text: "", dateTime: "" };
  return {
    dateTime: entry.day,
    text: new Date(year, month - 1, day).toLocaleDateString([], {
      day: "numeric",
      month: "short",
      year: "numeric",
    }),
  };
}

// A comment is a person's words, on the selection surface as an answer
// is; history sits back on the well.
const nodeTone: Record<ActivityEntry["kind"], string> = {
  note: "border-rule bg-card text-ink-muted",
  question:
    "border-callout-question-edge/30 bg-callout-question text-callout-question-edge",
  handoff:
    "border-callout-handoff-edge/30 bg-callout-handoff text-callout-handoff-edge",
  comment: "border-select/30 bg-select-surface text-ink",
  history: "border-rule bg-well text-ink-muted",
};

const nodeIcon: Record<ActivityEntry["kind"], IconName> = {
  note: "edit",
  question: "warning",
  handoff: "next",
  comment: "comment",
  history: "history",
};

const emptyWords: Record<ActivityFilter, string> = {
  all: "No comments, notes, history, handoff updates or questions yet.",
  comments: "No comments yet.",
  history: "No history yet.",
};

// CommentEditing edits people's comments in place (FH-125): detail is the
// ticket's activity, hashed by the activity file's hash; send saves a
// comment's text on base, and remove, where the page can, deletes a
// comment on base (FH-140).
export interface CommentEditing {
  detail: { id: string; hash: string; comments: Comment[] };
  send(comment: string, base: string, body: string): Promise<Saved>;
  onSaved(saved: Saved | undefined, what: string): void;
  remove?(comment: string, base: string): Promise<void>;
}

// ActivityTimeline is the ticket's activity, oldest first, on one rule
// (FH-65, FH-125): people's comments (each editable in place where the page
// can save), notes, history, agents' questions with their answers and
// handoff updates, as the filter shows them.
export function ActivityTimeline({
  entries,
  filter = "all",
  context,
  keys,
  editing,
}: {
  entries: ActivityEntry[];
  filter?: ActivityFilter;
  context: LinkContext;
  keys: Set<string>;
  editing?: CommentEditing;
}) {
  if (entries.length === 0)
    return <p className="text-sm text-ink-muted">{emptyWords[filter]}</p>;
  return (
    <ol className="relative flex flex-col gap-5 before:absolute before:top-3 before:bottom-3 before:left-[13px] before:w-0.5 before:bg-rule">
      {entries.map((entry) =>
        entry.kind === "comment" ? (
          <CommentItem
            key={entry.key}
            entry={entry}
            context={context}
            keys={keys}
            editing={entry.byPerson ? editing : undefined}
          />
        ) : (
          <Item key={entry.key} entry={entry}>
            <Head entry={entry} />
            <Body entry={entry} context={context} keys={keys} />
          </Item>
        ),
      )}
    </ol>
  );
}

function Item({
  entry,
  className = "",
  children,
}: {
  entry: ActivityEntry;
  className?: string;
  children: ReactNode;
}) {
  return (
    <li
      className={`relative grid grid-cols-[1.75rem_minmax(0,1fr)] gap-3 ${className}`}
    >
      <span
        aria-hidden="true"
        className={`grid size-7 place-items-center rounded-full border ${nodeTone[entry.kind]}`}
      >
        <Icon name={nodeIcon[entry.kind]} size={13} />
      </span>
      <div className="min-w-0">{children}</div>
    </li>
  );
}

// CommentItem is a person's comment: who, when and whether it was edited,
// its quote and its text, and, sent to a session, its delivery (FH-127);
// where the page can save, a pencil opens its text in place, until it is
// delivered (an edit already open stays, and its save is refused), and a
// Delete control beside it deletes it, or says why it cannot (FH-140).
function CommentItem({
  entry,
  context,
  keys,
  editing,
}: {
  entry: CommentEntry;
  context: LinkContext;
  keys: Set<string>;
  editing?: CommentEditing;
}) {
  const item = useInPlaceItem(`comment-${entry.id}`, "comment");
  const locked = entry.delivery === "delivered" && !item.open;
  const edit =
    editing?.detail.hash && item.editable && !locked ? editing : undefined;
  const remove =
    editing?.remove && editing.detail.hash && item.editable && !item.open
      ? editing.remove
      : undefined;
  return (
    <Item entry={entry} className={edit || remove ? editGroup : ""}>
      <Head
        entry={entry}
        action={
          (edit && !item.open) || remove ? (
            <>
              {edit && !item.open ? (
                <EditPencil item={item} name="comment" />
              ) : null}
              {remove && editing ? (
                <CommentDelete
                  entry={entry}
                  base={editing.detail.hash}
                  remove={remove}
                />
              ) : null}
            </>
          ) : null
        }
      />
      {entry.quote ? (
        <QuoteFigure from={entry.quoteFrom || "the ticket"}>
          <Markdown context={context} keys={keys}>
            {entry.quote}
          </Markdown>
        </QuoteFigure>
      ) : null}
      {edit && item.open ? (
        <InPlaceEditor
          item={item}
          name="comment"
          detail={edit.detail}
          read={(activity) =>
            activity.comments.find((comment) => comment.id === entry.id)
              ?.body ?? ""
          }
          isDirty={(draft, base) => draft !== base}
          send={(base, value) => edit.send(entry.id, base, value)}
          mine={(value) => value}
          theirs={(file) => commentInFile(file, entry.id)}
          check={(value) =>
            value.trim() ? undefined : "A comment needs some text."
          }
          onSaved={edit.onSaved}
          unsaved={`Your edit to a comment on ${edit.detail.id} was not saved`}
        >
          {(value, set) => (
            <TextArea
              aria-label="Comment"
              value={value}
              onChange={set}
              rows={Math.min(12, Math.max(3, value.split("\n").length + 1))}
            />
          )}
        </InPlaceEditor>
      ) : (
        // biome-ignore lint/a11y/noStaticElementInteractions: a double-click is a shortcut; the pencil is the control.
        <div onDoubleClick={edit ? item.onDoubleClick : undefined}>
          <Markdown context={context} keys={keys}>
            {entry.markdown}
          </Markdown>
        </div>
      )}
      <DeliveryNote entry={entry} />
    </Item>
  );
}

const deliveryIcon: Record<CommentDelivery, IconName> = {
  waiting: "next",
  delivered: "check",
  missed: "warning",
};

// DeliveryNote says where a comment sent to a session stands (FH-127):
// waiting for the session, delivered to it at a time (and so locked), or
// missed because the session ended first.
function DeliveryNote({ entry }: { entry: CommentEntry }) {
  if (!entry.delivery) return null;
  const at =
    entry.delivery === "delivered" && entry.delivered
      ? when({ ...entry, time: entry.delivered })
      : undefined;
  return (
    <p className="mt-2 flex items-start gap-1.5 text-xs text-ink-muted">
      <Icon
        name={deliveryIcon[entry.delivery]}
        size={12}
        className="mt-0.5 shrink-0"
      />
      <span>
        {deliveryWords(entry.delivery, entry.sentTo)}
        {entry.delivery === "delivered" ? (
          <>
            {at?.text ? (
              <>
                {" at "}
                <time dateTime={at.dateTime}>{at.text}</time>
              </>
            ) : null}
            . It can no longer be edited.
          </>
        ) : null}
      </span>
    </p>
  );
}

function Head({ entry, action }: { entry: ActivityEntry; action?: ReactNode }) {
  const date = when(entry);
  const verb =
    entry.kind === "note"
      ? entry.by
        ? "added a note"
        : "Note"
      : entry.kind === "question"
        ? "asked you"
        : entry.kind === "comment"
          ? "commented"
          : entry.kind === "history"
            ? andList(entry.words)
            : "updated the handoff";
  return (
    <div className="flex min-h-7 items-center gap-2">
      <p className="flex flex-1 flex-wrap items-center gap-x-1.5 text-xs text-ink-muted">
        {entry.by ? (
          <span className="font-semibold text-ink">{entry.by}</span>
        ) : null}
        <span>{verb}</span>
        {date.text ? (
          <>
            <span aria-hidden="true">·</span>
            <time dateTime={date.dateTime}>{date.text}</time>
          </>
        ) : null}
        {entry.kind === "comment" && entry.edited ? (
          <>
            <span aria-hidden="true">·</span>
            <span title={when({ ...entry, time: entry.edited }).text}>
              edited
            </span>
          </>
        ) : null}
        {entry.kind === "comment" && entry.state ? (
          <RequestPill state={entry.state} />
        ) : null}
      </p>
      {action ? (
        <div className="-my-1.5 flex items-center">{action}</div>
      ) : null}
    </div>
  );
}

function Body({
  entry,
  context,
  keys,
}: {
  entry: ActivityEntry;
  context: LinkContext;
  keys: Set<string>;
}) {
  switch (entry.kind) {
    case "note":
      return (
        <div>
          <Markdown context={context} keys={keys}>
            {entry.markdown}
          </Markdown>
        </div>
      );
    case "handoff":
      return entry.next.length > 0 ? (
        <p className="text-base text-ink">
          <span className="text-ink-muted">Next: </span>
          {entry.next.map((step, index) => (
            <span key={step}>
              {index > 0 ? "; " : null}
              <LinkedText text={step} keys={keys} />
            </span>
          ))}
        </p>
      ) : null;
    case "question":
      return (
        <>
          <p className="text-base text-ink">
            <LinkedText text={entry.question} keys={keys} />
          </p>
          <Reply>
            {entry.state === "answered" ? (
              <>
                <span className="font-semibold text-ink">
                  {entry.answeredBy || "You"}
                </span>{" "}
                answered: <LinkedText text={entry.answer} keys={keys} />
              </>
            ) : entry.state === "in-session" ? (
              "Answered in the session's own chat."
            ) : (
              "Waiting for your answer, above."
            )}
          </Reply>
        </>
      );
    default:
      return null;
  }
}

function Reply({ children }: { children: ReactNode }) {
  return (
    <p className="mt-2 rounded-card border border-select/20 bg-select-surface px-3 py-2 text-sm text-ink">
      {children}
    </p>
  );
}
