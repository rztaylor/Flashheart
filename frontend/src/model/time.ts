// runningTime formats the age of a timestamp as a compact running time.
export function runningTime(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "";
  const seconds = Math.max(0, (now.getTime() - then) / 1000);
  if (seconds < 60) return "now";
  const minutes = seconds / 60;
  if (minutes < 60) return `${Math.floor(minutes)}m`;
  const hours = minutes / 60;
  if (hours < 24) return `${Math.floor(hours)}h`;
  const days = hours / 24;
  if (days < 14) return `${Math.floor(days)}d`;
  const weeks = days / 7;
  if (weeks < 52) return `${Math.floor(weeks)}w`;
  return `${Math.floor(days / 365)}y`;
}

const plural = (count: number, unit: string) =>
  `${count} ${unit}${count === 1 ? "" : "s"}`;

// durationWords says how long ago a timestamp was in roomy words for the
// Overview ("4 min", "3 h", "2 days"); empty for missing or invalid input.
export function durationWords(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "";
  const minutes = Math.max(0, (now.getTime() - then) / 60_000);
  if (minutes < 1) return "under a minute";
  if (minutes < 60) return `${Math.floor(minutes)} min`;
  const hours = minutes / 60;
  if (hours < 24) return `${Math.floor(hours)} h`;
  const days = hours / 24;
  if (days < 14) return plural(Math.floor(days), "day");
  if (days < 364) return plural(Math.floor(days / 7), "week");
  return plural(Math.floor(days / 365), "year");
}

export function absoluteTime(iso: string): string {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "";
  return new Date(then).toLocaleString([], {
    dateStyle: "medium",
    timeStyle: "short",
  });
}
