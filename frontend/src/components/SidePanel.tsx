import { type ReactNode, useEffect, useRef } from "react";

import { Icon } from "./Icon";

interface SidePanelProps {
  label: string;
  onClose(): void;
  // actions sit beside the close button (the card panel's full page link).
  actions?: ReactNode;
  children: ReactNode;
}

// SidePanel sits beside the board on desktop, which narrows to make room and
// stays scrollable (CARD-1); on narrow screens it covers the view. It is not
// modal: focus moves in on open, Escape closes it and the caller restores
// focus.
export function SidePanel({
  label,
  onClose,
  actions,
  children,
}: SidePanelProps) {
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
      className="panel-enter relative flex min-h-0 flex-col border-l border-rule bg-panel shadow-panel outline-none max-md:fixed max-md:inset-x-0 max-md:top-14 max-md:bottom-0 max-md:z-30 md:w-[clamp(26rem,32vw,35rem)]"
    >
      <div className="absolute top-4 right-4 z-10 flex items-center gap-1">
        {actions}
        <button
          type="button"
          onClick={onClose}
          aria-label="Close ticket"
          title="Close (Esc)"
          className="grid size-9 place-items-center rounded-full text-ink-muted transition-colors hover:bg-well hover:text-ink"
        >
          <Icon name="close" />
        </button>
      </div>
      {children}
    </aside>
  );
}
