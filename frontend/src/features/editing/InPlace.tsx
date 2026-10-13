import {
  createContext,
  type Dispatch,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type RefObject,
  type SetStateAction,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";

import type { Saved } from "../../api/edit";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { Icon } from "../../components/Icon";
import { guardLeaving } from "../../state/leaveGuard";
import { ConflictDialog } from "./ConflictDialog";
import { useTicketDraft } from "./useTicketDraft";

// In-place editing (FH-66): one item of a ticket (its title, a section, a
// property) open for editing at a time, where it is shown. InPlaceEditing
// coordinates the items on a page: opening another item, or leaving the
// page, while the open one has unsaved changes asks whether to keep or
// discard them. Each item opens with useInPlaceItem and edits in an
// InPlaceEditor, one draft per open item (useTicketDraft).

interface OpenItem {
  id: string;
  name: string;
}

interface Coordinator {
  open: string | null;
  request(id: string, name: string): void;
  finish(id: string): void;
  setDirty(id: string, dirty: boolean): void;
  setFocus(id: string, focus: (() => void) | null): void;
  holdDraft(id: string, draft: HeldDraft | null): void;
}

// HeldDraft is unsaved work on the page outside any open item (a comment
// being written, FH-125; the card panel's Edit tab, FH-138): leaving the
// page asks about it too, and Discard drops it. returnTo, when given,
// shows the draft and moves focus into it: Keep editing calls it when the
// prompt was not opened from a field.
interface HeldDraft {
  name: string;
  discard(): void;
  returnTo?(): void;
}

// inField: focus is in a control that takes text or a choice, where the
// person was editing when the prompt opened (the dialog returns focus
// there when it closes).
const inField = (element: Element | null) =>
  !!element?.matches(
    "input, textarea, select, [contenteditable]:not([contenteditable=false])",
  );

const EditingContext = createContext<Coordinator | null>(null);

// InPlaceEditing holds the page's one open item, its unsaved-changes
// prompt, and the guard on leaving the page while it has unsaved changes.
export function InPlaceEditing({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<OpenItem | null>(null);
  const [dirty, setDirty] = useState(false);
  // asking is the prompt shown: the open item's name, and what Discard
  // goes on to do.
  const [asking, setAsking] = useState<{
    name: string;
    discard(): void;
  } | null>(null);
  const [kept, setKept] = useState(0);
  const [drafts, setDrafts] = useState<ReadonlyMap<string, HeldDraft>>(
    () => new Map(),
  );
  const openId = useRef<string | null>(null);
  openId.current = open?.id ?? null;
  const focusOpen = useRef<{ id: string; focus(): void } | null>(null);
  const release = useRef<(() => void) | null>(null);
  const held = useRef(drafts);
  held.current = drafts;
  const openDirty = useRef(false);
  openDirty.current = !!open && dirty;

  const show = (next: OpenItem | null) => {
    setDirty(false);
    setOpen(next);
  };

  // While the open item or a held draft has unsaved changes, leaving the
  // page asks first, naming them; Discard drops them all and lets the
  // navigation go on once nothing guards it any more.
  useEffect(() => {
    const names = [
      ...(open && dirty ? [open.name] : []),
      ...[...drafts.values()].map((draft) => draft.name),
    ];
    if (names.length === 0) return;
    const drop = guardLeaving((proceed) =>
      setAsking({
        name: names.join(" and the "),
        discard: () => {
          release.current?.();
          setDirty(false);
          setOpen(null);
          for (const draft of drafts.values()) draft.discard();
          setDrafts(new Map());
          proceed();
        },
      }),
    );
    release.current = drop;
    return () => {
      drop();
      if (release.current === drop) release.current = null;
    };
  }, [open, dirty, drafts]);

  // Keep editing returns focus to the open item once the prompt is gone,
  // unless only a held draft that can take focus has changes: then focus
  // stays in the field the prompt was opened from, or else goes back to
  // that draft (the Edit tab, after Close or another card was pressed).
  useEffect(() => {
    if (kept === 0) return;
    const back = [...held.current.values()].find((draft) => draft.returnTo);
    if (focusOpen.current && (openDirty.current || !back))
      focusOpen.current.focus();
    else if (back && !inField(document.activeElement)) back.returnTo?.();
  }, [kept]);

  const coordinator: Coordinator = {
    open: open?.id ?? null,
    request(id, name) {
      if (open?.id === id) return;
      if (open && dirty) {
        setAsking({ name: open.name, discard: () => show({ id, name }) });
        return;
      }
      show({ id, name });
    },
    finish(id) {
      if (openId.current === id) show(null);
    },
    setDirty(id, value) {
      if (openId.current === id) setDirty(value);
    },
    setFocus(id, focus) {
      if (focus && openId.current === id) focusOpen.current = { id, focus };
      else if (!focus && focusOpen.current?.id === id) focusOpen.current = null;
    },
    holdDraft(id, draft) {
      setDrafts((held) => {
        if (!draft && !held.has(id)) return held;
        const next = new Map(held);
        if (draft) next.set(id, draft);
        else next.delete(id);
        return next;
      });
    },
  };

  return (
    <EditingContext.Provider value={coordinator}>
      {children}
      {asking ? (
        <Dialog
          title={`Unsaved changes to the ${asking.name}`}
          onClose={() => {
            setAsking(null);
            setKept((count) => count + 1);
          }}
          actions={
            <>
              <Button
                onClick={() => {
                  setAsking(null);
                  asking.discard();
                }}
              >
                Discard changes
              </Button>
              <Button
                variant="primary"
                onClick={() => {
                  setAsking(null);
                  setKept((count) => count + 1);
                }}
              >
                Keep editing
              </Button>
            </>
          }
        >
          <p className="text-ink-muted">
            The {asking.name} has changes that are not saved. Keep editing to
            save them, or discard them.
          </p>
        </Dialog>
      ) : null}
    </EditingContext.Provider>
  );
}

export interface InPlaceItem {
  // editable: the item is on a page that edits in place.
  editable: boolean;
  open: boolean;
  // trigger is what opened the item (its pencil, or Add): focus returns
  // to it when the edit is saved or cancelled.
  trigger: RefObject<HTMLButtonElement | null>;
  start(): void;
  // finish closes the item after a save or cancel, returning focus.
  finish(): void;
  setDirty(dirty: boolean): void;
  setFocus(focus: (() => void) | null): void;
  // onDoubleClick opens the item from its content, unless the double
  // click was on a link or control.
  onDoubleClick(event: MouseEvent): void;
}

// useInPlaceItem is one editable item of the page: id is unique on the
// page, name says what it is ("description", "priority").
export function useInPlaceItem(id: string, name: string): InPlaceItem {
  const coordinator = useContext(EditingContext);
  const open = coordinator?.open === id;
  const trigger = useRef<HTMLButtonElement>(null);
  // returning holds focus on the item after a save or cancel until the
  // person moves on: the saved version may replace its trigger (Add
  // context becomes Edit context), and focus follows to the new one.
  const returning = useRef(false);
  useEffect(() => {
    if (open || !returning.current || !trigger.current) return;
    const active = document.activeElement;
    if (active === trigger.current) return;
    if (!active || active === document.body || !active.isConnected)
      trigger.current.focus();
  });
  const finish = () => {
    returning.current = true;
    const moveOn = () => {
      returning.current = false;
      window.removeEventListener("pointerdown", moveOn, true);
      window.removeEventListener("keydown", moveOn, true);
    };
    window.addEventListener("pointerdown", moveOn, true);
    window.addEventListener("keydown", moveOn, true);
    coordinator?.finish(id);
  };
  return {
    editable: !!coordinator,
    open,
    trigger,
    start: () => coordinator?.request(id, name),
    finish,
    setDirty: (dirty) => coordinator?.setDirty(id, dirty),
    setFocus: (focus) => coordinator?.setFocus(id, focus),
    onDoubleClick: (event) => {
      const target = event.target as Element;
      if (target.closest("a, button, input, label, select, textarea, summary"))
        return;
      coordinator?.request(id, name);
    },
  };
}

// useHeldDraft holds unsaved work that is not an in-place item (a comment
// being written, the panel's Edit tab) while dirty, so leaving the page
// asks about it as about the open item; discard empties it, and returnTo,
// when given, shows it and moves focus into it on Keep editing. Opening
// items never asks about it.
export function useHeldDraft(
  id: string,
  name: string,
  dirty: boolean,
  discard: () => void,
  returnTo?: () => void,
) {
  const coordinator = useContext(EditingContext);
  const latest = useRef({ coordinator, name, discard, returnTo });
  latest.current = { coordinator, name, discard, returnTo };
  const returns = !!returnTo;
  useEffect(() => {
    const { coordinator: held } = latest.current;
    if (!dirty) return;
    held?.holdDraft(id, {
      name: latest.current.name,
      discard: () => latest.current.discard(),
      ...(returns ? { returnTo: () => latest.current.returnTo?.() } : {}),
    });
    return () => held?.holdDraft(id, null);
  }, [id, dirty, returns]);
}

// editGroup is the class an item's container takes so its pencil shows
// while the item is hovered or holds focus.
export const editGroup = "group/edit";

// EditPencil opens an item ("Edit description"). It shows while its item
// is hovered or focused, and always where there is no hover (touch).
export function EditPencil({
  item,
  name,
  className = "",
}: {
  item: InPlaceItem;
  name: string;
  className?: string;
}) {
  return (
    <button
      ref={item.trigger}
      type="button"
      aria-label={`Edit ${name}`}
      title={`Edit ${name}`}
      onClick={item.start}
      className={`grid size-7 shrink-0 place-items-center rounded-control text-ink-muted opacity-0 transition-opacity group-focus-within/edit:opacity-100 group-hover/edit:opacity-100 hover:bg-well hover:text-ink focus-visible:opacity-100 [@media(hover:none)]:opacity-100 ${className}`}
    >
      <Icon name="edit" size={14} />
    </button>
  );
}

// AddItem opens an empty section in place ("Add context"): an empty slot.
export function AddItem({ item, name }: { item: InPlaceItem; name: string }) {
  return (
    <button
      ref={item.trigger}
      type="button"
      onClick={item.start}
      className="flex w-full items-center gap-2 rounded-card border border-dashed border-rule px-3 py-2 text-left text-sm font-medium text-ink-muted transition-colors hover:border-ink-muted hover:text-ink"
    >
      <Icon name="plus" size={14} />
      Add {name}
    </button>
  );
}

// mac: shortcuts take Cmd (⌘) rather than Ctrl.
export const mac =
  typeof navigator !== "undefined" &&
  /Mac|iP(hone|ad)/.test(navigator.platform);
// sendKeys names Ctrl or Cmd and Enter, which saves an open item and posts
// a comment.
export const sendKeys = `${mac ? "⌘" : "Ctrl+"}Enter`;
// shortcuts names the keys that save and cancel an open item.
export const shortcuts = `${sendKeys} saves · Esc cancels`;

export interface InPlaceEditorProps<D extends { id: string; hash: string }, T> {
  item: InPlaceItem;
  // name says what is edited, for the editor's group ("Editing tags").
  name: string;
  detail: D;
  // read is the item's value in a version of the ticket; isDirty measures
  // an edit against the version it opened on.
  read(detail: D): T;
  isDirty(draft: T, base: T): boolean;
  // send saves value on base, the hash of the version it was made on.
  send(base: string, value: T): Promise<Saved>;
  // mine shows value as "Your version" in the conflict dialog.
  mine(value: T): string;
  // theirs is the same part of the file now on disk, for "Now on disk";
  // without it the dialog shows the whole file.
  theirs?(file: string): string;
  // readFile is the item's value in a file's text: a save that conflicts
  // with a version that left it as it was is saved again (useTicketDraft).
  readFile?(file: string): T;
  // check refuses a value before it is sent, saying why.
  check?(value: T): string | undefined;
  onSaved(saved: Saved | undefined, what: string): void;
  children(value: T, set: Dispatch<SetStateAction<T>>): ReactNode;
  className?: string;
}

// InPlaceEditor is an open item: its controls (children), then Save and
// Cancel. Ctrl or Cmd and Enter saves, Escape cancels; a save with nothing
// changed writes nothing. A save that fails keeps the item open with the
// reason beside it, and one over a newer version opens the conflict
// dialog (EDIT-7). Focus starts in its first control.
export function InPlaceEditor<D extends { id: string; hash: string }, T>({
  item,
  name,
  detail,
  read,
  isDirty,
  send,
  mine,
  theirs = (file) => file,
  readFile,
  check,
  onSaved,
  children,
  className = "",
}: InPlaceEditorProps<D, T>) {
  const draft = useTicketDraft({ detail, read, readFile, isDirty, onSaved });
  const [problem, setProblem] = useState("");
  const frame = useRef<HTMLFieldSetElement>(null);
  const { dirty, busy, value } = draft;

  // biome-ignore lint/correctness/useExhaustiveDependencies: tells the page whenever the draft becomes dirty or clean.
  useEffect(() => item.setDirty(dirty), [dirty]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: registers once, while open.
  useEffect(() => {
    // A control marked data-autofocus (the formatted editor) takes focus
    // before the first field.
    const focus = () =>
      (
        frame.current?.querySelector<HTMLElement>("[data-autofocus]") ??
        frame.current?.querySelector<HTMLElement>("input, textarea, select")
      )?.focus();
    focus();
    item.setFocus(focus);
    return () => item.setFocus(null);
  }, []);

  const save = async () => {
    if (busy) return;
    if (!dirty) {
      item.finish();
      return;
    }
    const wrong = check?.(value);
    setProblem(wrong ?? "");
    if (wrong) return;
    const outcome = await draft.save({
      send: (base) => send(base, value),
      what: "Saved",
      mine: mine(value),
    });
    if (outcome === "saved") item.finish();
  };
  const onKeyDown = (event: KeyboardEvent) => {
    // A key a control inside has handled (Escape closing a menu) is not
    // the item's.
    if (event.defaultPrevented) return;
    if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
      event.preventDefault();
      void save();
    } else if (event.key === "Escape" && !busy) {
      event.preventDefault();
      event.stopPropagation();
      item.finish();
    }
  };
  const error = problem || draft.error;

  return (
    <fieldset
      ref={frame}
      onKeyDown={onKeyDown}
      className={`flex min-w-0 flex-col gap-2 rounded-card border border-rule bg-well p-3 ${className}`}
    >
      <legend className="sr-only">Editing {name}</legend>
      {children(value, draft.set)}
      {error ? (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      ) : null}
      {/* While saving, Save and Cancel stay focusable (aria-disabled), so
          focus is not lost when the save fails. */}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="primary"
          className="h-8 py-0 text-xs aria-disabled:cursor-wait aria-disabled:opacity-50"
          onClick={() => void save()}
          aria-disabled={busy || undefined}
        >
          {busy ? "Saving…" : "Save"}
        </Button>
        <Button
          variant="quiet"
          className="h-8 py-0 text-xs aria-disabled:opacity-50"
          onClick={() => {
            if (!busy) item.finish();
          }}
          aria-disabled={busy || undefined}
        >
          Cancel
        </Button>
        <span className="text-xs text-ink-muted">{shortcuts}</span>
      </div>
      {draft.conflict ? (
        <ConflictDialog
          id={detail.id}
          mine={draft.conflict.mine}
          current={theirs(draft.conflict.current.content)}
          onCancel={draft.cancel}
          onReload={() => {
            draft.reload();
            item.finish();
          }}
          onOverwrite={() =>
            void draft.overwrite().then((outcome) => {
              if (outcome === "saved") item.finish();
            })
          }
        />
      ) : null}
    </fieldset>
  );
}
