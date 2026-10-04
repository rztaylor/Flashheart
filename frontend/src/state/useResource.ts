import { useCallback, useEffect, useRef, useState } from "react";

export type Resource<T> =
  | { status: "loading"; data?: undefined; error?: undefined }
  | { status: "ready"; data: T; error?: string }
  | { status: "error"; data?: undefined; error: string };

// REFRESH_MS is how often visible views re-read the board. It stands in for
// the long-poll live updates that arrive with board-editing (LIFE-3).
export const REFRESH_MS = 10_000;

// useResource loads a value, keeps the last good value while refreshing,
// and refreshes every REFRESH_MS while the page is visible. A refresh failure
// keeps the previous data and reports the error alongside it.
export function useResource<T>(
  load: ((signal: AbortSignal) => Promise<T>) | undefined,
  key: string,
): Resource<T> & { reload(): void } {
  const [state, setState] = useState<Resource<T>>({ status: "loading" });
  const [tick, setTick] = useState(0);
  const loadRef = useRef(load);
  loadRef.current = load;
  const keyRef = useRef(key);

  useEffect(() => {
    if (keyRef.current !== key) {
      keyRef.current = key;
      setState({ status: "loading" });
    }
  }, [key]);

  const enabled = load !== undefined;
  // biome-ignore lint/correctness/useExhaustiveDependencies: key and tick are reload triggers; the loader is read from a ref.
  useEffect(() => {
    const current = loadRef.current;
    if (!enabled || !current) return;
    const controller = new AbortController();
    current(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) setState({ status: "ready", data });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        const message =
          error instanceof Error ? error.message : "Could not load";
        setState((previous) =>
          previous.status === "ready"
            ? { ...previous, error: message }
            : { status: "error", error: message },
        );
      });
    return () => controller.abort();
  }, [key, tick, enabled]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") setTick((value) => value + 1);
    }, REFRESH_MS);
    const onVisible = () => {
      if (document.visibilityState === "visible") setTick((value) => value + 1);
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, []);

  const reload = useCallback(() => setTick((value) => value + 1), []);
  return { ...state, reload };
}
