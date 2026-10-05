import type { Card } from "../api/board";

export type StateFilter = "all" | "blocked" | "unblocked" | "repair";

export interface Filters {
  query: string;
  type: string;
  priority: string;
  workstream: string;
  state: StateFilter;
  hideLater: boolean;
}

export const NO_WORKSTREAM = "(none)";

export const emptyFilters: Filters = {
  query: "",
  type: "",
  priority: "",
  workstream: "",
  state: "all",
  hideLater: false,
};

export function isFiltered(filters: Filters): boolean {
  return (
    filters.query.trim() !== "" ||
    filters.type !== "" ||
    filters.priority !== "" ||
    filters.workstream !== "" ||
    filters.state !== "all" ||
    filters.hideLater
  );
}

function haystack(card: Card): string {
  return [
    card.id,
    card.title,
    card.slug,
    card.tags.join(" "),
    card.excerpt,
    card.searchText ?? "",
    card.project,
  ]
    .join("\n")
    .toLowerCase();
}

// applyFilters keeps cards matching every active filter (VIEW-7). Every
// search word must appear somewhere in the card.
export function applyFilters(cards: Card[], filters: Filters): Card[] {
  const words = filters.query.toLowerCase().split(/\s+/).filter(Boolean);
  return cards.filter((card) => {
    if (filters.type && card.type !== filters.type) return false;
    if (filters.priority && card.priority !== filters.priority) return false;
    if (filters.workstream === NO_WORKSTREAM && card.workstream !== "")
      return false;
    if (
      filters.workstream &&
      filters.workstream !== NO_WORKSTREAM &&
      card.workstream !== filters.workstream
    )
      return false;
    if (filters.state === "blocked" && !card.blocked) return false;
    if (filters.state === "unblocked" && card.blocked) return false;
    if (filters.state === "repair" && card.needsRepair.length === 0)
      return false;
    if (filters.hideLater && card.tags.includes("later-possibility"))
      return false;
    if (words.length > 0) {
      const text = haystack(card);
      return words.every((word) => text.includes(word));
    }
    return true;
  });
}

const priorityOrder = ["high", "medium", "low"];

export function filterOptions(cards: Card[]) {
  const types = new Set<string>();
  const priorities = new Set<string>();
  const workstreams = new Set<string>();
  for (const card of cards) {
    if (card.type) types.add(card.type);
    if (card.priority) priorities.add(card.priority);
    if (card.workstream) workstreams.add(card.workstream);
  }
  return {
    types: [...types].sort(),
    priorities: [...priorities].sort((a, b) => {
      const ia = priorityOrder.indexOf(a);
      const ib = priorityOrder.indexOf(b);
      return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib) || a.localeCompare(b);
    }),
    workstreams: [...workstreams].sort(),
  };
}
