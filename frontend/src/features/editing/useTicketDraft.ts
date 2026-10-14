import {
  type Dispatch,
  type SetStateAction,
  useEffect,
  useRef,
  useState,
} from "react";

import type { Conflict, Saved } from "../../api/edit";
import { messageOf } from "../../model/errors";
import { guardUnloading } from "../../state/leaveGuard";
import { type SaveOutcome, saveWithBase } from "./precondition";

// DraftSave is one save of a draft.
export interface DraftSave {
  // send writes the draft with base as its precondition hash: the draft's
  // base, or the current file's after Overwrite.
  send(base: string): Promise<Saved>;
  // what names the save for onSaved ("Saved", "Saved the file").
  what: string;
  // mine is the draft as text, shown as "Your version" on a conflict.
  mine: string;
}

// DraftConflict is a save that lost a race (EDIT-7): what was meant to be
// saved beside the file on disk now.
export interface DraftConflict {
  mine: string;
  current: Conflict;
}

// DraftLeft is a draft whose save failed or met a conflict after it was
// unmounted (its item closed, the panel or page left; FH-156): its base,
// its value as last edited, and why it was not saved, so its item can take
// it back as it was (restore).
export interface DraftLeft<T> {
  base: { hash: string; value: T };
  value: T;
  // error is why the save failed; "" when it met a conflict.
  error: string;
  conflict: (DraftConflict & { request: DraftSave }) | null;
}

export interface TicketDraft<T> {
  // base is the version the draft started from: its hash and value.
  base: { hash: string; value: T };
  value: T;
  set: Dispatch<SetStateAction<T>>;
  // dirty: the draft differs from its base.
  dirty: boolean;
  busy: boolean;
  // error is why the last save failed, other than a conflict; "" if none.
  error: string;
  conflict: DraftConflict | null;
  // save resolves to how it ended: saved, a pending conflict, or failed
  // (error says why).
  save(request: DraftSave): Promise<SaveOutcome["kind"]>;
  // overwrite saves the conflicting draft again on the current file's hash.
  overwrite(): Promise<SaveOutcome["kind"]>;
  // reload drops the draft for the file now, and takes the next version.
  reload(): void;
  // cancel closes the conflict and keeps the draft.
  cancel(): void;
  // restore takes back a draft left by a late failure.
  restore(left: DraftLeft<T>): void;
}

// useTicketDraft owns an edit of a ticket under the content-hash
// precondition (STO-3, EDIT-7). The draft is read from the ticket (read)
// and measured against the version it started from (isDirty). It follows
// the file while it is clean, keeps its edit and base when the file changes
// under it, adopts the saved version after a save, and turns a conflict
// into a pending choice: overwrite, reload or cancel. onSaved hears each
// save (and a reload, with no result) so the caller can reload and
// announce it. A save in flight keeps the browser's prompt on a reload or
// closed tab (state/leaveGuard); one that fails or meets a conflict after
// the draft was unmounted goes to left, if given, with the draft as it was
// (FH-156), and one that succeeds is still heard by onSaved.
export function useTicketDraft<D extends { hash: string }, T>({
  detail,
  read,
  readFile,
  isDirty,
  onSaved,
  left,
  initial,
}: {
  detail: D;
  read(detail: D): T;
  // readFile reads the draft's part from a file's text. With it, a save
  // that conflicts with a version the page had not heard of yet, which
  // left that part as it was (an agent's checkpoint during a section
  // edit), is saved again on that version once, without asking (FH-16).
  readFile?(content: string): T;
  isDirty(draft: T, base: T): boolean;
  onSaved(saved: Saved | undefined, what: string): void;
  left?(draft: DraftLeft<T>): void;
  // initial starts the draft as a late failure left it.
  initial?: DraftLeft<T>;
}): TicketDraft<T> {
  // base is the version of the file the draft started from. Edits are
  // measured against it and saved with its hash, so a change on disk since
  // then is a conflict, never a silent overwrite.
  const [base, setBase] = useState(
    () => initial?.base ?? { hash: detail.hash, value: read(detail) },
  );
  const [value, set] = useState(initial?.value ?? base.value);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(initial?.error ?? "");
  const [pending, setPending] = useState<
    (DraftConflict & { request: DraftSave }) | null
  >(initial?.conflict ?? null);
  // adopt takes the next version of the file after a save or a reload.
  const adopt = useRef(false);
  const latest = useRef(detail);
  latest.current = detail;
  // now is the draft and its callers as last rendered, for a save that
  // answers after the draft was unmounted.
  const now = useRef({ base, value, onSaved, left });
  now.current = { base, value, onSaved, left };
  const mounted = useRef(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const dirty = isDirty(value, base.value);

  const reset = (from: D) => {
    const next = { hash: from.hash, value: read(from) };
    setBase(next);
    set(next.value);
  };
  // Follow the file while nothing is being edited, and after a save. While
  // editing, a new version that left the draft's own part as it was (an
  // agent's note under a section being edited) becomes the base, so only a
  // change to what the draft edits is a conflict (FH-66).
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs when the file itself changes.
  useEffect(() => {
    if (detail.hash === base.hash) return;
    if (adopt.current || !dirty) {
      adopt.current = false;
      reset(detail);
    } else if (!isDirty(read(detail), base.value))
      setBase({ hash: detail.hash, value: base.value });
  }, [detail.hash]);

  const run = async (
    request: DraftSave,
    from: string,
    retried = false,
  ): Promise<SaveOutcome["kind"]> => {
    setBusy(true);
    setError("");
    const saving = guardUnloading();
    const outcome = await saveWithBase(from, request.send).finally(saving);
    if (
      outcome.kind === "conflict" &&
      readFile &&
      !retried &&
      !isDirty(readFile(outcome.conflict.content), base.value)
    )
      return run(request, outcome.conflict.hash, true);
    if (!mounted.current) {
      // The person moved on while the save was in flight (FH-156).
      const { base: from, value: last, onSaved: heard, left } = now.current;
      if (outcome.kind === "saved") heard(outcome.saved, request.what);
      else
        left?.({
          base: from,
          value: last,
          error:
            outcome.kind === "failed"
              ? messageOf(outcome.error, "Not saved")
              : "",
          conflict:
            outcome.kind === "conflict"
              ? { request, mine: request.mine, current: outcome.conflict }
              : null,
        });
      return outcome.kind;
    }
    if (outcome.kind === "saved") {
      // Live updates may already have brought the saved version.
      if (latest.current.hash === outcome.saved.hash) reset(latest.current);
      else adopt.current = true;
      onSaved(outcome.saved, request.what);
    } else if (outcome.kind === "conflict") {
      setPending({
        request,
        mine: request.mine,
        current: outcome.conflict,
      });
    } else setError(messageOf(outcome.error, "Not saved"));
    setBusy(false);
    return outcome.kind;
  };

  return {
    base,
    value,
    set,
    dirty,
    busy,
    error,
    conflict: pending ? { mine: pending.mine, current: pending.current } : null,
    save: (request) => run(request, base.hash),
    overwrite: () => {
      if (!pending) return Promise.resolve("conflict" as const);
      setPending(null);
      return run(pending.request, pending.current.hash);
    },
    reload: () => {
      setPending(null);
      // Take the version on disk now, and any newer one the reload brings.
      reset(latest.current);
      adopt.current = true;
      onSaved(undefined, "Reloaded");
    },
    cancel: () => setPending(null),
    restore: (draft) => {
      setBase(draft.base);
      set(draft.value);
      setError(draft.error);
      setPending(draft.conflict);
    },
  };
}
