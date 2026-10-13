import { type Extension, StateEffect, StateField } from "@codemirror/state";
import {
  Decoration,
  type DecorationSet,
  EditorView,
  WidgetType,
} from "@codemirror/view";

import { hasFiles } from "../../components/FileDrop";
import {
  referenceFor,
  type Upload,
  type UploadResult,
  uploadAll,
  uploadName,
} from "../../model/uploads";

// Files pasted, dropped or chosen into the editor (FH-18): each shows a
// placeholder where it goes while it uploads, which becomes its markdown
// reference (an image for an image, a link for anything else) or goes
// away if the file is refused. The placeholder is a decoration, never
// text, so a save made meanwhile carries none of it; the reference is
// typed into the text like any edit and saved with the section.

interface Pending {
  id: number;
  pos: number;
  name: string;
  // order keeps a batch's placeholders, at one place, in their order.
  order: number;
}

const addPending = StateEffect.define<Pending>({
  map: (value, change) => ({ ...value, pos: change.mapPos(value.pos, 1) }),
});
const removePending = StateEffect.define<number>();

// PendingWidget is a file's placeholder while it uploads.
class PendingWidget extends WidgetType {
  constructor(
    readonly id: number,
    readonly fileName: string,
  ) {
    super();
  }
  get name() {
    return `uploading(${this.fileName})`;
  }
  eq(other: PendingWidget) {
    return other.id === this.id;
  }
  toDOM() {
    const note = document.createElement("span");
    note.className = "fh-upload";
    note.dataset.uploading = this.fileName;
    note.textContent = `Uploading ${this.fileName}…`;
    return note;
  }
  ignoreEvent() {
    return true;
  }
}

// pendingUploads holds the placeholders, moving with the text around them.
export const pendingUploads = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(placeholders, tr) {
    let next = placeholders.map(tr.changes);
    for (const effect of tr.effects) {
      if (effect.is(addPending)) {
        const { id, pos, name, order } = effect.value;
        next = next.update({
          add: [
            Decoration.widget({
              widget: new PendingWidget(id, name),
              side: 1 + order,
            }).range(pos),
          ],
          sort: true,
        });
      } else if (effect.is(removePending)) {
        next = next.update({
          filter: (_from, _to, decoration) =>
            (decoration.spec.widget as PendingWidget).id !== effect.value,
        });
      }
    }
    return next;
  },
  provide: (field) => EditorView.decorations.from(field),
});

// placeholderAt is where a file's placeholder is now, if it is still there.
function placeholderAt(view: EditorView, id: number): number | undefined {
  let found: number | undefined;
  view.state
    .field(pendingUploads, false)
    ?.between(0, view.state.doc.length, (from, _to, decoration) => {
      if ((decoration.spec.widget as PendingWidget).id !== id) return;
      found = from;
      return false;
    });
  return found;
}

let nextId = 1;

// attachFiles uploads files into the ticket, one at a time, with a
// placeholder for each at pos; each upload that succeeds puts its
// reference where its placeholder was (on its own line after another of
// the same batch), moving a cursor that was there past it. It resolves
// with each file's result, for the words.
export async function attachFiles(
  view: EditorView,
  files: File[],
  pos: number,
  upload: Upload,
  pasted: boolean,
): Promise<UploadResult[]> {
  const batch = files.map((file, order) => ({
    id: nextId++,
    name: uploadName(file, pasted),
    order,
  }));
  view.dispatch({
    effects: batch.map(({ id, name, order }) =>
      addPending.of({ id, pos, name, order }),
    ),
  });
  let inserted = false;
  return uploadAll(files, upload, pasted, (result, index) => {
    const id = batch[index]?.id ?? 0;
    const at = placeholderAt(view, id);
    if (!result.uploaded || at === undefined) {
      view.dispatch({ effects: removePending.of(id) });
      return;
    }
    const text =
      (inserted ? "\n" : "") + referenceFor(result.uploaded, result.name);
    const cursor = view.state.selection.main;
    view.dispatch({
      changes: { from: at, insert: text },
      effects: removePending.of(id),
      ...(cursor.empty && cursor.head === at
        ? { selection: { anchor: at + text.length } }
        : {}),
      userEvent: "input",
    });
    inserted = true;
  });
}

// AttachHandler takes files given to the editor: where they go, and
// whether they were pasted (a pasted image is a screenshot).
export type AttachHandler = (
  view: EditorView,
  files: File[],
  pos: number,
  pasted: boolean,
) => void;

// fileInput sends dropped files to onFiles, where they are dropped.
// Pasted files go through the editor's paste handler (paste.ts), which
// puts them before pasted text and HTML.
export function fileInput(onFiles: AttachHandler): Extension {
  return EditorView.domEventHandlers({
    dragover(event) {
      if (!hasFiles(event.dataTransfer)) return false;
      event.preventDefault();
      if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
      return true;
    },
    drop(event, view) {
      const files = Array.from(event.dataTransfer?.files ?? []);
      if (!files.length) return false;
      event.preventDefault();
      onFiles(view, files, dropPos(view, event), false);
      return true;
    },
  });
}

// dropPos is where a drop lands in the text, else the cursor.
function dropPos(view: EditorView, event: DragEvent): number {
  try {
    const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
    if (pos !== null) return pos;
  } catch {
    // No layout to measure (a test document): the cursor will do.
  }
  return view.state.selection.main.head;
}

// ignoreDroppedFiles keeps a dropped file out of the text (CodeMirror
// would insert its contents) where files cannot be attached.
export const ignoreDroppedFiles = EditorView.domEventHandlers({
  drop(event) {
    if (!event.dataTransfer?.files.length) return false;
    event.preventDefault();
    return true;
  },
});
