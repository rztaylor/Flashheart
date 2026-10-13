import {
  type DragEvent,
  type ReactNode,
  type Ref,
  useRef,
  useState,
} from "react";

import { type UploadResult, uploadedWords } from "../model/uploads";
import { Button } from "./Button";
import { Icon } from "./Icon";

// Taking files from the person (FH-18): the Attach files button, the
// keyboard's way to what paste and drop do, an area files can be dropped
// on, and the notes that say why a file was not taken. They hand over
// File objects; what happens to them is the caller's.

// hasFiles reports a drag that carries files (not text or a card).
export function hasFiles(data: DataTransfer | null): boolean {
  return !!data && Array.from(data.types).includes("Files");
}

// AttachButton, "Attach files", opens the system file picker; the chosen
// files go to onFiles. buttonRef lets the caller return focus to it.
export function AttachButton({
  onFiles,
  disabled,
  buttonRef,
}: {
  onFiles(files: File[]): void;
  disabled?: boolean;
  buttonRef?: Ref<HTMLButtonElement>;
}) {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <Button
        ref={buttonRef}
        disabled={disabled}
        onClick={() => input.current?.click()}
        className="py-1 text-xs"
      >
        <Icon name="attachment" size={14} />
        Attach files
      </Button>
      <input
        ref={input}
        type="file"
        multiple
        hidden
        tabIndex={-1}
        aria-hidden="true"
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (files.length) onFiles(files);
        }}
      />
    </>
  );
}

// FileDropArea takes files dropped anywhere on it, showing that it will
// (data-dropping, for the caller's look) while files are dragged over it.
// A drop something inside already handled (defaultPrevented) is left.
export function FileDropArea({
  onFiles,
  className,
  children,
}: {
  onFiles(files: File[]): void;
  className?: string;
  children: ReactNode;
}) {
  const [dropping, setDropping] = useState(false);
  // depth counts dragenter over dragleave, which fire for every child.
  const depth = useRef(0);
  const reset = () => {
    depth.current = 0;
    setDropping(false);
  };
  const over = (event: DragEvent) => {
    if (!hasFiles(event.dataTransfer)) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = "copy";
  };
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: a drop target for the pointer; AttachButton is its keyboard way.
    <div
      data-dropping={dropping || undefined}
      className={className}
      onDragEnter={(event) => {
        if (!hasFiles(event.dataTransfer)) return;
        depth.current += 1;
        setDropping(true);
      }}
      onDragOver={over}
      onDragLeave={() => {
        depth.current = Math.max(0, depth.current - 1);
        if (depth.current === 0) setDropping(false);
      }}
      onDrop={(event) => {
        reset();
        if (event.defaultPrevented) return;
        const files = Array.from(event.dataTransfer?.files ?? []);
        if (!files.length) return;
        event.preventDefault();
        onFiles(files);
      }}
    >
      {children}
    </div>
  );
}

// useUploadNotes keeps what UploadNotes shows for a batch of files: start
// as it is sent, finish with each file's result.
export function useUploadNotes() {
  const [status, setStatus] = useState("");
  const [refusals, setRefusals] = useState<string[]>([]);
  return {
    status,
    refusals,
    start(count: number) {
      setRefusals([]);
      setStatus(
        count === 1 ? "Uploading 1 file…" : `Uploading ${count} files…`,
      );
    },
    finish(results: UploadResult[]) {
      setStatus(uploadedWords(results));
      setRefusals(results.flatMap((result) => result.refusal ?? []));
    },
    dismiss() {
      setRefusals([]);
    },
  };
}

// UploadNotes says what happened to the files just given: a quiet line
// for those in progress or attached, then each refusal in danger words
// (an alert). Dismiss clears the refusals; onDismiss also puts focus back
// where the person was working, since the button goes with them.
export function UploadNotes({
  refusals,
  status,
  onDismiss,
}: {
  refusals: string[];
  status: string;
  onDismiss(): void;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <p role="status" className="text-xs text-ink-muted">
        {status}
      </p>
      {refusals.length > 0 ? (
        <div
          role="alert"
          className="flex min-w-0 items-start gap-2 text-sm text-danger"
        >
          <Icon name="warning" size={14} className="mt-1 shrink-0" />
          <ul className="min-w-0 flex-1 break-words">
            {refusals.map((refusal) => (
              <li key={refusal}>{refusal}</li>
            ))}
          </ul>
          <button
            type="button"
            aria-label="Dismiss"
            title="Dismiss"
            onClick={onDismiss}
            className="grid size-6 shrink-0 place-items-center rounded-control text-danger transition-colors hover:bg-danger-surface"
          >
            <Icon name="close" size={12} />
          </button>
        </div>
      ) : null}
    </div>
  );
}
