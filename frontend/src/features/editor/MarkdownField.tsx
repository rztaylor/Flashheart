import type { Extension } from "@codemirror/state";
import { lazy, type Ref, Suspense } from "react";

import type { LinkContext } from "../../model/markdown";

// MarkdownField is the formatted markdown editor (FH-16, D61) as the rest
// of the app uses it. The editor itself (CodeMirror) loads on first use,
// in its own chunk, so the board's first load does not grow.

export interface MarkdownEditorProps {
  // label is the editor's accessible name ("Description").
  label: string;
  value: string;
  onChange(value: string): void;
  // context and keys resolve links and ticket ids as the view does.
  context: LinkContext;
  keys: Set<string>;
  // autoFocus puts the cursor in the text once it loads (default true).
  autoFocus?: boolean;
  // extensions add editor behaviour, such as FH-18's paste and drop of
  // files (EditorView.domEventHandlers).
  extensions?: readonly Extension[];
  // handle inserts text at the cursor and focuses the editor.
  handle?: Ref<MarkdownEditorHandle>;
}

export interface MarkdownEditorHandle {
  // insert replaces the selection with text (markdown), as typed.
  insert(text: string): void;
  focus(): void;
}

const MarkdownEditor = lazy(() => import("./MarkdownEditor"));

export function MarkdownField(props: MarkdownEditorProps) {
  return (
    <Suspense fallback={<Loading />}>
      <MarkdownEditor {...props} />
    </Suspense>
  );
}

// Loading holds the editor's place for the moment its code takes to load.
function Loading() {
  return (
    <div
      role="status"
      className="grid min-h-[10.5rem] place-items-center rounded-control border border-rule bg-card text-sm text-ink-muted"
    >
      Opening the editor…
    </div>
  );
}
