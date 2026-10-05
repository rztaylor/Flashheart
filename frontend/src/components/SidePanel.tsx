import { type ReactNode, useEffect, useRef } from "react";

import { Icon } from "./Icon";

interface SidePanelProps {
  label: string;
  onClose(): void;
  children: ReactNode;
}

// SidePanel sits beside the board on desktop, which narrows to make room and
// stays scrollable (CARD-1); on narrow screens it covers the view. It is not
// modal: focus moves in on open, Escape closes it and the caller restores
// focus.
export function SidePanel({ label, onClose, children }: SidePanelProps) {
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    ref.current?.focus();
  }, []);
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !event.defaultPrevented) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <aside
      ref={ref}
      tabIndex={-1}
      aria-label={label}
      className="panel-enter relative flex min-h-0 flex-col border-l border-rule bg-card shadow-panel outline-none max-md:fixed max-md:inset-x-0 max-md:top-12 max-md:bottom-0 max-md:z-30 md:w-[clamp(26rem,36vw,35rem)]"
    >
      <button
        type="button"
        onClick={onClose}
        aria-label="Close ticket"
        title="Close (Esc)"
        className="absolute top-3 right-3 z-10 grid size-8 place-items-center rounded-control text-ink-muted transition-colors hover:bg-well hover:text-ink"
      >
        <Icon name="close" />
      </button>
      {children}
    </aside>
  );
}
