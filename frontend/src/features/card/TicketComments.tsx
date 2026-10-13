import type { TicketDetail } from "../../api/board";
import { Markdown } from "../../components/Markdown";
import type { ActivityEntry, ActivityFilter } from "../../model/activity";
import { quoteChoices } from "../../model/comments";
import { ActivityTimeline } from "./ActivityTimeline";
import { CommentBox } from "./CommentBox";
import type { TicketState } from "./useTicket";

// TicketComments is a ticket's activity timeline ending in the comment box
// (FH-125): the full page's Activity and the card panel's Comments tab
// (FH-128) both show it. Where the ticket can be saved, a person's comment
// edits in place on the activity file's hash and the box posts a new one;
// both need an in-place editing coordinator (editing/InPlace) above them.
export function TicketComments({
  detail,
  ticket,
  entries,
  filter,
  keys,
}: {
  detail: TicketDetail;
  ticket: TicketState;
  // entries are the activity shown, as filter chose them.
  entries: ActivityEntry[];
  filter: ActivityFilter;
  keys: Set<string>;
}) {
  const context = { project: detail.project, ticket: detail.id };
  return (
    <>
      <ActivityTimeline
        entries={entries}
        filter={filter}
        context={context}
        keys={keys}
        editing={
          ticket.editComment
            ? {
                detail: {
                  id: detail.id,
                  hash: detail.activityHash,
                  comments: detail.comments,
                },
                send: ticket.editComment,
                onSaved: (saved, what) =>
                  ticket.saved(
                    saved,
                    what === "Saved" ? "Saved your comment on" : what,
                  ),
              }
            : undefined
        }
      />
      {ticket.comment ? (
        <CommentBox
          choices={quoteChoices(detail)}
          tell={{ ticket: detail.id, sessions: detail.tellSessions ?? [] }}
          onPost={ticket.comment}
          renderQuote={(text) =>
            text.trim() ? (
              <Markdown context={context} keys={keys}>
                {text}
              </Markdown>
            ) : null
          }
        />
      ) : null}
    </>
  );
}
