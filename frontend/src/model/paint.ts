import type { Card } from "../api/board";
import { priorityLabel } from "./status";

// Paint is the board's "Colour by" option: a ticket attribute shown as a
// tinted card header and a named tag (VIEW-6). Workstreams keep the line
// colours; paint never uses them.
export type PaintMode = "type" | "priority" | "age" | "none";

export const PAINT_MODES: { value: PaintMode; label: string }[] = [
  { value: "type", label: "Type" },
  { value: "priority", label: "Priority" },
  { value: "age", label: "Age" },
  { value: "none", label: "None" },
];

// Paint names a colour token (--fh-paint-<token>, -tint, -ink), the words
// that always accompany it and the value it stands for, which filters by it
// (FH-39).
export interface Paint {
  token: string;
  label: string;
  value: string;
}

const types = [
  "feature",
  "bug",
  "infra",
  "test",
  "refactor",
  "docs",
  "spike",
] as const;
const priorities = ["high", "medium", "low"] as const;
const ages = [
  ["today", "Today"],
  ["week", "This week"],
  ["older", "Older"],
] as const;

const day = 24 * 60 * 60 * 1000;

function ageBucket(iso: string, now: Date) {
  const then = Date.parse(iso);
  if (!iso || Number.isNaN(then)) return undefined;
  const elapsed = now.getTime() - then;
  if (elapsed < day) return ages[0];
  if (elapsed < 7 * day) return ages[1];
  return ages[2];
}

// ageOf is the age bucket of a card's last change: today, week or older.
export function ageOf(card: Card, now: Date): string | undefined {
  return ageBucket(card.modified, now)?.[0];
}

export function paintFor(
  card: Card,
  mode: PaintMode,
  now: Date,
): Paint | undefined {
  switch (mode) {
    case "type": {
      if (!card.type) return undefined;
      const known = (types as readonly string[]).includes(card.type);
      return {
        token: `type-${known ? card.type : "other"}`,
        label: card.type,
        value: card.type,
      };
    }
    case "priority": {
      const match = priorities.find((value) => value === card.priority);
      return match
        ? {
            token: `priority-${match}`,
            label: priorityLabel(match),
            value: match,
          }
        : undefined;
    }
    case "age": {
      const bucket = ageBucket(card.modified, now);
      return bucket
        ? { token: `age-${bucket[0]}`, label: bucket[1], value: bucket[0] }
        : undefined;
    }
    default:
      return undefined;
  }
}

// paintKey lists the paints present on the cards, in the mode's own order,
// for the board's colour key.
export function paintKey(cards: Card[], mode: PaintMode, now: Date): Paint[] {
  const seen = new Map<string, Paint>();
  for (const card of cards) {
    const paint = paintFor(card, mode, now);
    if (paint && !seen.has(paint.label)) seen.set(paint.label, paint);
  }
  const order: string[] =
    mode === "type"
      ? [...types]
      : mode === "priority"
        ? priorities.map(priorityLabel)
        : ages.map(([, label]) => label);
  const rank = (paint: Paint) => {
    const index = order.indexOf(paint.label);
    return index === -1 ? order.length : index;
  };
  return [...seen.values()].sort(
    (a, b) => rank(a) - rank(b) || a.label.localeCompare(b.label),
  );
}

// paintVars returns the CSS custom properties a painted element reads.
export function paintVars(paint: Paint | undefined) {
  if (!paint) return undefined;
  return {
    "--paint": `var(--fh-paint-${paint.token})`,
    "--paint-tint": `var(--fh-paint-${paint.token}-tint)`,
    "--paint-ink": `var(--fh-paint-${paint.token}-ink)`,
  } as Record<string, string>;
}
