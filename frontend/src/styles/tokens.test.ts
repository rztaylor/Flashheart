import { describe, expect, it } from "vitest";

import css from "./tokens.css?raw";

// The palettes are checked where they are defined: every text and surface
// pair the UI uses must meet WCAG 2.2 AA in both themes (NFR-3), and the
// dark theme's neutrals must be black and charcoal, never navy (D18).

function block(selector: string): Map<string, string> {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`no ${selector} block`);
  const body = css.slice(start, css.indexOf("\n}", start));
  const tokens = new Map<string, string>();
  for (const match of body.matchAll(/--fh-([\w-]+):\s*(#[0-9a-f]{6})\b/gi)) {
    tokens.set(match[1] ?? "", (match[2] ?? "").toLowerCase());
  }
  return tokens;
}

const light = block(":root");
// Dark inherits anything it does not redefine.
const dark = new Map([...light, ...block(':root[data-theme="dark"]')]);
const themes = { light, dark };

function channels(hex: string): [number, number, number] {
  const value = Number.parseInt(hex.slice(1), 16);
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255].map(
    (channel) => {
      const c = channel / 255;
      return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    },
  ) as [number, number, number];
}

function contrast(a: string, b: string): number {
  const luminance = (hex: string) => {
    const [r, g, bl] = channels(hex);
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  };
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05);
}

// oklab gives a colour's OKLab a and b axes.
function oklab(hex: string): [number, number] {
  const [r, g, b] = channels(hex);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ];
}

// hue is OKLCH hue in degrees.
function hue(hex: string): number {
  const [a, b] = oklab(hex);
  return ((Math.atan2(b, a) * 180) / Math.PI + 360) % 360;
}

// chroma is OKLCH chroma: 0 for a true grey.
function chroma(hex: string): number {
  return Math.hypot(...oklab(hex));
}

const surfaces = ["ground", "card", "column", "well", "panel", "popover"];
const text: [string, string, number][] = [
  ...["ink", "ink-muted", "ink-faint"].flatMap((ink) =>
    surfaces.map((surface): [string, string, number] => [ink, surface, 4.5]),
  ),
  ["on-band", "band", 4.5],
  ["on-band-muted", "band", 4.5],
  ["on-band-tab-active", "band-tab-active", 4.5],
  ["on-band-muted", "band-field", 4.5],
  ["on-rail", "rail", 4.5],
  ["on-rail-muted", "rail", 4.5],
  ["on-rail-active", "rail-active", 4.5],
  ["on-rail-active-muted", "rail-active", 4.5],
  ["on-rail-active-mark", "rail-active-mark", 4.5],
  ["on-action", "action", 4.5],
  ["on-attention", "attention", 4.5],
  ["danger", "danger-surface", 4.5],
  ["on-danger", "danger", 4.5],
  ["danger", "card", 4.5],
  ["danger", "ground", 4.5],
  ["ink", "select-surface", 4.5],
  // The ticket page's outline: a count, done or not, on the ground or the
  // current section's surface (FH-65).
  ["ink-faint", "select-surface", 4.5],
  ["state-done-ink", "ground", 4.5],
  ["state-done-ink", "select-surface", 4.5],
  ["focus", "ground", 3],
  ["focus", "card", 3],
  // The no-entry sign (FH-148) is a non-text mark: on its pill it is the
  // blocked pair above; on its own (Blocked by, the State menu) it is the
  // blocked ink on the surface it sits on.
  ...["ground", "card", "panel", "popover"].map(
    (surface): [string, string, number] => ["state-blocked-ink", surface, 3],
  ),
  ...["handoff", "question"].flatMap((callout): [string, string, number][] => [
    ["ink", `callout-${callout}`, 4.5],
    ["ink-muted", `callout-${callout}`, 4.5],
  ]),
  ...["neutral", "progress", "review", "done", "blocked"].map(
    (state): [string, string, number] => [
      `state-${state}-ink`,
      `state-${state}`,
      4.5,
    ],
  ),
  // A column's icon in its state ink beside its word, on the Overview's
  // projects list (FH-153): a graphic, so 3:1.
  ...["neutral", "progress", "review", "done"].map(
    (state): [string, string, number] => [`state-${state}-ink`, "card", 3],
  ),
  ...[
    "type-feature",
    "type-bug",
    "type-infra",
    "type-test",
    "type-refactor",
    "type-docs",
    "type-spike",
    "type-other",
    "priority-high",
    "priority-medium",
    "priority-low",
    "age-today",
    "age-week",
    "age-older",
  ].flatMap((paint): [string, string, number][] => [
    [`paint-${paint}-ink`, `paint-${paint}`, 4.5],
    [`paint-${paint}`, `paint-${paint}-tint`, 4.5],
    ["ink", `paint-${paint}-tint`, 4.5],
    ["ink-muted", `paint-${paint}-tint`, 4.5],
  ]),
  ...Array.from({ length: 9 }, (_, index): [string, string, number] => [
    `line-ink-${index}`,
    `line-${index}`,
    4.5,
  ]),
];

