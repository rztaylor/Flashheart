import { useEffect } from "react";

import { Button } from "./Button";
import { Icon } from "./Icon";

export interface ToastMessage {
  id: number;
  text: string;
  // detail lines, such as review warnings, shown under the text.
  details?: string[];
  action?: { label: string; run(): void };
}

// An action such as Undo stays long enough to reach by keyboard.
const DISMISS_MS = 8_000;
const ACTION_DISMISS_MS = 20_000;

// Toast announces the result of an edit in a polite live region at the foot
// of the screen, with an optional action such as Undo.
export function Toast({
  message,
  onDismiss,
}: {
  message: ToastMessage | null;
  onDismiss(): void;
}) {
  useEffect(() => {
    if (!message) return;
    const timer = window.setTimeout(
      onDismiss,
      message.action ? ACTION_DISMISS_MS : DISMISS_MS,
    );
    return () => window.clearTimeout(timer);
  }, [message, onDismiss]);
  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed inset-x-0 bottom-4 z-40 flex justify-center px-4"
    >
      {message ? (
        <div
          key={message.id}
          className="toast-enter pointer-events-auto flex max-w-xl items-start gap-3 rounded-panel border border-band-rule bg-band px-4 py-3 text-sm text-on-band shadow-card-hover"
        >
          <div className="min-w-0">
            <p>{message.text}</p>
            {message.details?.map((detail) => (
              <p key={detail} className="mt-0.5 text-xs text-on-band-muted">
                {detail}
              </p>
            ))}
          </div>
          {message.action ? (
            <Button
              variant="band"
              className="h-7 shrink-0 py-0 text-xs"
              onClick={() => {
                message.action?.run();
                onDismiss();
              }}
            >
              <Icon name="undo" size={12} />
              {message.action.label}
            </Button>
          ) : null}
          <button
            type="button"
            onClick={onDismiss}
            aria-label="Dismiss"
            className="-mr-1 grid size-7 shrink-0 place-items-center rounded-control text-on-band-muted hover:text-on-band focus-visible:outline-on-band"
          >
            <Icon name="close" size={14} />
          </button>
        </div>
      ) : null}
    </div>
  );
}
