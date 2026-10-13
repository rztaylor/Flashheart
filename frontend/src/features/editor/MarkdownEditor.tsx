import { Annotation, EditorState, Transaction } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { type FocusEvent, useEffect, useRef, useState } from "react";

import type { EditorMode } from "../../api/preferences";
import {
  AttachButton,
  FileDropArea,
  UploadNotes,
  useUploadNotes,
} from "../../components/FileDrop";
import { SegmentedControl } from "../../components/SegmentedControl";
import { ALLOWED_WORDS } from "../../model/uploads";
import type { MarkdownEditorProps } from "./MarkdownField";
import { useEditorMode } from "./mode";
import { SelectionToolbar } from "./SelectionToolbar";
import {
  type EditorOptions,
  editorExtensions,
  modeCompartment,
  modeExtensions,
} from "./setup";
import { attachFiles } from "./uploads";

// MarkdownEditor is the formatted markdown editor (FH-16, D61): CodeMirror
// editing the markdown text itself, with a live preview, a slash menu, a
// selection toolbar and a switch to the markdown as written. It mounts in
// a shadow root: CodeMirror styles itself with a style sheet it builds at
// run time, which the page's CSP (style-src 'self') refuses in the
// document but allows as a shadow root's constructed style sheet.
// Loaded lazily through MarkdownField.

// external marks a change that came from outside (a newer version of the
// file adopted by the draft), which is not the person's edit.
const external = Annotation.define<boolean>();

const modes: { value: EditorMode; label: string }[] = [
  { value: "formatted", label: "Formatted" },
  { value: "markdown", label: "Markdown" },
];

// TOOLBAR_ROOM is the height the toolbar needs above the selection; nearer
// the top it goes below the selection instead.
const TOOLBAR_ROOM = 44;
const TOOLBAR_WIDTH = 216;

