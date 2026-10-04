import { type AuthenticatedFetch, getJSON, isRecord } from "./client";

export type ThemePreference = "system" | "light" | "dark";

export interface ServerInfo {
  name: "Flashheart";
  version: string;
  commit: string;
  buildDate: string;
  protocolVersion: number;
  root: string;
  theme: ThemePreference;
}

export function fetchInfo(
  fetcher: AuthenticatedFetch,
  signal?: AbortSignal,
): Promise<ServerInfo> {
  return getJSON(
    fetcher,
    "/api/info",
    isServerInfo,
    "Server info response was invalid",
    signal,
  );
}

function isServerInfo(value: unknown): value is ServerInfo {
  return (
    isRecord(value) &&
    value.name === "Flashheart" &&
    typeof value.version === "string" &&
    typeof value.commit === "string" &&
    typeof value.buildDate === "string" &&
    typeof value.protocolVersion === "number" &&
    typeof value.root === "string" &&
    (value.theme === "system" ||
      value.theme === "light" ||
      value.theme === "dark")
  );
}
