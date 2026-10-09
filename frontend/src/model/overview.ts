// The Overview's sections (VIEW-3, D32): what a project manager asks of the
// board, about tickets, with agent sessions as evidence. Pure grouping,
// order and wording; the view draws them.
import type { Card } from "../api/board";
import type { Question, Run, TicketSessions } from "../api/runs";
import { permissionReason, questionReason, reasonFor } from "./runs";

export type SectionId =
  | "decision"
  | "review"
  | "risk"
  | "progress"
  | "unticketed"
  | "upNext";

export interface Section {
  id: SectionId;
  title: string;
  // empty is the section's one calm sentence when it has nothing.
  empty: string;
}

export const SECTIONS: Section[] = [
  {
    id: "decision",
    title: "Needs your decision",
    empty: "Nothing needs your decision.",
  },
  {
    id: "review",
    title: "Ready for your review",
    empty: "Nothing is waiting for your review.",
  },
  { id: "risk", title: "At risk", empty: "Nothing is at risk." },
  { id: "progress", title: "In progress", empty: "Nothing is in progress." },
  {
    id: "unticketed",
    title: "Work with no ticket",
    empty: "Every live session has a ticket.",
  },
  { id: "upNext", title: "Up next", empty: "Up next is empty." },
];

// sectionOpen is whether a section starts open: the decision and review
// sections always, At risk and In progress when they have something, and
// the last two as collapsed tiles.
export function sectionOpen(id: SectionId, count: number): boolean {
  if (id === "decision" || id === "review") return true;
  if (id === "risk" || id === "progress") return count > 0;
  return false;
}

// collapsible: the sections that are not always open.
export const collapsible = (id: SectionId) =>
  id !== "decision" && id !== "review";

export interface DecisionItem {
  kind: "question" | "permission";
  // key is the question's id, or the run's for a permission prompt.
  key: string;
  run: Run;
  question?: Question;
  reason: string;
  // since is when it started waiting.
  since: string;
  project: string;
  branch: string;
  // ticket is absent when the run and question are about none.
  ticket?: { id: string; title: string };
}

export interface ReviewItem {
  card: Card;
  since: string;
}

export interface RiskItem {
  card: Card;
  reasons: string[];
  since: string;
  // sinceSession: since is a session's last activity; otherwise it is when
  // the ticket last changed.
  sinceSession: boolean;
}

export interface ProgressItem {
  card: Card;
  words: string;
  since: string;
}

export interface Overview {
  decision: DecisionItem[];
  review: ReviewItem[];
  risk: RiskItem[];
  progress: ProgressItem[];
  unticketed: Run[];
  upNext: Card[];
  // upNextTotal counts every up-next ticket, of which upNext is the top.
  upNextTotal: number;
}

// UP_NEXT_SHOWN is how many of each project's Up next the tile lists.
const UP_NEXT_SHOWN = 3;

const time = (iso: string) => Date.parse(iso) || 0;
const oldestFirst = <T extends { since: string }>(a: T, b: T) =>
  time(a.since) - time(b.since);
const newestFirst = <T extends { since: string }>(a: T, b: T) =>
  time(b.since) - time(a.since);

const plural = (count: number, one: string, many: string) =>
  count === 1 ? one : many;

// subagentWords says a ticket's subagents in words, leaving out zero parts:
// "subagents 3 done · 2 running · 1 needs you".
export function subagentWords(counts: TicketSessions["subagents"]): string {
  const parts = [
    counts.done ? `${counts.done} done` : "",
    counts.running ? `${counts.running} running` : "",
    counts.needsYou
      ? `${counts.needsYou} ${plural(counts.needsYou, "needs", "need")} you`
      : "",
  ].filter(Boolean);
  return parts.length ? `subagents ${parts.join(" · ")}` : "";
}

const STATE_WORDS: Record<TicketSessions["state"], string> = {
  "needs-you": "Needs you",
  working: "Working",
  quiet: "Quiet",
  waiting: "Waiting for a prompt",
  ended: "Session ended",
  "": "No session",
};

// sessionWords is an In progress row's line: the state of the session that
// speaks for the ticket, then its subagents.
export function sessionWords(sessions: TicketSessions): string {
  return [STATE_WORDS[sessions.state], subagentWords(sessions.subagents)]
    .filter(Boolean)
    .join(" · ");
}

// lastActivity is a ticket's latest session activity, or else when the
// ticket last changed.
const lastActivity = (card: Card) =>
  card.sessions?.lastActivity || card.updated || card.modified;

function blockerName(card: Card): string {
  const [first] = card.blockedBy;
  if (!first) return "";
  const name = first.ticket
    ? first.ticket.id
    : first.workstream
      ? `workstream ${first.workstream}`
      : first.text;
  const more = card.blockedBy.length - 1;
  return more > 0 ? `${name} and ${more} more` : name;
}