export default function MarkdownEditor({
  label,
  value,
  onChange,
  context,
  keys,
  autoFocus = true,
  upload,
}: MarkdownEditorProps) {
  const { mode, setMode } = useEditorMode();
  const host = useRef<HTMLDivElement>(null);
  const frame = useRef<HTMLDivElement>(null);
  const toolbar = useRef<HTMLDivElement>(null);
  const [view, setView] = useState<EditorView | null>(null);
  const latest = useRef({ onChange, context, keys, mode, upload });
  latest.current = { onChange, context, keys, mode, upload };
  // What happened to the files last given: in progress or attached, and
  // the refusals with why (FH-18).
  const notes = useUploadNotes();
  const attach = (
    editor: EditorView,
    files: File[],
    pos: number,
    pasted: boolean,
  ) => {
    const send = latest.current.upload;
    if (!send || files.length === 0) return;
    notes.start(files.length);
    void attachFiles(editor, files, pos, send, pasted).then(notes.finish);
  };
  const attachRef = useRef(attach);
  attachRef.current = attach;
  // The toolbar shows over a selection while focus is in the text or the
  // toolbar, or wherever the cursor is after Alt+F10 (asked).
  const [focusWithin, setFocusWithin] = useState(false);
  const [asked, setAsked] = useState(false);
  const [selected, setSelected] = useState(false);
  const [spot, setSpot] = useState<{ top: number; left: number } | null>(null);
  const focusToolbar = useRef(false);

  // The editor is made once; its options read the latest props.
  // biome-ignore lint/correctness/useExhaustiveDependencies: mounts once; later values reach it through latest and the effects below.
  useEffect(() => {
    const element = host.current;
    if (!element) return;
    const root =
      element.shadowRoot ??
      element.attachShadow({ mode: "open", delegatesFocus: true });
    const parent = document.createElement("div");
    root.append(parent);
    const options: EditorOptions = {
      label,
      get context() {
        return latest.current.context;
      },
      get keys() {
        return latest.current.keys;
      },
      onToolbar() {
        focusToolbar.current = true;
        setAsked(true);
        return true;
      },
      ...(upload
        ? {
            onFiles: (editor, files, pos, pasted) =>
              attachRef.current(editor, files, pos, pasted),
          }
        : {}),
    };
    const place = (editor: EditorView) => {
      const { main } = editor.state.selection;
      setSelected(!main.empty);
      editor.requestMeasure({
        key: "fh-toolbar",
        read() {
          const box = frame.current?.getBoundingClientRect();
          const start = editor.coordsAtPos(main.from);
          const end = editor.coordsAtPos(main.to) ?? start;
          if (!box || !start || !end) return null;
          const above = start.top - box.top >= TOOLBAR_ROOM;
          return {
            top: above
              ? start.top - box.top - 6
              : end.bottom - box.top + 6 + TOOLBAR_ROOM - 4,
            left: Math.max(
              0,
              Math.min(start.left - box.left, box.width - TOOLBAR_WIDTH),
            ),
          };
        },
        write(measured) {
          setSpot(measured);
        },
      });
    };
    const editor = new EditorView({
      parent,
      root,
      state: EditorState.create({
        doc: value,
        extensions: [
          editorExtensions(options, latest.current.mode),
          EditorView.updateListener.of((update) => {
            if (
              update.docChanged &&
              !update.transactions.some((tr) => tr.annotation(external))
            )
              latest.current.onChange(update.state.doc.toString());
            if (
              update.selectionSet ||
              update.docChanged ||
              update.geometryChanged
            )
              place(update.view);
          }),
        ],
      }),
    });
    optionsOf.set(editor, options);
    setView(editor);
    place(editor);
    if (autoFocus) editor.focus();
    return () => {
      editor.destroy();
      parent.remove();
      setView(null);
    };
  }, []);

  // A newer version of the text from outside replaces the document; it is
  // not an edit, so it is neither reported nor undone.
  useEffect(() => {
    if (!view) return;
    const current = view.state.doc.toString();
    if (current === value) return;
    view.dispatch({
      changes: { from: 0, to: current.length, insert: value },
      annotations: [external.of(true), Transaction.addToHistory.of(false)],
    });
  }, [value, view]);

  // Switching between formatted and markdown changes only decorations.
  useEffect(() => {
    const options = view && optionsOf.get(view);
    if (!view || !options) return;
    view.dispatch({
      effects: modeCompartment.reconfigure(modeExtensions(mode, options)),
    });
  }, [mode, view]);

  const shown = view && spot && focusWithin && (selected || asked);
  // Alt+F10 moves focus into the toolbar once it shows.
  useEffect(() => {
    if (!shown || !focusToolbar.current) return;
    focusToolbar.current = false;
    toolbar.current?.querySelector<HTMLElement>('[tabindex="0"]')?.focus();
  });

  const onBlur = (event: FocusEvent) => {
    if (frame.current?.contains(event.relatedTarget as Node | null)) return;
    setFocusWithin(false);
    setAsked(false);
  };

  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
        <p className="min-w-0 text-xs text-ink-muted">
          {mode === "formatted"
            ? "Type / for blocks · select text to format · Alt+Enter opens a link or ticks a task"
            : "The markdown as it is saved · type / for blocks"}
        </p>
        <SegmentedControl
          label="Show the text as"
          options={modes}
          value={mode}
          onChange={setMode}
        />
      </div>
      {/* A drop the text did not take (on its padding) goes to the cursor;
          the text shows it will take files only where they are attached. */}
      <FileDropArea
        className={upload ? "group min-w-0" : "min-w-0"}
        onFiles={(files) => {
          if (view && upload)
            attach(view, files, view.state.selection.main.head, false);
        }}
      >
        {/* biome-ignore lint/a11y/noStaticElementInteractions: it only notes whether focus is in the text or its toolbar. */}
        <div
          ref={frame}
          className="relative min-w-0"
          onFocus={() => setFocusWithin(true)}
          onBlur={onBlur}
        >
          <div
            ref={host}
            data-autofocus
            className="min-w-0 rounded-control border border-rule bg-card transition-colors hover:border-ink-muted focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-focus group-data-dropping:border-select group-data-dropping:ring-2 group-data-dropping:ring-select"
          />
          {shown ? (
            <SelectionToolbar view={view} position={spot} toolbar={toolbar} />
          ) : null}
        </div>
      </FileDropArea>
      {upload ? (
        <div className="flex min-w-0 flex-col gap-1">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <AttachButton
              disabled={!view}
              onFiles={(files) => {
                if (view)
                  attach(view, files, view.state.selection.main.head, false);
              }}
            />
            <p className="min-w-0 text-xs text-ink-muted">
              Paste or drop files to attach them · {ALLOWED_WORDS}
            </p>
          </div>
          <UploadNotes
            refusals={notes.refusals}
            status={notes.status}
            onDismiss={() => {
              notes.dismiss();
              view?.focus();
            }}
          />
        </div>
      ) : null}
    </div>
  );
}

// optionsOf keeps each editor's options for reconfiguring its view.
const optionsOf = new WeakMap<EditorView, EditorOptions>();
