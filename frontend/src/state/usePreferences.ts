import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { AuthenticatedFetch } from "../api/client";
import {
  defaultPreferences,
  fetchPreferences,
  type Preferences,
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
// SAVE_DELAY_MS after the last change. While it watches the page, a pending
// save is sent at once, kept alive, when the page is hidden or unloaded
// (pagehide, or visibilitychange to hidden), so a change made just before a
// reload or tab close is not lost (FH-48). Nothing pending, nothing sent.
export function preferenceSaver(fetcher: AuthenticatedFetch) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pending: Preferences | undefined;

  const send = (keepalive: boolean) => {
    clearTimeout(timer);
    timer = undefined;
    const preferences = pending;
    pending = undefined;
    if (preferences === undefined) return;
    // A refused kept-alive save (a body over the browser's 64 KiB keepalive
    // cap) is retried as an ordinary one, which a hidden page still sends.
    void savePreferences(fetcher, preferences, { keepalive })
      .catch((error: unknown) => {
        if (keepalive) return savePreferences(fetcher, preferences);
        throw error;
      })
      .catch(() => undefined);
  };
  const cancel = () => {
    clearTimeout(timer);
    timer = undefined;
    pending = undefined;
  };

  return {
    schedule(preferences: Preferences) {
      pending = preferences;
      clearTimeout(timer);
      timer = setTimeout(() => send(false), SAVE_DELAY_MS);
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

// usePreferences loads the saved UI preferences once and saves changes
// shortly after they are made (CFG-2). Until they load, defaults apply; a
// read-only server keeps the defaults in memory.
export function usePreferences(fetcher: AuthenticatedFetch, enabled: boolean) {
  const [preferences, setPreferences] =
    useState<Preferences>(defaultPreferences);
  const [loaded, setLoaded] = useState(false);
  const latest = useRef<Preferences>(defaultPreferences);
  // Nothing is saved until the saved preferences have loaded, so an early
  // change cannot overwrite them with defaults.
  const saving = useRef(false);
  const saver = useMemo(() => preferenceSaver(fetcher), [fetcher]);

  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    fetchPreferences(fetcher, controller.signal)
      .then((saved) => {
        latest.current = saved;
        saving.current = true;
        setPreferences(saved);
        setLoaded(true);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setLoaded(true);
      });
    return () => controller.abort();
  }, [fetcher, enabled]);

  useEffect(() => saver.watch(window, document), [saver]);

  const update = useCallback(
    (change: (current: Preferences) => Preferences) => {
      const next = change(latest.current);
      if (next === latest.current) return;
      latest.current = next;
      setPreferences(next);
      if (saving.current) saver.schedule(next);
    },
    [saver],
  );

  return { preferences, loaded, update };
}
