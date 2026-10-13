import { type Conflict, conflictOf, type Saved } from "../../api/edit";

// SaveOutcome is how a save under the content-hash precondition ended:
// saved, refused because the file changed since base (a 409 carrying the
// current file, EDIT-7), or failed for another reason, as thrown.
export type SaveOutcome =
  | { kind: "saved"; saved: Saved }
  | { kind: "conflict"; conflict: Conflict }
  | { kind: "failed"; error: unknown };

// saveWithBase is the one way the browser saves a ticket edit (STO-3): it
// sends base, the hash of the version the edit was made on, and sorts the
// result. Callers decide what each outcome shows.
export async function saveWithBase(
  base: string,
  send: (base: string) => Promise<Saved>,
): Promise<SaveOutcome> {
  try {
    return { kind: "saved", saved: await send(base) };
  } catch (error) {
    const conflict = conflictOf(error);
    return conflict
      ? { kind: "conflict", conflict }
      : { kind: "failed", error };
  }
}
