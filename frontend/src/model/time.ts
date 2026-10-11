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

// dayAndTime names a past moment by its local day and clock time for the
// Overview's headline ("yesterday 18:00", "Mon 09:12", "20 Sept 09:12");
// empty for missing or invalid input.
export function dayAndTime(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const then = new Date(Date.parse(iso));
  if (Number.isNaN(then.getTime())) return "";
  const clock = then.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
  const day = (date: Date) =>
    new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const days = Math.round((day(now) - day(then)) / 86_400_000);
  if (days <= 0) return `today ${clock}`;
  if (days === 1) return `yesterday ${clock}`;
  if (days < 7) {
    return `${then.toLocaleDateString([], { weekday: "short" })} ${clock}`;
  }
  const date = then.toLocaleDateString([], {
    day: "numeric",
    month: "short",
    year: then.getFullYear() === now.getFullYear() ? undefined : "numeric",
  });
  return `${date} ${clock}`;
}

export function absoluteTime(iso: string): string {
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return "";
  return new Date(then).toLocaleString([], {
    dateStyle: "medium",
    timeStyle: "short",
  });
}
