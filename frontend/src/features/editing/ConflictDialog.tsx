import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";

// ConflictDialog shows a save that lost a race (EDIT-7): what you meant to
// save beside what is on disk now. Reload keeps the file; Overwrite replaces
// it with your version deliberately.
export function ConflictDialog({
  id,
  mine,
  current,
  onReload,
  onOverwrite,
  onCancel,
}: {
  id: string;
  mine: string;
  current: string;
  onReload(): void;
  onOverwrite(): void;
  onCancel(): void;
}) {
  return (
    <Dialog
      title={`${id} changed while you were editing`}
      onClose={onCancel}
      wide
      actions={
        <>
          <Button onClick={onReload}>Discard mine and reload</Button>
          <Button variant="primary" onClick={onOverwrite}>
            Overwrite with mine
          </Button>
        </>
      }
    >
      <p className="mb-3 text-ink-muted">
        Someone else (an agent, an editor or another tab) saved this ticket
        after you opened it. Nothing has been lost yet.
      </p>
      <div className="grid gap-3 md:grid-cols-2">
        {[
          { label: "Your version", text: mine },
          { label: "Now on disk", text: current },
        ].map((side) => (
          <section key={side.label} aria-label={side.label} className="min-w-0">
            <h3 className="mb-1 text-sm station-sign">{side.label}</h3>
            {/* Focusable so keyboard users can scroll a long file. */}
            <pre
              // biome-ignore lint/a11y/noNoninteractiveTabindex: a scrollable region must be reachable by keyboard.
              tabIndex={0}
              className="max-h-80 overflow-auto rounded-card border border-rule bg-well p-2 font-mono text-xs whitespace-pre-wrap"
            >
              {side.text}
            </pre>
          </section>
        ))}
      </div>
    </Dialog>
  );
}
