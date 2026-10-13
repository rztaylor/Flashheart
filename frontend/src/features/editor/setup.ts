import {
  autocompletion,
  closeCompletion,
  completionKeymap,
  currentCompletions,
} from "@codemirror/autocomplete";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { markdownKeymap } from "@codemirror/lang-markdown";
import {
  Compartment,
  EditorState,
  type Extension,
  Prec,
} from "@codemirror/state";
import {
  EditorView,
  type KeyBinding,
  keymap,
  placeholder,
} from "@codemirror/view";

import type { EditorMode } from "../../api/preferences";
import {
  activate,
  insertLink,
  toggleBold,
  toggleCode,
  toggleItalic,
} from "./commands";
import { markdownSupport } from "./language";
import { type LinkOptions, openTarget, targetAt } from "./links";
import { pasteInput } from "./paste";
import { livePreview } from "./preview";
import { slashCompletions } from "./slash";
import { editorTheme } from "./theme";
import {
  type AttachHandler,
  fileInput,
  ignoreDroppedFiles,
  pendingUploads,
} from "./uploads";

// The editor's CodeMirror setup: markdown, history, the keys, the slash
// menu, paste (files, then a web page's HTML as markdown, else plain
// text; Shift for plain text), dropped files, link clicks and, in the
// formatted view, the live preview. The page around it (InPlaceEditor) owns Ctrl or Cmd with Enter
// (save) and Escape (cancel), so the editor leaves both alone; Tab is not
// bound, so it leaves the editor.

export interface EditorOptions extends LinkOptions {
  // label is the editor's accessible name ("Description").
  label: string;
  // onToolbar moves focus to the selection toolbar (Alt+F10).
  onToolbar?(): boolean;
  // onFiles attaches files pasted or dropped into the text (FH-18).
  onFiles?: AttachHandler;
}

// The keys the page owns: saving and cancelling the open item.
const pageKeys = new Set(["Mod-Enter", "Escape"]);

// menuKeys are the slash menu's keys. Escape closes the menu only while it
// shows; otherwise it is the page's (the stock binding also takes it while
// a query is pending, just after typing).
const menuKeys: KeyBinding[] = completionKeymap.map((binding) =>
  binding.key === "Escape"
    ? {
        key: "Escape",
        run: (view) =>
          currentCompletions(view.state).length > 0 && closeCompletion(view),
      }
    : binding,
);

// formatKeys are the formatting shortcuts the toolbar names.
export const formatKeys = {
  bold: "Mod-b",
  italic: "Mod-i",
  code: "Mod-e",
  link: "Mod-k",
} as const;

function editorKeys(options: EditorOptions): KeyBinding[] {
  return [
    { key: formatKeys.bold, run: toggleBold },
    { key: formatKeys.italic, run: toggleItalic },
    { key: formatKeys.code, run: toggleCode },
    { key: formatKeys.link, run: insertLink },
    // Alt+Enter opens the link under the cursor, or ticks its line's task.
    { key: "Alt-Enter", run: activate(options) },
    { key: "Alt-F10", run: () => options.onToolbar?.() ?? false },
  ];
}

// The view, formatted or markdown, is one compartment: switching it
// changes only decorations, never the text.
export const modeCompartment = new Compartment();

export function modeExtensions(
  mode: EditorMode,
  options: LinkOptions,
): Extension {
  return mode === "formatted"
    ? [
        livePreview(options),
        EditorView.editorAttributes.of({ class: "fh-formatted" }),
      ]
    : EditorView.editorAttributes.of({ class: "fh-markdown" });
}

// linkClicks opens a link or ticket id clicked with Ctrl or Cmd held, in a
// new tab (a plain click places the cursor to edit it).
function linkClicks(options: LinkOptions): Extension {
  return EditorView.domEventHandlers({
    mousedown(event, view) {
      if (!(event.ctrlKey || event.metaKey) || event.button !== 0) return false;
      const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
      if (pos === null) return false;
      if (!openTarget(targetAt(view.state, pos, options))) return false;
      event.preventDefault();
      return true;
    },
  });
}

// editorExtensions is the whole setup for one editor in mode.
export function editorExtensions(
  options: EditorOptions,
  mode: EditorMode,
): Extension[] {
  return [
    markdownSupport,
    history(),
    EditorView.lineWrapping,
    EditorView.contentAttributes.of({
      "aria-label": options.label,
      spellcheck: "true",
    }),
    EditorState.phrases.of({ Completions: "Blocks" }),
    placeholder("Write in markdown. Type / for blocks."),
    autocompletion({
      override: [slashCompletions],
      icons: false,
      defaultKeymap: false,
    }),
    Prec.highest(keymap.of(menuKeys)),
    keymap.of([
      ...editorKeys(options),
      ...markdownKeymap,
      ...historyKeymap,
      ...defaultKeymap.filter((binding) => !pageKeys.has(binding.key ?? "")),
    ]),
    linkClicks(options),
    // Pasted files are attached where the editor can attach them (FH-18),
    // pasted web HTML becomes markdown (FH-142); dropped files are
    // attached likewise, and otherwise kept out of the text.
    pasteInput(options.onFiles),
    options.onFiles ? fileInput(options.onFiles) : ignoreDroppedFiles,
    pendingUploads,
    modeCompartment.of(modeExtensions(mode, options)),
    editorTheme,
  ];
}
