import type { Card, WorkstreamBrief } from "../api/board";
import { ageOf } from "./paint";

export type StateFilter = "all" | "blocked" | "unblocked" | "repair";

// Choice is one filter dimension: values shown only (include, any of them)
// and values hidden (exclude). Both empty means no filter.
export interface Choice {
  include: string[];
  exclude: string[];
}

export type ChoiceKey = "type" | "priority" | "workstream" | "age";

export interface Filters {
  query: string;
  type: Choice;
  priority: Choice;
  workstream: Choice;
  // age filters by the Colour by age buckets (today, week, older).
  age: Choice;
  state: StateFilter;
}

export const NO_WORKSTREAM = "(none)";

export const noChoice: Choice = { include: [], exclude: [] };

export const emptyFilters: Filters = {
  query: "",
  type: noChoice,
  priority: noChoice,
  workstream: noChoice,
  age: noChoice,
  state: "all",
};

export const CHOICE_KEYS: ChoiceKey[] = [
  "type",
  "priority",
  "workstream",
  "age",
];

export type ChoiceState = "included" | "excluded" | "idle";

export function choiceState(choice: Choice, value: string): ChoiceState {
  if (choice.include.includes(value)) return "included";
  if (choice.exclude.includes(value)) return "excluded";
  return "idle";
}

export function choiceCount(choice: Choice): number {
  return choice.include.length + choice.exclude.length;
}

// toggleChoice is a click on a filter chip or menu item: a click on a
// chosen value restores it; otherwise a plain click shows only it (with any
// other shown values) and a modified click (Cmd or Ctrl) hides it.
export function toggleChoice(
  choice: Choice,
  value: string,
  exclude: boolean,
): Choice {
  const without = (values: string[]) => values.filter((v) => v !== value);
  if (choiceState(choice, value) !== "idle")
    return {
      include: without(choice.include),
      exclude: without(choice.exclude),
    };
  return exclude
    ? { include: choice.include, exclude: [...choice.exclude, value] }
    : { include: [...choice.include, value], exclude: choice.exclude };
}

function matches(choice: Choice, value: string): boolean {
  if (choice.include.length > 0 && !choice.include.includes(value))
    return false;
  return !choice.exclude.includes(value);
}

export function isFiltered(filters: Filters): boolean {
  return (
    filters.query.trim() !== "" ||
    CHOICE_KEYS.some((key) => choiceCount(filters[key]) > 0) ||
    filters.state !== "all"
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

// applyFilters keeps cards matching every active filter (VIEW-7, FH-39).
// Every search word must appear somewhere in the card.
export function applyFilters(
  cards: Card[],
  filters: Filters,
  now = new Date(),
): Card[] {
  const words = filters.query.toLowerCase().split(/\s+/).filter(Boolean);
  return cards.filter((card) => {
    if (!matches(filters.type, card.type)) return false;
    if (!matches(filters.priority, card.priority)) return false;
    if (!matches(filters.workstream, card.workstream || NO_WORKSTREAM))
      return false;
    if (
      choiceCount(filters.age) > 0 &&
      !matches(filters.age, ageOf(card, now) ?? "")
    )
      return false;
    if (filters.state === "blocked" && !card.blocked) return false;
    if (filters.state === "unblocked" && card.blocked) return false;
    if (filters.state === "repair" && card.needsRepair.length === 0)
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

// chipWorkstreams orders a project's workstream chips by open tickets (not
// done and not in the backlog), most first, and leaves out finished
// workstreams unless they are chosen in the filter (FH-39).
export function chipWorkstreams(
  workstreams: WorkstreamBrief[],
  cards: Card[],
  chosen: string[] = [],
): WorkstreamBrief[] {
  const open = new Map<string, number>();
  for (const card of cards) {
    if (!card.workstream || card.column === "done" || card.column === "backlog")
      continue;
    open.set(card.workstream, (open.get(card.workstream) ?? 0) + 1);
  }
  const finished = (line: WorkstreamBrief) =>
    line.status === "completed" || (line.total > 0 && line.done === line.total);
  return workstreams
    .filter((line) => !finished(line) || chosen.includes(line.slug))
    .map((line, index) => ({ line, index, open: open.get(line.slug) ?? 0 }))
    .sort((a, b) => b.open - a.open || a.index - b.index)
    .map(({ line }) => line);
}

// fittingCount is how many chips of the given widths fit, in order, on one
// line of the available width with gap between them.
export function fittingCount(
  widths: number[],
  available: number,
  gap: number,
): number {
  let used = 0;
  for (const [index, width] of widths.entries()) {
    used += (index > 0 ? gap : 0) + width;
    if (used > available) return index;
  }
  return widths.length;
}
