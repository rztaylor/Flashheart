import { useId } from "react";

import { Button } from "./Button";
import { Icon } from "./Icon";

// LoadError is a load failure (ui-layout.md §8): what could not be loaded
// and why, in danger ink and announced as an alert, with Try again, which
// is described by the failure it retries. className places it in its view.
export function LoadError({
  what,
  error,
  onRetry,
  className = "",
}: {
  // what names the failure ("The archive could not be loaded"); absent,
  // the error stands alone.
  what?: string;
  error: string;
  onRetry(): void;
  className?: string;
}) {
  const id = useId();
  return (
    <div className={`flex flex-wrap items-center gap-3 text-sm ${className}`}>
      <p id={id} role="alert" className="text-danger">
        {what ? `${what}: ${error}` : error}
      </p>
      <Button aria-describedby={id} onClick={onRetry}>
        <Icon name="refresh" size={14} />
        Try again
      </Button>
    </div>
  );
}
