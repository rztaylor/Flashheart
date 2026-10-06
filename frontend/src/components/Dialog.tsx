import { type ReactNode, useEffect, useId, useRef } from "react";

import { Icon } from "./Icon";

interface DialogProps {
  title: string;
  onClose(): void;
  children: ReactNode;
  // actions sit at the foot of the dialog, primary last.
  actions?: ReactNode;
  wide?: boolean;
}

// Dialog is a modal native <dialog>: it traps focus, closes on Escape, and
// returns focus to whatever opened it when it unmounts. Used only for steps
// that need a decision before continuing (a blocked move, a save conflict, a
// new ticket).
export function Dialog({
  title,
  onClose,
  children,
  actions,
  wide,
}: DialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  useEffect(() => {
    const dialog = ref.current;
    const opener = document.activeElement;
    dialog?.showModal();
    return () => {
      dialog?.close();
      if (opener instanceof HTMLElement && opener.isConnected) opener.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      // Escape closes only the dialog, not the panel underneath it.
      onKeyDown={(event) => {
        if (event.key !== "Escape") return;
        event.preventDefault();
        event.stopPropagation();
        onClose();
      }}
      className={`m-auto max-h-[min(44rem,90vh)] ${wide ? "w-[min(calc(100vw-2rem),60rem)]" : "w-[min(calc(100vw-2rem),32rem)]"} overflow-hidden rounded-panel border border-rule bg-panel p-0 text-ink shadow-card-hover`}
    >
      <div className="flex max-h-[inherit] flex-col">
        <header className="flex items-start justify-between gap-4 px-6 pt-5">
          <h2 id={titleId} className="text-xl leading-tight display-cut">
            {title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="-mr-2 grid size-8 place-items-center rounded-control text-ink-muted transition-colors hover:bg-well hover:text-ink"
          >
            <Icon name="close" />
          </button>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 pt-3 pb-5 text-sm">
          {children}
        </div>
        {actions ? (
          <footer className="flex flex-wrap justify-end gap-2 border-t border-rule bg-well px-6 py-3">
            {actions}
          </footer>
        ) : null}
      </div>
    </dialog>
  );
}
