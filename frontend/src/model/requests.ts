// Change requests in review (REV-6, D47, FH-126): a person's comments
// marked change requested, which an agent marks addressed and the person
// resolves or reopens. The page lists those still open or addressed, the
// card counts them, and the Request changes dialog offers the comments it
// can mark. Pure: what the server sent, in words.
import type { ChangeRequests, Comment } from "../api/board";
import type { StateTone } from "./status";

export type RequestState = "change-requested" | "addressed" | "resolved";

// requestStates are the states a change request shows, each a state pill
// with its word: open work in progress's amber, addressed waiting
// for review's blue, resolved done's green.
export const requestStates: Record<
  RequestState,
  { label: string; tone: StateTone }
> = {
  "change-requested": { label: "Change requested", tone: "progress" },
  addressed: { label: "Addressed", tone: "review" },
  resolved: { label: "Resolved", tone: "done" },
};

// requestStateOf is a comment's state as a change request, or undefined
// for a plain comment (open, or a state this version does not know).
export function requestStateOf(state: string): RequestState | undefined {
  return Object.hasOwn(requestStates, state)
    ? (state as RequestState)
    : undefined;
}

// requestsOf lists a ticket's change requests: open ones (waiting for the
// agent) first, then addressed ones (waiting for the person), then
// resolved ones, each oldest first. Only people's comments are requests.
export function requestsOf(comments: Comment[]): Comment[] {
  const mine = comments.filter((comment) => comment.byPerson);
  return (["change-requested", "addressed", "resolved"] as const).flatMap(
    (state) => mine.filter((comment) => comment.state === state),
  );
}

// countRequests counts the requests still waiting on someone: open and
// addressed.
export function countRequests(comments: Comment[]): ChangeRequests {
  const list = requestsOf(comments);
  const count = (state: string) =>
    list.filter((comment) => comment.state === state).length;
  return { open: count("change-requested"), addressed: count("addressed") };
}

const plural = (n: number, one: string, many: string) =>
  `${n} ${n === 1 ? one : many}`;

// requestWords says a count in words, for a card's chip and the page's
// heading: "2 changes requested", "1 open · 2 addressed", "1 addressed".
// "" when nothing waits.
export function requestWords(count: ChangeRequests | undefined): string {
  if (!count || count.open + count.addressed === 0) return "";
  if (count.addressed === 0)
    return plural(count.open, "change requested", "changes requested");
  if (count.open === 0) return `${count.addressed} addressed`;
  return `${count.open} open · ${count.addressed} addressed`;
}

// excerpt is a comment's text on one line, cut to limit characters.
export function excerpt(text: string, limit = 90): string {
  const line = text.replace(/\s+/g, " ").trim();
  return line.length > limit ? `${line.slice(0, limit - 1).trimEnd()}…` : line;
}

// requestChoices are the comments the Request changes dialog offers:
// every person's comment (marking an addressed or resolved one reopens
// it); open requests come checked, since they go back with the ticket.
// Newest first, as the person last wrote them.
export function requestChoices(
  comments: Comment[],
): { comment: Comment; checked: boolean }[] {
  return comments
    .filter((comment) => comment.byPerson)
    .map((comment) => ({
      comment,
      checked: comment.state === "change-requested",
    }))
    .reverse();
}

// sentWords is the toast after a request for changes, saying where the
// requests went: to the live session holding the ticket, or to whichever
// session picks it up next. sentTo is that session's short id, as the
// server gives it (agent-protocol §2).
export function sentWords(
  ticket: string,
  count: number,
  sentTo: string,
  verb = "Requested",
): string {
  const what =
    verb === "Requested"
      ? `Requested ${plural(count, "change", "changes")} on ${ticket}`
      : `${verb} the change request on ${ticket}`;
  return sentTo
    ? `${what}; sent to ${sentTo}.`
    : `${what}; the next session on it will see ${count === 1 ? "it" : "them"}.`;
}
