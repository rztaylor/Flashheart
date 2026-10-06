// Marginal remarks (FH-22, PRODUCT.md voice): dry asides drawn from the
// user's reviewed collection in remarks.md, which is the catalogue: its
// numbers are the remarks' ids and its sections their categories. Each
// placement is a real product state with the ids that suit it; lines that
// suit no state yet are deferred rather than forced. Remarks are secondary
// copy only: never in errors, blockers, permissions or anything to act on,
// and never sent to agents.
import source from "./remarks.md?raw";

export interface Remark {
  id: number;
  category: string;
  text: string;
  // quote names the source of a direct quote.
  quote?: string;
}

const entry = /^(\d+)\.\s+(.+)$/;
const quoted = /^\*\*(.+?)\*\*\s+—\s+Direct quote;\s*(.+?)\.?$/;

function parse(markdown: string): Remark[] {
  const remarks: Remark[] = [];
  let category = "";
  for (const line of markdown.split("\n")) {
    if (line.startsWith("## ")) {
      category = line.slice(3).trim();
      continue;
    }
    const match = entry.exec(line.trim());
    if (!match || !category) continue;
    const [, id = "", body = ""] = match;
    const quote = quoted.exec(body);
    remarks.push(
      quote
        ? { id: Number(id), category, text: quote[1] ?? "", quote: quote[2] }
        : { id: Number(id), category, text: body },
    );
  }
  return remarks;
}

export const catalogue: Remark[] = parse(source);

export type Placement =
  | "brand"
  | "board-empty"
  | "search"
  | "review-empty"
  | "agents-none"
  | "needs-you-clear"
  | "workstreams-none"
  | "plan-empty"
  | "plan"
  | "handoff"
  | "handoff-missing"
  | "progress"
  | "completion"
  | "shutdown";

// PLACEMENTS maps each product state to the remarks that suit it, and the
// priority it has for the screen's one aside (the highest shows).
export const PLACEMENTS: Record<
  Placement,
  { ids: number[]; priority: number }
> = {
  // The wordmark's tooltip: an Easter egg, never visible text.
  brand: { ids: [1, 2, 3, 4, 5, 6, 7], priority: 0 },
  // Agents, when runs exist and none needs you.
  "needs-you-clear": { ids: [11, 13, 14, 17, 18, 19], priority: 10 },
  // An empty review column on the board.
  "review-empty": { ids: [71, 72], priority: 10 },
  // A board or project with no tickets at all.
  "board-empty": { ids: [21, 23, 24, 25, 27, 28, 30], priority: 20 },
  // No workstreams yet.
  "workstreams-none": { ids: [31], priority: 20 },
  // No agent runs yet.
  "agents-none": { ids: [41, 43, 44, 46, 47, 48, 49, 50], priority: 20 },
  // Filters or a search that match nothing.
  search: { ids: [82, 83, 85, 86, 87, 88], priority: 20 },
  // An open run with a plan, or with none.
  plan: { ids: [32, 34, 35, 36, 39, 40], priority: 30 },
  "plan-empty": { ids: [33], priority: 30 },
  // A ticket's current handoff with a next step; an in-progress ticket
  // with no handoff.
  handoff: { ids: [75, 77, 79, 80], priority: 30 },
  "handoff-missing": { ids: [73], priority: 30 },
  // The toast after a move to Done: progress now and then; completion
  // only when the move finishes a workstream.
  progress: { ids: [51, 52, 53, 55, 57, 58], priority: 40 },
  completion: {
    ids: [61, 62, 63, 64, 65, 66, 67, 68, 69, 70],
    priority: 40,
  },
  // The stopped screen after a successful Quit.
  shutdown: { ids: [92, 93, 95, 96, 99, 100], priority: 0 },
};

// DEFERRED lists reviewed remarks with no honest placement yet: 54 claims a
// plan's first task, and 60 a handoff someone else will read.
export const DEFERRED = [54, 60];

export function remarksFor(placement: Placement): Remark[] {
  const { ids } = PLACEMENTS[placement];
  return catalogue.filter((remark) => ids.includes(remark.id));
}

// pickRemark chooses from a pool, never the one shown last when there is
// any other.
export function pickRemark(
  pool: Remark[],
  last: number | undefined,
  random: () => number = Math.random,
): Remark {
  const choices =
    pool.length > 1 ? pool.filter((remark) => remark.id !== last) : pool;
  const index = Math.min(
    choices.length - 1,
    Math.floor(random() * choices.length),
  );
  return choices[index] ?? (pool[0] as Remark);
}

// lastShown remembers each placement's last remark for this page's life,
// so a remount does not repeat it.
const lastShown = new Map<Placement, number>();

// nextRemark picks a placement's remark and remembers it.
export function nextRemark(placement: Placement): Remark {
  const remark = pickRemark(remarksFor(placement), lastShown.get(placement));
  lastShown.set(placement, remark.id);
  return remark;
}

// winningAside is the registered aside that shows: the highest priority,
// the earliest registered among equals.
export function winningAside(
  registered: { key: string; priority: number }[],
): string | undefined {
  let best: { key: string; priority: number } | undefined;
  for (const item of registered) {
    if (!best || item.priority > best.priority) best = item;
  }
  return best?.key;
}

interface Ticketish {
  id: string;
  project: string;
  column: string;
  workstream: string;
}

// completesWorkstream is true when moving card to `to` finishes its
// workstream: it is the last of the workstream's tickets on the board not in
// Done. Tickets not on the board (archived, or beyond the done limit) are
// done already.
export function completesWorkstream(
  cards: Ticketish[],
  card: Ticketish,
  to: string,
): boolean {
  if (to !== "done" || card.column === "done" || !card.workstream) return false;
  return cards.every(
    (other) =>
      other.id === card.id ||
      other.project !== card.project ||
      other.workstream !== card.workstream ||
      other.column === "done",
  );
}

const progressGap = 10 * 60_000;

// progressDue spaces progress remarks at least ten minutes apart.
export function progressDue(last: number | undefined, now: number): boolean {
  return last === undefined || now - last >= progressGap;
}