describe.each(Object.entries(themes))("%s palette", (_, tokens) => {
  it.each(text)("%s on %s meets %s:1", (fg, bg, minimum) => {
    const a = tokens.get(fg);
    const b = tokens.get(bg);
    expect(a, `--fh-${fg}`).toBeDefined();
    expect(b, `--fh-${bg}`).toBeDefined();
    expect(contrast(a ?? "", b ?? "")).toBeGreaterThanOrEqual(minimum);
  });
});

describe("dark palette", () => {
  it.each([
    "ground",
    "card",
    "column",
    "well",
    "panel",
    "popover",
    "rule",
    "band",
    "band-field",
    "band-tab-active",
    "rail",
    "rail-active",
    "ink-muted",
    "ink-faint",
  ])("%s is a neutral black or charcoal, not navy", (name) => {
    const value = dark.get(name);
    expect(value, `--fh-${name}`).toBeDefined();
    expect(chroma(value ?? "")).toBeLessThan(0.012);
  });
});

// A card's marks (FH-147, ui-layout.md §2) sit on the card or, with Colour
// by, on any paint's tint, since the whole card takes it. The priority
// bars and type icons are non-text marks (3:1, WCAG 2.2 SC 1.4.11), a
// filled bar also against an empty one; a change-request count is text.
describe.each(Object.entries(themes))("%s card marks", (_, tokens) => {
  const grounds = [
    "card",
    ...[...tokens.keys()].filter(
      (name) => name.startsWith("paint-") && name.endsWith("-tint"),
    ),
  ];
  const marks: [string, number][] = [
    ["priority-high", 3],
    ["priority-medium", 3],
    ["priority-low", 3],
    ...[
      "feature",
      "bug",
      "infra",
      "test",
      "refactor",
      "docs",
      "spike",
      "other",
    ].map((type): [string, number] => [`paint-type-${type}`, 3]),
    ["state-blocked-ink", 3],
    ["state-progress-ink", 4.5],
    ["state-review-ink", 4.5],
    ["ink-muted", 4.5],
  ];
  it.each(
    marks.flatMap(([mark, minimum]) =>
      grounds.map((ground): [string, string, number] => [
        mark,
        ground,
        minimum,
      ]),
    ),
  )("%s on %s meets %s:1", (fg, bg, minimum) => {
    const a = tokens.get(fg);
    const b = tokens.get(bg);
    expect(a, `--fh-${fg}`).toBeDefined();
    expect(b, `--fh-${bg}`).toBeDefined();
    expect(contrast(a ?? "", b ?? "")).toBeGreaterThanOrEqual(minimum);
  });

  it.each(["high", "medium", "low"])(
    "a filled %s bar stands out from an empty one",
    (level) => {
      const filled = tokens.get(`priority-${level}`) ?? "";
      const empty = tokens.get("priority-off") ?? "";
      expect(empty, "--fh-priority-off").not.toBe("");
      expect(contrast(filled, empty)).toBeGreaterThanOrEqual(3);
    },
  );
});

// Beside their words (FH-154), the same marks sit on the Table's and the
// archive's rows (the card surface, the hover well and the selected row's
// tint), the card panel's header, the ticket page's properties (the column
// surface) and the Colour by chips (the card, the chosen chip's tint and
// the ground under a hidden chip): 3:1 on each, in both themes.
describe.each(Object.entries(themes))("%s marks beside words", (_, tokens) => {
  const grounds = [
    "card",
    "well",
    "select-surface",
    "panel",
    "column",
    "ground",
  ];
  const marks = [
    "priority-high",
    "priority-medium",
    "priority-low",
    ...[
      "feature",
      "bug",
      "infra",
      "test",
      "refactor",
      "docs",
      "spike",
      "other",
    ].map((type) => `paint-type-${type}`),
  ];
  it.each(
    marks.flatMap((mark) =>
      grounds.map((ground): [string, string] => [mark, ground]),
    ),
  )("%s on %s meets 3:1", (fg, bg) => {
    const a = tokens.get(fg);
    const b = tokens.get(bg);
    expect(a, `--fh-${fg}`).toBeDefined();
    expect(b, `--fh-${bg}`).toBeDefined();
    expect(contrast(a ?? "", b ?? "")).toBeGreaterThanOrEqual(3);
  });
});

// Colour by Priority tints cards in the priority bars' own colours, so a
// priority has one colour code on the board (FH-166): high and low share
// their bar's hue, medium stays a neutral grey like its bars.
describe.each(Object.entries(themes))("%s priority paint", (_, tokens) => {
  it.each(["high", "low"])("%s paint and tint share the bar's hue", (level) => {
    const bar = hue(tokens.get(`priority-${level}`) ?? "");
    for (const name of [
      `paint-priority-${level}`,
      `paint-priority-${level}-tint`,
    ]) {
      const shade = tokens.get(name);
      expect(shade, `--fh-${name}`).toBeDefined();
      const apart = Math.abs(hue(shade ?? "") - bar);
      expect(Math.min(apart, 360 - apart), name).toBeLessThan(20);
    }
  });

  it("medium paint and tint are neutral grey", () => {
    for (const name of [
      "paint-priority-medium",
      "paint-priority-medium-tint",
    ]) {
      expect(chroma(tokens.get(name) ?? ""), name).toBeLessThan(0.02);
    }
  });
});
