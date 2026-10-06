import { useEffect, useState } from "react";

// useNow returns the current time, ticking every 30 seconds, so running
// times ("2m") stay current between board revisions: a quiet agent writes no
// events, so nothing else would re-render them.
export function useNow(intervalMs = 30_000): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), intervalMs);
    return () => window.clearInterval(timer);
  }, [intervalMs]);
  return now;
}
