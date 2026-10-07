import { useEffect } from "react";

import { Button } from "./Button";
import { Dialog } from "./Dialog";
import { Icon } from "./Icon";

export interface LightboxItem {
  src: string;
  caption: string;
  name: string;
}

// Lightbox shows one image of a set large in a modal dialog
// (ui-layout.md §3): its caption, file name and place in the set, with
// Previous and Next, which the arrow keys also press, when there is more
// than one. Escape closes it and focus returns to whatever opened it
// (Dialog).
export function Lightbox({
  items,
  index,
  onIndex,
  onClose,
}: {
  items: LightboxItem[];
  index: number;
  onIndex(index: number): void;
  onClose(): void;
}) {
  const item = items[index];
  const last = items.length - 1;
  useEffect(() => {
    const step = (event: KeyboardEvent) => {
      if (event.key === "ArrowLeft" && index > 0) onIndex(index - 1);
      if (event.key === "ArrowRight" && index < last) onIndex(index + 1);
    };
    window.addEventListener("keydown", step);
    return () => window.removeEventListener("keydown", step);
  }, [index, last, onIndex]);
  if (!item) return null;
  return (
    <Dialog
      title={item.caption || item.name}
      onClose={onClose}
      wide
      actions={
        last > 0 ? (
          <>
            <span className="mr-auto self-center text-xs text-ink-muted tabular-nums">
              {index + 1} of {items.length}
            </span>
            <Button disabled={index === 0} onClick={() => onIndex(index - 1)}>
              <Icon name="next" size={14} className="rotate-180" />
              Previous
            </Button>
            <Button
              disabled={index === last}
              onClick={() => onIndex(index + 1)}
            >
              Next
              <Icon name="next" size={14} />
            </Button>
          </>
        ) : undefined
      }
    >
      <figure className="flex flex-col items-center gap-2">
        <img
          src={item.src}
          alt={item.caption || item.name}
          className="max-h-[60vh] w-auto max-w-full rounded-card border border-rule bg-well object-contain"
        />
        <figcaption className="font-mono text-2xs text-ink-faint">
          {item.name}
        </figcaption>
      </figure>
    </Dialog>
  );
}
