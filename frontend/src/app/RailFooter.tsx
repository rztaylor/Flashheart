import type { ThemePreference } from "../api/info";
import type { ServerInfoState } from "./useServerInfo";

const themes: { value: ThemePreference; label: string }[] = [
  { value: "system", label: "System" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

// RailFooter shows the board root and version, and the theme choice, saved
// with the other preferences (CFG-2).
export function RailFooter({
  root,
  info,
  theme,
  onTheme,
}: {
  root: string;
  info: ServerInfoState;
  theme: ThemePreference;
  onTheme(theme: ThemePreference): void;
}) {
  return (
    <section
      aria-label="Board root"
      className="border-t border-rail-rule px-4 py-3 text-2xs text-on-rail-muted max-[90rem]:group-data-[panel=open]/work:hidden"
    >
      <fieldset className="mb-2.5 flex items-center gap-2">
        <legend className="sr-only">Theme</legend>
        <span aria-hidden="true">Theme</span>
        <span className="flex rounded-control border border-rail-rule p-0.5">
          {themes.map((option) => (
            <label
              key={option.value}
              className={`cursor-pointer rounded-inner px-2 py-0.5 text-xs transition-colors has-focus-visible:outline-2 has-focus-visible:outline-focus ${
                theme === option.value
                  ? "bg-rail-active font-semibold text-on-rail-active shadow-card"
                  : "text-on-rail-muted hover:text-on-rail"
              }`}
            >
              <input
                type="radio"
                name="theme"
                value={option.value}
                checked={theme === option.value}
                onChange={() => onTheme(option.value)}
                className="sr-only"
              />
              {option.label}
            </label>
          ))}
        </span>
      </fieldset>
      <dl>
        <dt className="sr-only">Board root</dt>
        {/* Truncated from the start so the meaningful tail stays visible. */}
        <dd
          className="truncate text-left font-mono [direction:rtl]"
          title={root}
        >
          <bdi>{root}</bdi>
        </dd>
        {info.status === "ready" ? (
          <>
            <dt className="sr-only">Version</dt>
            <dd className="mt-0.5">
              {info.info.version} · agent protocol {info.info.protocolVersion}
            </dd>
          </>
        ) : null}
      </dl>
    </section>
  );
}
