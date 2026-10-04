import { type ReactNode, useEffect, useRef } from "react";

import { Icon } from "./Icon";

interface SidePanelProps {
  label: string;
  onClose(): void;
  children: ReactNode;
}

// SidePanel slides over the right of the screen and keeps the board visible
// (CARD-1). It is not modal: focus moves in on open, Escape closes it and
// the caller restores focus.
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
      className="panel-enter fixed top-12 right-0 bottom-0 z-30 flex w-[min(560px,100vw)] flex-col border-l border-rule bg-card shadow-panel outline-none"
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
