import { type ReactNode, useCallback, useEffect, useState } from "react";

import { ApiError } from "../../api/client";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, TextInput } from "../../components/Field";
import { StateNote } from "../../components/StateNote";
import { confirmsDelete } from "../../model/archive";

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The request failed";

// TypedDeleteDialog is the one shape of a permanent delete (EDIT-8, PRJ-5):
// it loads a preview of everything the delete touches, shows it, and arms
// the red button only once the user has typed word. When the server says
// something listed has changed, the preview is reloaded and the typed word
// cleared, so nothing is deleted before the new list has been read.
export function TypedDeleteDialog<Plan>({
  title,
  word,
  hint,
  load,
  remove,
  onClose,
  onDeleted,
  children,
}: {
  title(plan: Plan | null): string;
  // word is what the user types to confirm: the id or the project name.
  word: string;
  hint: string;
  load(signal?: AbortSignal): Promise<Plan>;
  remove(plan: Plan, typed: string): Promise<void>;
  onClose(): void;
  onDeleted(plan: Plan): void;
  children(plan: Plan): ReactNode;
}) {
  const [plan, setPlan] = useState<Plan | null>(null);
  const [error, setError] = useState("");
  const [changed, setChanged] = useState(false);
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(
    (signal?: AbortSignal) =>
      load(signal)
        .then((next) => {
          setPlan(next);
          setError("");
        })
        .catch((reason: unknown) => {
          if (!signal?.aborted) setError(failure(reason));
        }),
    [load],
  );
  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  const ready = !!plan && confirmsDelete(typed, word) && !busy;
  const confirm = async () => {
    if (!plan || !ready) return;
    setBusy(true);
    try {
      await remove(plan, typed.trim());
      onDeleted(plan);
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 409) {
        setChanged(true);
        setTyped("");
        await refresh();
      } else {
        setError(failure(reason));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      title={title(plan)}
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>Keep it archived</Button>
          <Button variant="danger" disabled={!ready} onClick={confirm}>
            {busy ? "Deleting…" : "Delete permanently"}
          </Button>
        </>
      }
    >
      {plan ? (
        <>
          {children(plan)}
          {changed ? (
            <div className="mt-4">
              <StateNote kind="warning">
                Something here changed since you opened this. The list is up to
                date now; read it, then type the name again to delete.
              </StateNote>
            </div>
          ) : null}
          <div className="mt-5">
            <FormField label={`Type ${word} to confirm`} hint={hint}>
              <TextInput
                value={typed}
                onChange={setTyped}
                autoFocus
                spellCheck={false}
              />
            </FormField>
          </div>
        </>
      ) : error ? null : (
        <p className="text-ink-muted">Checking what the delete touches…</p>
      )}
      {error ? (
        <div className="mt-3 flex items-center gap-3">
          <p role="alert" className="text-danger">
            {error}
          </p>
          {plan ? null : (
            <Button onClick={() => void refresh()}>Try again</Button>
          )}
        </div>
      ) : null}
    </Dialog>
  );
}
