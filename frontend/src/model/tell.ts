// Telling a session now (D47, FH-127): a comment can also be sent to a
// live session that has claimed the ticket, chosen in the comment box.
// Pure: the sessions the server offers grouped for the picker, and a sent
// comment's delivery in words.
import type { CommentDelivery, TellSession } from "../api/board";
import { shortRun } from "./requests";
import { STATE_LABEL } from "./runs";

// TellGroup is one session in the picker with its subagents under it;
// session is undefined for subagents whose session is not offered (they
// claimed the ticket themselves).
export interface TellGroup {
  label: string;
  session?: TellSession;
  subagents: TellSession[];
}

// tellGroups groups the offered runs by session, in the server's order.
export function tellGroups(sessions: TellSession[]): TellGroup[] {
  const groups: TellGroup[] = sessions
    .filter((run) => !run.parent)
    .map((session) => ({
      label: `Session ${session.short}`,
      session,
      subagents: sessions.filter((run) => run.parent === session.run),
    }));
  const placed = new Set(groups.flatMap((group) => group.subagents));
  const others = sessions.filter((run) => run.parent && !placed.has(run));
  if (others.length > 0) groups.push({ label: "Subagents", subagents: others });
  return groups;
}

// tellLabel names an offered run in the picker: a session by its short id,
// a subagent by its kind, each with its state.
export function tellLabel(run: TellSession): string {
  const state = STATE_LABEL[run.state] ?? run.state;
  if (!run.parent) return `${run.short} · ${state}`;
  const kind = run.agentType ? `${run.agentType} subagent` : "Subagent";
  return `${kind} · ${state}`;
}

// noSessionReason says why a comment cannot be sent now.
export function noSessionReason(ticket: string): string {
  return `No live session holds ${ticket} now.`;
}

// commentedWords is the toast after posting: where the comment went
// (sentTo the run it was sent to, "" for none).
export function commentedWords(ticket: string, sentTo: string): string {
  return sentTo
    ? `Commented on ${ticket} and sent it to ${shortRun(sentTo)}.`
    : `Commented on ${ticket}.`;
}

// deliveryWords is a sent comment's delivery in words, before its time
// (when delivered, the caller adds it): waiting for the session to take
// it, delivered, or missed because it ended first.
export function deliveryWords(delivery: CommentDelivery, run: string): string {
  switch (delivery) {
    case "waiting":
      return `Waiting for ${run} to take it with its next prompt or tool call.`;
    case "delivered":
      return `Delivered to ${run}`;
    case "missed":
      return `Not delivered: ${run} ended before taking it. It stays on the ticket.`;
  }
}
