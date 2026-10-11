import { Icon } from "../components/Icon";
import { RunStateMark } from "../components/RunState";

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
      className={`flex h-8 items-center gap-1.5 rounded-full bg-attention px-2.5 text-xs font-semibold whitespace-nowrap text-on-attention transition-opacity hover:opacity-90 focus-visible:outline-on-band sm:h-9 sm:px-3.5 sm:text-sm ${
        on ? "shadow-[inset_0_0_0_2px_var(--fh-on-attention)]" : ""
      }`}
    >
      <RunStateMark state="needs-you" size={11} />
      {count}
      <span className="hidden sm:inline">
        {count === 1 ? " needs you" : " need you"}
      </span>
      <span className="sm:hidden">
        {count === 1 ? " agent needs you" : " agents need you"}
      </span>
      {/* The ring alone marks it pressed on a phone, where the band is full. */}
      {on ? <Icon name="close" size={12} className="max-sm:hidden" /> : null}
    </button>
  );
}
