import { EditorView } from "@codemirror/view";

// The editor's look in the board's own tokens (DESIGN.md, ui-layout.md):
// the same type scale and colours as rendered markdown (.markdown in
// styles/index.css), so a section reads the same while edited. The editor
// lives in a shadow root, where the page's stylesheet and Tailwind do not
// reach; the tokens are custom properties, which do, so both themes
// (Metro Pop, Night Service) follow without a second palette.

const mono = "var(--font-mono)";
const headingCut = { fontWeight: "700", fontStretch: "87.5%" };

export const editorTheme = EditorView.theme({
  "&": {
    color: "var(--fh-ink)",
    backgroundColor: "transparent",
    fontSize: "var(--text-base)",
  },
  "&.cm-focused": { outline: "none" },
  ".cm-scroller": {
    fontFamily: "inherit",
    lineHeight: "var(--text-base--line-height)",
  },
  ".cm-content": {
    padding: "10px 0",
    minHeight: "7.5rem",
    caretColor: "var(--fh-ink)",
  },
  ".cm-line": { padding: "0 12px" },
  "&.cm-focused .cm-cursor": { borderLeftColor: "var(--fh-ink)" },
  "& ::selection": {
    backgroundColor: "var(--fh-selection)",
    color: "var(--fh-ink)",
  },
  ".cm-placeholder": { color: "var(--fh-ink-faint)" },

  // Markdown view: the text as written, in the code face.
  "&.fh-markdown .cm-scroller": {
    fontFamily: mono,
    fontSize: "var(--text-sm)",
    lineHeight: "var(--text-sm--line-height)",
  },

  // The live preview (preview.ts).
  ".fh-syntax, .fh-list-mark, .fh-bullet": { color: "var(--fh-ink-faint)" },
  ".fh-h1": {
    ...headingCut,
    fontSize: "var(--text-lg)",
    lineHeight: "var(--text-lg--line-height)",
  },
  ".fh-h2": {
    ...headingCut,
    fontSize: "var(--text-md)",
    lineHeight: "var(--text-md--line-height)",
  },
  ".fh-h3, .fh-h4, .fh-h5, .fh-h6": {
    ...headingCut,
    fontSize: "var(--text-sm)",
    paddingTop: "4px",
  },
  ".fh-strong": { fontWeight: "700" },
  ".fh-em": { fontStyle: "italic" },
  ".fh-strike": { textDecoration: "line-through" },
  ".fh-code": {
    fontFamily: mono,
    fontSize: "0.8125em",
    backgroundColor: "var(--fh-well)",
    borderRadius: "var(--radius-mark)",
  },
  ".fh-codeblock, .fh-fence": {
    fontFamily: mono,
    fontSize: "var(--text-sm)",
    backgroundColor: "var(--fh-well)",
  },
  ".fh-fence": { color: "var(--fh-ink-faint)" },
  ".fh-table": { fontFamily: mono, fontSize: "var(--text-sm)" },
  ".fh-quote": {
    color: "var(--fh-ink-muted)",
    boxShadow: "inset 3px 0 0 var(--fh-rule)",
    paddingLeft: "24px",
  },
  ".fh-link": {
    textDecoration: "underline",
    textDecorationColor: "var(--fh-ink-faint)",
    textUnderlineOffset: "0.2em",
  },
  ".fh-ticket": {
    fontWeight: "500",
    textDecoration: "underline",
    textDecorationColor:
      "color-mix(in srgb, var(--fh-rule-strong) 40%, transparent)",
    textUnderlineOffset: "0.2em",
  },
  ".fh-task": {
    width: "0.875rem",
    height: "0.875rem",
    margin: "0 0.5rem 0 0",
    verticalAlign: "-0.15em",
    accentColor: "var(--fh-select)",
    cursor: "pointer",
  },
  // Inline in its line (a block would open an empty line box above it).
  ".fh-image": {
    display: "inline-block",
    verticalAlign: "bottom",
    maxWidth: "100%",
    maxHeight: "20rem",
    margin: "4px 0",
    border: "1px solid var(--fh-rule)",
    borderRadius: "var(--radius-card)",
  },
  ".fh-image-note": { color: "var(--fh-ink-muted)" },
  ".fh-rule": {
    display: "inline-block",
    width: "100%",
    verticalAlign: "middle",
    borderTop: "1px solid var(--fh-rule)",
  },

  // The slash menu (@codemirror/autocomplete), on the popover surface.
  ".cm-tooltip": {
    border: "none",
    borderRadius: "var(--radius-control)",
    backgroundColor: "var(--fh-popover)",
    color: "var(--fh-ink)",
    boxShadow: "var(--fh-shadow-popover)",
  },
  // Tall enough for every block at once: a list that scrolls would need a
  // tab stop of its own (axe scrollable-region-focusable), and its options
  // are reached with the arrow keys from the text.
  ".cm-tooltip.cm-tooltip-autocomplete > ul": {
    fontFamily: "inherit",
    maxHeight: "none",
    minWidth: "14rem",
    padding: "4px",
  },
  ".cm-tooltip.cm-tooltip-autocomplete > ul > li": {
    display: "flex",
    alignItems: "baseline",
    gap: "12px",
    padding: "6px 10px",
    borderRadius: "var(--radius-inner)",
    lineHeight: "var(--text-base--line-height)",
  },
  ".cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]": {
    backgroundColor: "var(--fh-select-surface) !important",
    color: "var(--fh-ink) !important",
    boxShadow: "inset 0 0 0 1px var(--fh-select)",
  },
  ".cm-completionLabel": { flex: "1" },
  ".cm-completionDetail": {
    marginLeft: "0",
    fontFamily: mono,
    fontStyle: "normal",
    fontSize: "var(--text-xs)",
    color: "var(--fh-ink-muted)",
  },
  ".cm-completionMatchedText": { textDecoration: "none", fontWeight: "700" },
});
