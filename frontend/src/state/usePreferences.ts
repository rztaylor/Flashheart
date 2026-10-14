import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { AuthenticatedFetch } from "../api/client";
import {
  defaultPreferences,
  fetchPreferences,
  type Preferences,
  preferencesPatch,
  savePreferences,
} from "../api/preferences";

// SAVE_DELAY_MS coalesces a burst of changes (typing in a filter) into one
// save.
export const SAVE_DELAY_MS = 400;

// PageVisibility is the part of document the saver watches.
export interface PageVisibility extends EventTarget {
  readonly visibilityState: DocumentVisibilityState;
}

// preferenceSaver saves preferences through the authenticated fetcher
// SAVE_DELAY_MS after the last change, as a patch of what changed from the
// server's copy (known, then each save; FH-173). While it watches the page,
// a pending save is sent at once, kept alive, when the page is hidden or
// unloaded (pagehide, or visibilitychange to hidden), so a change made just
// before a reload or tab close is not lost (FH-48). Nothing pending,
// nothing sent. onIdle hears each save end with nothing pending or still
// being sent.
export function preferenceSaver(
  fetcher: AuthenticatedFetch,
  onIdle: () => void = () => undefined,
) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pending: Preferences | undefined;
  // saved is the server's copy as far as this page knows; undefined after
  // a failed save, so the next one sends the whole copy.
  let saved: Preferences | undefined;
  let sending = 0;

  const send = (keepalive: boolean) => {
    clearTimeout(timer);
    timer = undefined;
    const preferences = pending;
    pending = undefined;
    if (preferences === undefined) return;
    const patch = preferencesPatch(saved, preferences);
    saved = preferences;
    if (patch === undefined) return;
    sending += 1;
    // A refused kept-alive save (a body over the browser's 64 KiB keepalive
    // cap) is retried as an ordinary one, which a hidden page still sends.
    void savePreferences(fetcher, patch, { keepalive })
      .catch((error: unknown) => {
        if (keepalive) return savePreferences(fetcher, patch);
        throw error;
      })
      .catch(() => {
        saved = undefined;
      })
      .finally(() => {
        sending -= 1;
        if (sending === 0 && pending === undefined) onIdle();
      });
  };
  const cancel = () => {
    clearTimeout(timer);
    timer = undefined;
    pending = undefined;
  };

  return {
    // known records the server's copy, read from it.
    known(preferences: Preferences) {
      saved = preferences;
    },
    schedule(preferences: Preferences) {
      pending = preferences;
      clearTimeout(timer);
      timer = setTimeout(() => send(false), SAVE_DELAY_MS);
    },
    // busy is whether a save is waiting or being sent.
    busy() {
      return pending !== undefined || sending > 0;
    },
    // watch flushes on hide and unload until the returned stop is called;
    // stopping drops anything still pending.
    watch(page: EventTarget, visibility: PageVisibility) {
      const flush = () => send(true);
      const onVisibility = () => {
        if (visibility.visibilityState === "hidden") flush();
      };
      page.addEventListener("pagehide", flush);
      visibility.addEventListener("visibilitychange", onVisibility);
      return () => {
        page.removeEventListener("pagehide", flush);
        visibility.removeEventListener("visibilitychange", onVisibility);
        cancel();
      };
    },
  };
}

const same = (a: Preferences, b: Preferences) =>
  JSON.stringify(a) === JSON.stringify(b);

// usePreferences loads the saved UI preferences and saves changes shortly
// after they are made (CFG-2). Until they load, defaults apply; a
// read-only server keeps the defaults in memory. refresh reads the
// server's copy again, as the board's revision moves, and takes it when
// nothing of this page's is waiting to be saved or was changed since the
// read began (FH-173): a change made elsewhere, such as an agent's
// workstream rename rewriting the saved filters (FH-170), reaches the
// page. A refresh while a save is waiting is read after the save. refreshed
// counts the refreshes that changed the page's copy.
export function usePreferences(fetcher: AuthenticatedFetch, enabled: boolean) {
  const [preferences, setPreferences] =
    useState<Preferences>(defaultPreferences);
  const [loaded, setLoaded] = useState(false);
  const [refreshed, setRefreshed] = useState(0);
  const latest = useRef<Preferences>(defaultPreferences);
  // Nothing is saved until the saved preferences have loaded, so an early
  // change cannot overwrite them with defaults.
  const saving = useRef(false);
  // changes counts the page's own changes, so a read that began before one
  // is not taken over it.
  const changes = useRef(0);
  // reads counts refreshes, so only the latest one's read is taken.
  const reads = useRef(0);
  // wanted is a refresh held back until the saves end.
  const wanted = useRef(false);
  const onIdle = useRef<() => void>(() => undefined);
  const saver = useMemo(
    () => preferenceSaver(fetcher, () => onIdle.current()),
    [fetcher],
  );

  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    fetchPreferences(fetcher, controller.signal)
      .then((saved) => {
        latest.current = saved;
        saver.known(saved);
        saving.current = true;
        setPreferences(saved);
        setLoaded(true);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setLoaded(true);
      });
    return () => controller.abort();
  }, [fetcher, enabled, saver]);

  useEffect(() => saver.watch(window, document), [saver]);

  const refresh = useCallback(() => {
    if (!saving.current) return;
    if (saver.busy()) {
      wanted.current = true;
      return;
    }
    wanted.current = false;
    const read = ++reads.current;
    const at = changes.current;
    fetchPreferences(fetcher)
      .then((saved) => {
        if (read !== reads.current) return;
        if (changes.current !== at || saver.busy()) {
          // The page changed its copy meanwhile: read again once that
          // change is saved.
          wanted.current = true;
          if (!saver.busy()) onIdle.current();
          return;
        }
        saver.known(saved);
        if (same(saved, latest.current)) return;
        latest.current = saved;
        setPreferences(saved);
        setRefreshed((count) => count + 1);
      })
      .catch(() => undefined);
  }, [fetcher, saver]);

  onIdle.current = () => {
    if (wanted.current) refresh();
  };

  const update = useCallback(
    (change: (current: Preferences) => Preferences) => {
      const next = change(latest.current);
      if (next === latest.current) return;
      latest.current = next;
      changes.current += 1;
      setPreferences(next);
      if (saving.current) saver.schedule(next);
    },
    [saver],
  );

  return { preferences, loaded, update, refresh, refreshed };
}
