import { type ReactNode, useEffect, useId, useRef, useState } from "react";

interface PopoverProps {
  // label is the button's accessible name ("Type: 2 chosen").
  label: string;
  // iconOnly also shows label as the button's tooltip.
  iconOnly?: boolean;
  button: ReactNode;
  // active highlights the button, as an applied filter.
  active?: boolean;
  align?: "start" | "end";
  // children receives close, for choices that finish with one pick.
  children(close: () => void): ReactNode;
}

// Popover is a disclosure: a button that shows a panel of controls below it
// (FH-39). Escape, a click outside and tabbing away close it; Escape returns
// focus to the button.
export function Popover({
  label,
  iconOnly,
  button,
  active,
  align = "start",
  children,
}: PopoverProps) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useId();
  useEffect(() => {
    if (!open) return;
    const away = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    // Escape closes it wherever focus is (Safari does not focus a clicked
    // button), and is claimed so the card panel underneath stays open.
    const onEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      event.preventDefault();
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener("pointerdown", away);
    document.addEventListener("keydown", onEscape, true);
    return () => {
      document.removeEventListener("pointerdown", away);
      document.removeEventListener("keydown", onEscape, true);
    };
  }, [open]);
  const close = () => {
    setOpen(false);
    trigger.current?.focus();
  };
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: focus leaving closes the disclosure; the button inside is the control.
    <div
      ref={root}
      className="relative"
      // Tabbing to a control outside closes it; a click on something that
      // takes no focus (a segment's label) blurs with no related target and
      // leaves it open for that click, while the pointerdown handler closes
      // it for clicks outside.
      onBlur={(event) => {
        const next = event.relatedTarget as Node | null;
        if (open && next && !root.current?.contains(next)) setOpen(false);
      }}
    >
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={open ? panel : undefined}
        aria-label={label}
        title={iconOnly ? label : undefined}
        onClick={() => setOpen(!open)}
        className={`flex h-8 items-center gap-1.5 rounded-control border px-2.5 text-xs font-medium whitespace-nowrap transition-colors ${
          active
            ? "border-select bg-select-surface text-ink ring-1 ring-select"
            : open
              ? "border-ink-muted bg-card text-ink"
              : "border-rule bg-card text-ink hover:border-ink-muted"
        }`}
      >
        {button}
      </button>
      {open ? (
        <div
          id={panel}
          className={`absolute top-full z-30 mt-1.5 min-w-48 rounded-control bg-popover p-1.5 shadow-popover ${
            align === "end" ? "right-0" : "left-0"
          }`}
        >
          {children(close)}
        </div>
      ) : null}
    </div>
  );
}
