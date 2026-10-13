import type { ThemePreference } from "../api/info";
import { Icon } from "../components/Icon";
import { Popover } from "../components/Popover";
import { SegmentedControl } from "../components/SegmentedControl";
import { TailText } from "../components/TailText";
import type { ServerInfoState } from "./useServerInfo";

const themes: { value: ThemePreference; label: string }[] = [
  { value: "system", label: "System" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

// SettingsMenu is the band's gear (FH-149): the app-wide settings, the
// theme choice saved with the other preferences (CFG-2), the board root and
// the version line. Per-view settings stay in the Board's View options.
export function SettingsMenu({
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
    <Popover
      label="Settings"
      iconOnly
      variant="band"
      align="end"
      button={<Icon name="settings" size={18} />}
    >
      {() => (
        // At most the screen's width, less the band's gutters, on a phone.
        <div className="flex w-72 max-w-[calc(100vw-2rem)] flex-col gap-3 p-1.5 text-xs">
          <div className="flex flex-col gap-1.5">
            <span aria-hidden="true" className="font-semibold">
              Theme
            </span>
            <SegmentedControl
              label="Theme"
              value={theme}
              onChange={onTheme}
              options={themes}
            />
          </div>
          <dl className="flex flex-col gap-1.5 border-t border-rule pt-3">
            <dt className="font-semibold">Board root</dt>
            <TailText as="dd" className="font-mono text-ink-muted">
              {root}
            </TailText>
            {info.status === "ready" ? (
              <>
                <dt className="sr-only">Version</dt>
                <dd className="text-ink-muted">
                  {info.info.version} · agent protocol{" "}
                  {info.info.protocolVersion}
                </dd>
              </>
            ) : null}
          </dl>
        </div>
      )}
    </Popover>
  );
}
