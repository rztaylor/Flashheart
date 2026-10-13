import type { ReactNode } from "react";
import { Icon, type IconName } from "../../components/Icon";
import { Markdown } from "../../components/Markdown";
import { LinkedText } from "../../components/TicketLink";
import type { ActivityEntry } from "../../model/activity";
import type { LinkContext } from "../../model/markdown";

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

const nodeTone: Record<ActivityEntry["kind"], string> = {
  note: "border-rule bg-card text-ink-muted",
  question:
    "border-callout-question-edge/30 bg-callout-question text-callout-question-edge",
  handoff:
    "border-callout-handoff-edge/30 bg-callout-handoff text-callout-handoff-edge",
};

const nodeIcon: Record<ActivityEntry["kind"], IconName> = {
  note: "edit",
  question: "warning",
  handoff: "next",
};

// ActivityTimeline is the ticket's notes, agents' questions with their
// answers and handoff updates, oldest first, on one rule (FH-65). It is
// read-only: notes are written by agents, and by people through the
// ticket's text, until the human-notes spike (FH-67) decides more.
export function ActivityTimeline({
  entries,
  context,
  keys,
}: {
  entries: ActivityEntry[];
  context: LinkContext;
  keys: Set<string>;
}) {
  if (entries.length === 0)
    return (
      <p className="text-sm text-ink-muted">
        No notes, questions or handoff updates yet.
      </p>
    );
  return (
    <ol className="relative flex flex-col gap-5 before:absolute before:top-3 before:bottom-3 before:left-[13px] before:w-0.5 before:bg-rule">
      {entries.map((entry) => (
        <li
          key={entry.key}
          className="relative grid grid-cols-[1.75rem_minmax(0,1fr)] gap-3"
        >
          <span
            aria-hidden="true"
            className={`grid size-7 place-items-center rounded-full border ${nodeTone[entry.kind]}`}
          >
            <Icon name={nodeIcon[entry.kind]} size={13} />
          </span>
          <div className="min-w-0">
            <Head entry={entry} />
            <Body entry={entry} context={context} keys={keys} />
          </div>
        </li>
      ))}
    </ol>
  );
}

function Head({ entry }: { entry: ActivityEntry }) {
  const date = when(entry);
  const verb =
    entry.kind === "note"
      ? entry.by
        ? "added a note"
        : "Note"
      : entry.kind === "question"
        ? "asked you"
        : "updated the handoff";
  return (
    <p className="flex min-h-7 flex-wrap items-center gap-x-1.5 text-xs text-ink-muted">
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
    </p>
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
    default:
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
  }
}

function Reply({ children }: { children: ReactNode }) {
  return (
    <p className="mt-2 rounded-card border border-select/20 bg-select-surface px-3 py-2 text-sm text-ink">
      {children}
    </p>
  );
}
