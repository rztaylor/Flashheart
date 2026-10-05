import { useCallback, useEffect, useRef, useState } from "react";

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

// usePreferences loads the saved UI preferences once and saves changes
// shortly after they are made (CFG-2). Until they load, defaults apply; a
// read-only server keeps the defaults in memory.
export function usePreferences(fetcher: AuthenticatedFetch, enabled: boolean) {
  const [preferences, setPreferences] =
    useState<Preferences>(defaultPreferences);
  const [loaded, setLoaded] = useState(false);
  const latest = useRef<Preferences>(defaultPreferences);
  const timer = useRef<number | undefined>(undefined);
  const saving = useRef(true);

  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    fetchPreferences(fetcher, controller.signal)
      .then((saved) => {
        latest.current = saved;
        setPreferences(saved);
        setLoaded(true);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        saving.current = false;
        setLoaded(true);
      });
    return () => controller.abort();
  }, [fetcher, enabled]);

  const update = useCallback(
    (change: (current: Preferences) => Preferences) => {
      const next = change(latest.current);
      if (next === latest.current) return;
      latest.current = next;
      setPreferences(next);
      if (!saving.current) return;
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => {
        void savePreferences(fetcher, latest.current).catch(() => undefined);
      }, SAVE_DELAY_MS);
    },
    [fetcher],
  );

  return { preferences, loaded, update };
}
