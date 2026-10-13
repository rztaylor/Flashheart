import {
  type Dispatch,
  type SetStateAction,
  useEffect,
  useRef,
  useState,
} from "react";

import type { Conflict, Saved } from "../../api/edit";
import { messageOf } from "../../model/errors";
import { saveWithBase } from "./precondition";

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
  save(request: DraftSave): Promise<void>;
  // overwrite saves the conflicting draft again on the current file's hash.
  overwrite(): void;
  // reload drops the draft for the file now, and takes the next version.
  reload(): void;
  // cancel closes the conflict and keeps the draft.
  cancel(): void;
}

// useTicketDraft owns an edit of a ticket under the content-hash
// precondition (STO-3, EDIT-7). The draft is read from the ticket (read)
// and measured against the version it started from (isDirty). It follows
// the file while it is clean, keeps its edit and base when the file changes
// under it, adopts the saved version after a save, and turns a conflict
// into a pending choice: overwrite, reload or cancel. onSaved hears each
// save (and a reload, with no result) so the caller can reload and
// announce it.
export function useTicketDraft<D extends { hash: string }, T>({
  detail,
  read,
  isDirty,
  onSaved,
}: {
  detail: D;
  read(detail: D): T;
  isDirty(draft: T, base: T): boolean;
  onSaved(saved: Saved | undefined, what: string): void;
}): TicketDraft<T> {
  // base is the version of the file the draft started from. Edits are
  // measured against it and saved with its hash, so a change on disk since
  // then is a conflict, never a silent overwrite.
  const [base, setBase] = useState(() => ({
    hash: detail.hash,
    value: read(detail),
  }));
  const [value, set] = useState(base.value);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pending, setPending] = useState<
    (DraftConflict & { request: DraftSave }) | null
  >(null);
  // adopt takes the next version of the file after a save or a reload.
  const adopt = useRef(false);
  const latest = useRef(detail);
  latest.current = detail;
  const dirty = isDirty(value, base.value);

  const reset = (from: D) => {
    const next = { hash: from.hash, value: read(from) };
    setBase(next);
    set(next.value);
  };
  // Follow the file while nothing is being edited, and after a save.
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs when the file itself changes.
  useEffect(() => {
    if (detail.hash === base.hash) return;
    if (adopt.current || !dirty) {
      adopt.current = false;
      reset(detail);
    }
  }, [detail.hash]);

  const run = async (request: DraftSave, from: string) => {
    setBusy(true);
    setError("");
    const outcome = await saveWithBase(from, request.send);
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
      if (!pending) return;
      setPending(null);
      void run(pending.request, pending.current.hash);
    },
    reload: () => {
      setPending(null);
      // Take the version on disk now, and any newer one the reload brings.
      reset(latest.current);
      adopt.current = true;
      onSaved(undefined, "Reloaded");
    },
    cancel: () => setPending(null),
  };
}
