// Manual card order (EDIT-9). The server keeps each project's saved order in
// its tickets' `rank` fields and sends cards in that order; these helpers
// turn a drop, a keyboard step or an Undo into the one thing a placement
// needs: the id of the card the ticket should follow ("" for the top of its
// project's cards in that column). Positions are taken among the cards the
// user can see, and only among the ticket's own project, so filtered-out
// cards and other projects in All projects are never reordered.
import type { Card, Column } from "../api/board";

export type Step = "top" | "up" | "down" | "bottom";

const peersOf = (cards: Card[], card: Card) =>
  cards.filter(
    (item) => item.project === card.project && item.column === card.column,
  );

// afterForIndex is the card to follow for a drop at index in shown, a
// column's cards as displayed with the moving card left out.
export function afterForIndex(
  shown: Card[],
  card: Card,
  index: number,
): string {
  const others = shown.filter((item) => item.id !== card.id);
  for (let at = Math.min(index, others.length) - 1; at >= 0; at--) {
    const item = others[at];
    if (item?.project === card.project) return item.id;
  }
  return "";
}

// afterForStep is the card to follow to move one step among the shown cards
// of the ticket's column, or null when it is already there.
export function afterForStep(
  shown: Card[],
  card: Card,
  step: Step,
): string | null {
  const peers = peersOf(shown, card);
  const at = peers.findIndex((item) => item.id === card.id);
  const last = peers.length - 1;
  if (at < 0) return null;
  if (step === "top" || step === "up") {
    if (at === 0) return null;
    if (step === "top") return "";
    return peers[at - 2]?.id ?? "";
  }
  if (at === last) return null;
  return (step === "down" ? peers[at + 1] : peers[last])?.id ?? null;
}

// predecessor is the card before this one in its project's column, or "" at
// the top: where Undo puts it back.
export function predecessor(cards: Card[], card: Card): string {
  const peers = peersOf(cards, card);
  const at = peers.findIndex((item) => item.id === card.id);
  return at > 0 ? (peers[at - 1]?.id ?? "") : "";
}

// placeCard shows a placement at once, before the save returns. Without
// after, the card only changes column.
export function placeCard(
  cards: Card[],
  id: string,
  to: Column,
  after: string | undefined,
): Card[] {
  const moving = cards.find((item) => item.id === id);
  if (!moving) return cards;
  const placed = { ...moving, column: to };
  if (after === undefined)
    return cards.map((item) => (item.id === id ? placed : item));
  const rest = cards.filter((item) => item.id !== id);
  let at =
    after === ""
      ? rest.findIndex(
          (item) => item.project === placed.project && item.column === to,
        )
      : rest.findIndex((item) => item.id === after);
  if (at >= 0 && after !== "") at += 1;
  // With nothing to follow or precede in that column, it joins the end.
  rest.splice(at < 0 ? rest.length : at, 0, placed);
  return rest;
}

// A display sort rearranges what columns show without changing the saved
// manual order; null shows the manual order. While a sort is on, cards
// cannot be reordered within a column (a position would mean nothing), and
// a move between columns keeps the ticket's place in the manual order.
export type DisplaySort = {
  by: "priority" | "created" | "title";
  descending: boolean;
} | null;

const priorityRank: Record<string, number> = { high: 0, medium: 1, low: 2 };

export function canReorder(sort: DisplaySort): boolean {
  return sort === null;
}

export function displayOrder(cards: Card[], sort: DisplaySort): Card[] {
  if (!sort) return cards;
  const value = (card: Card): string | number =>
    sort.by === "priority"
      ? (priorityRank[card.priority] ?? 3)
      : sort.by === "created"
        ? card.created
        : card.title.toLowerCase();
  const direction = sort.descending ? -1 : 1;
  return [...cards].sort((a, b) => {
    const x = value(a);
    const y = value(b);
    return (x < y ? -1 : x > y ? 1 : 0) * direction;
  });
}
