import { lazy, Suspense } from "react";

import type { LinkContext } from "../../model/markdown";
import type { Upload } from "../../model/uploads";

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
  // upload stores a file in the ticket (FH-18): with it, files pasted,
  // dropped or chosen with Attach files go into the ticket and their
  // references into the text at the cursor. It is read when the editor
  // opens: an editor opened without it keeps dropped files out.
  upload?: Upload;
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
