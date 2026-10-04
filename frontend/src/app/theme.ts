import { useEffect } from "react";

import type { ThemePreference } from "../api/info";

export type ResolvedTheme = "light" | "dark";

const darkQuery = "(prefers-color-scheme: dark)";

export function resolveTheme(
  preference: ThemePreference | undefined,
  systemPrefersDark: boolean,
): ResolvedTheme {
  if (preference === "light" || preference === "dark") return preference;
  return systemPrefersDark ? "dark" : "light";
}

export function applyTheme(preference: ThemePreference | undefined) {
  const prefersDark = window.matchMedia(darkQuery).matches;
  document.documentElement.dataset.theme = resolveTheme(
    preference,
    prefersDark,
  );
}

// useTheme keeps <html data-theme> in step with the saved preference and,
// for "system", with the operating system setting.
export function useTheme(preference: ThemePreference | undefined) {
  useEffect(() => {
    applyTheme(preference);
    const media = window.matchMedia(darkQuery);
    const onChange = () => applyTheme(preference);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [preference]);
}