// riskReasons says why a ticket is at risk, in words, first reason
// capitalised: an in-progress ticket no live session works on or whose
// session has gone quiet, a ticket whose session ended without a handoff,
// or the top of its project's Up next held by a blocker. Empty when it is
// not at risk.
export function riskReasons(card: Card, topOfUpNext: boolean): string[] {
  if (card.column === "done") return [];
  const sessions = card.sessions;
  const reasons: string[] = [];
  if (card.column === "in-progress" && sessions) {
    if (sessions.noLiveSession)
      reasons.push(
        sessions.run ? "No live session" : "No session has worked on it",
      );
    else if (sessions.state === "quiet") reasons.push("Session gone quiet");
  }
  if (sessions?.noHandoff) reasons.push("Ended without a handoff");
  if (topOfUpNext && card.column === "up-next" && card.blocked)
    reasons.push("Top of Up next", `blocked by ${blockerName(card)}`);
  return reasons.map((reason, index) =>
    index === 0 ? reason : reason.charAt(0).toLowerCase() + reason.slice(1),
  );
}

// openForYou: a question that keeps its run in Needs you (RUN-3): open, or
// answered on the board and waiting for the session's next prompt, where
// its card says so. One answered in the session, or from an ended session,
// does not need you (RUN-8).
const openForYou = (question: Question) =>
  !question.delivered && !question.answeredInSession && !question.sessionEnded;

function decisions(runs: Run[], cards: Map<string, Card>): DecisionItem[] {
  const titled = (id: string, run: Run) =>
    id
      ? {
          id,
          title:
            cards.get(id)?.title ?? (id === run.ticket ? run.ticketTitle : ""),
        }
      : undefined;
  const items: DecisionItem[] = [];
  const seen = new Set<string>();
  for (const run of runs) {
    const questions = run.questions.filter(openForYou);
    for (const question of questions) {
      if (seen.has(question.id)) continue;
      seen.add(question.id);
      items.push({
        kind: "question",
        key: question.id,
        run,
        question,
        reason: questionReason(question),
        since: question.asked,
        project: run.project,
        branch: run.branch,
        ticket: titled(question.ticket || run.ticket, run),
      });
    }
    if (run.state !== "needs-you" || questions.length > 0) continue;
    // A session waiting on a subagent is answered through that subagent,
    // which is listed itself.
    const waitsOnChild = runs.some(
      (other) => other.parent === run.id && other.state === "needs-you",
    );
    if (waitsOnChild && !run.permission) continue;
    items.push({
      kind: "permission",
      key: run.id,
      run,
      reason:
        run.kind === "subagent"
          ? reasonFor(run.permission, run.agentType || "Subagent")
          : permissionReason(run.permission),
      since: run.lastActivity,
      project: run.project,
      branch: run.branch,
      ticket: titled(run.ticket, run),
    });
  }
  return items.sort(oldestFirst);
}

// overview groups a scope's tickets and runs into the Overview's sections.
// cards are in the board's order; runs come from the runs API.
export function overview(cards: Card[], runs: Run[]): Overview {
  const byID = new Map(cards.map((card) => [card.id, card]));
  const upNext = cards.filter((card) => card.column === "up-next");
  // The top of each project's Up next, and its first few.
  const tops = new Set<string>();
  const shown: Card[] = [];
  const perProject = new Map<string, number>();
  for (const card of upNext) {
    const count = perProject.get(card.project) ?? 0;
    if (count === 0) tops.add(card.id);
    if (count < UP_NEXT_SHOWN) shown.push(card);
    perProject.set(card.project, count + 1);
  }

  const risk: RiskItem[] = [];
  const atRisk = new Set<string>();
  for (const card of cards) {
    const reasons = riskReasons(card, tops.has(card.id));
    if (reasons.length === 0) continue;
    atRisk.add(card.id);
    risk.push({
      card,
      reasons,
      since: lastActivity(card),
      sinceSession: Boolean(card.sessions?.lastActivity),
    });
  }

  const runIDs = new Set(runs.map((run) => run.id));
  return {
    decision: decisions(runs, byID),
    review: cards
      .filter((card) => card.column === "review")
      .map((card) => ({ card, since: card.updated || card.modified }))
      .sort(oldestFirst),
    risk: risk.sort(oldestFirst),
    progress: cards
      .filter((card) => card.column === "in-progress" && !atRisk.has(card.id))
      .map((card) => ({
        card,
        words: card.sessions ? sessionWords(card.sessions) : "No session",
        since: lastActivity(card),
      }))
      .sort(newestFirst),
    unticketed: runs
      .filter(
        (run) =>
          !run.ticket &&
          run.state !== "ended" &&
          !(run.parent && runIDs.has(run.parent)),
      )
      .map((run) => ({ run, since: run.lastActivity }))
      .sort(newestFirst)
      .map((item) => item.run),
    upNext: shown,
    upNextTotal: upNext.length,
  };
}
