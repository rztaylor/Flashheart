// The Overview's headline metrics (VIEW-3, FH-52): typed request and
// response check.
import { type AuthenticatedFetch, getJSON, isRecord } from "./client";

// Metrics count what changed in a scope since `since`: the human's latest
// board activity there when `lastChange` is set, otherwise a day ago. done,
// review and created count distinct tickets; criteriaTicked counts ticket
// updates that ticked criteria.
export interface Metrics {
  revision: number;
  since: string;
  lastChange: boolean;
  done: number;
  review: number;
  created: number;
  criteriaTicked: number;
}

const isCount = (value: unknown): value is number =>
  typeof value === "number" && Number.isInteger(value) && value >= 0;

export function isMetrics(value: unknown): value is Metrics {
  return (
    isRecord(value) &&
    typeof value.revision === "number" &&
    typeof value.since === "string" &&
    typeof value.lastChange === "boolean" &&
    isCount(value.done) &&
    isCount(value.review) &&
    isCount(value.created) &&
    isCount(value.criteriaTicked)
  );
}

// fetchMetrics loads a project's headline metrics, or every project's when
// project is "".
export function fetchMetrics(
  fetcher: AuthenticatedFetch,
  project: string,
  signal?: AbortSignal,
) {
  const suffix = project ? `?${new URLSearchParams({ project })}` : "";
  return getJSON(
    fetcher,
    `/api/metrics${suffix}`,
    isMetrics,
    "Metrics response was invalid",
    signal,
  );
}
