import { useCallback, useEffect, useRef, useState } from "react";

export type Resource<T> =
  | { status: "loading"; data?: undefined; error?: undefined }
  | { status: "ready"; data: T; error?: string }
  | { status: "error"; data?: undefined; error: string };

// useResource loads a value and keeps the last good value while reloading.
// It reloads when the board revision moves (live updates, LIFE-3) and when
// the page becomes visible again. A reload failure keeps the previous data
// and reports the error alongside it.
export function useResource<T>(
  load: ((signal: AbortSignal) => Promise<T>) | undefined,
  key: string,
  revision = 0,
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
  // biome-ignore lint/correctness/useExhaustiveDependencies: key, tick and revision are reload triggers; the loader is read from a ref.
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
  }, [key, tick, enabled, revision]);

  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === "visible") setTick((value) => value + 1);
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, []);

  const reload = useCallback(() => setTick((value) => value + 1), []);
  return { ...state, reload };
}
