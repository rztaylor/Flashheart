import { Icon } from "../components/Icon";
import { plateClass, plateMark } from "../components/NeedsYou";
import { RunStateMark } from "../components/RunState";
import { needsYouPhrase } from "../model/runs";

interface NeedsYouPillProps {
  // count is the runs that need you, across every project (SPEC §7).
  count: number;
  // on shows only the tickets that need you (VIEW-2, FH-44).
  on: boolean;
  onToggle(): void;
}

// NeedsYouPill is the band's Needs you plate, visible from every view, and
// the Board and Table's Needs you filter (FH-44): pressed, they show only
// tickets with a run in Needs you or an open question. It stays while on,
// so the filter can be switched off once nothing needs you.
export function NeedsYouPill({ count, on, onToggle }: NeedsYouPillProps) {
  if (count === 0 && !on) return null;
  return (
    <button
      type="button"
      aria-pressed={on}
      title={
        on
          ? "Showing only tickets that need you. Click to show all."
          : "Show only tickets that need you"
      }
      onClick={onToggle}
      className={`${plateClass("band", on)} transition-opacity hover:opacity-90 focus-visible:outline-on-band`}
    >
      <RunStateMark state="needs-you" size={plateMark.band} />
      {count}
      <span className="hidden sm:inline">{` ${needsYouPhrase(count)}`}</span>
      <span className="sm:hidden">{` ${needsYouPhrase(count, true)}`}</span>
      {/* The ring alone marks it pressed on a phone, where the band is full. */}
      {on ? <Icon name="close" size={12} className="max-sm:hidden" /> : null}
    </button>
  );
}
