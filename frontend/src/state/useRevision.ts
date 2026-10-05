import { useEffect, useState } from "react";

import type { AuthenticatedFetch } from "../api/client";
import { waitForChanges } from "../api/edit";

// RETRY_MS is how long to wait before long-polling again after an error.
export const RETRY_MS = 2_000;

// useRevision long-polls the board revision (LIFE-3) and returns the latest
// one, so views reload when the files change rather than on a timer.
export function useRevision(
  fetcher: AuthenticatedFetch,
  enabled: boolean,
): number {
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    let since = 0;
    const loop = async () => {
      while (!controller.signal.aborted) {
        try {
          const next = await waitForChanges(fetcher, since, controller.signal);
          if (next !== since) {
            since = next;
            setRevision(next);
          }
        } catch {
          if (controller.signal.aborted) return;
          await new Promise((resolve) => window.setTimeout(resolve, RETRY_MS));
        }
      }
    };
    void loop();
    return () => controller.abort();
  }, [fetcher, enabled]);
  return revision;
}
