import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";

import type { ToastMessage } from "../../components/Toast";
import { messageOf } from "../../model/errors";

// Late saves (FH-156, D70): a save still waiting for its answer is not
// unsaved work, so the person may leave its item, the card panel or the
// page while it is in flight. When it then fails or meets a conflict and
// its item is no longer open, the edit is held here, in the tab's memory
// only, and a toast says it was not saved, with Back to the edit: that goes
// back to where the edit was made and reopens it with the person's text
// (and the conflict dialog, for a conflict). An in-place item opened again
// by the person takes its held edit too.

// EditPlace is where an edit was made, to go back to it: a ticket's full
// page, the card panel on a tab, or a workstream's page.
export type EditPlace =
  | { kind: "ticket"; id: string }
  | { kind: "panel"; id: string; tab: "comments" | "edit" }
  | { kind: "workstream"; project: string; slug: string };

// LateSave is one edit held after its save failed late.
export interface LateSave {
  place: EditPlace;
  // item is the in-place item or held draft the edit was made in, by its
  // id on its place; draft is true for a held draft (a comment being
  // written, the Edit tab), which takes its edit back itself.
  item: string;
  draft: boolean;
  // name is the item's name, for its unsaved-changes prompt.
  name: string;
  // text says what was not saved and why.
  text: string;
  // edit is the edit as it was left, for its item to take back.
  edit: unknown;
}

interface Held extends LateSave {
  // returning: Back to the edit was pressed and its place has not yet
  // reopened it.
  returning: boolean;
}

export interface LateSaves {
  hold(save: LateSave): void;
  // peek is an item's held edit; forget lets it go once taken (a render
  // peeks, its effect forgets, as React may render twice).
  peek(place: EditPlace, item: string): LateSave | undefined;
  forget(save: LateSave): void;
  // returning lists the edits asked back to place.
  returning(place: EditPlace): LateSave[];
  // answered marks an edit asked back as reopened (or asked about).
  answered(save: LateSave): void;
  listen(listener: () => void): () => void;
}

export const placeKey = (place: EditPlace) =>
  place.kind === "workstream"
    ? `workstream:${place.project}/${place.slug}`
    : `${place.kind}:${place.id}`;

const keyOf = (place: EditPlace, item: string) => `${placeKey(place)}\n${item}`;

// notSaved is a late failure's words: what was not saved, then why, the
// file's changing on disk for a conflict (label names the file).
export function notSaved(
  what: string,
  label: string,
  failure: { error: string } | { conflict: true },
) {
  return `${what}: ${"error" in failure ? failure.error : `${label} changed on disk`}`;
}

// failureWords is a thrown failure's reason for notSaved.
export const failureWords = (error: unknown) => ({
  error: messageOf(error, "the request failed"),
});

const Context = createContext<LateSaves | null>(null);

// LateSavesProvider holds the late saves of everything under it (the app
// shell). notify shows a toast; back goes to a place, asking first while
// the page there has unsaved work (the leave guard), then calls ready,
// which asks the place to reopen the edit.
export function LateSavesProvider({
  notify,
  back,
  children,
}: {
  notify(message: Omit<ToastMessage, "id">): void;
  back(place: EditPlace, ready: () => void): void;
  children: ReactNode;
}) {
  const latest = useRef({ notify, back });
  latest.current = { notify, back };
  const [store] = useState<LateSaves>(() => {
    const held = new Map<string, Held>();
    const listeners = new Set<() => void>();
    const changed = () => {
      for (const listener of listeners) listener();
    };
    const store: LateSaves = {
      hold(save) {
        const entry: Held = { ...save, returning: false };
        held.set(keyOf(save.place, save.item), entry);
        latest.current.notify({
          text: save.text,
          action: {
            label: "Back to the edit",
            icon: "edit",
            run: () =>
              latest.current.back(save.place, () => {
                // A later failure of the same item replaced this one: the
                // newer edit is the one to go back to.
                const now = held.get(keyOf(save.place, save.item));
                if (!now) return;
                now.returning = true;
                changed();
              }),
          },
        });
      },
      peek: (place, item) => held.get(keyOf(place, item)),
      forget(save) {
        const key = keyOf(save.place, save.item);
        if (held.get(key) === save) held.delete(key);
      },
      returning: (place) =>
        [...held.values()].filter(
          (entry) =>
            entry.returning && placeKey(entry.place) === placeKey(place),
        ),
      answered(save) {
        const entry = held.get(keyOf(save.place, save.item));
        if (entry === save) entry.returning = false;
      },
      listen(listener) {
        listeners.add(listener);
        return () => {
          listeners.delete(listener);
        };
      },
    };
    return store;
  });
  return <Context.Provider value={store}>{children}</Context.Provider>;
}

// useLateSaves is the late saves above, or null where none are held (a
// view rendered on its own).
export const useLateSaves = () => useContext(Context);

// useReturning calls onReturn whenever edits are asked back to place, and
// once as it mounts: the page opened by Back to the edit.
export function useReturning(
  place: EditPlace | undefined,
  onReturn: (saves: LateSave[]) => void,
) {
  const store = useLateSaves();
  const latest = useRef(onReturn);
  latest.current = onReturn;
  const [asked, setAsked] = useState(0);
  useEffect(() => store?.listen(() => setAsked((count) => count + 1)), [store]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs as it mounts and each time edits are asked back.
  useEffect(() => {
    if (!store || !place) return;
    const saves = store.returning(place);
    if (saves.length > 0) latest.current(saves);
  }, [asked, store, place && placeKey(place)]);
}
