import { useRef, useState } from "react";

import { Button } from "../../components/Button";
import { Icon } from "../../components/Icon";
import { KeyBadge } from "../../components/KeyBadge";
import { Popover } from "../../components/Popover";
import { messageOf } from "../../model/errors";

interface ProjectImageMenuProps {
  project: { displayName: string; key: string; image?: string };
  // onSet sends a chosen file to the server as the project's image; it
  // rejects with the server's words when the file is refused.
  onSet(file: File): Promise<void>;
  // onClear removes the project's image.
  onClear(): Promise<void>;
  // onDone announces a saved change (the shell's toast).
  onDone(text: string): void;
}

// ProjectImageMenu is the page header's project badge as a control
// (FH-151): the key badge, or the project's image, opens a menu to choose
// a JPEG or PNG for the project, replace it or remove it. A refusal (not
// an image, too large) is shown in the menu and leaves the badge as it was.
export function ProjectImageMenu({
  project,
  onSet,
  onClear,
  onDone,
}: ProjectImageMenuProps) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState<"saving" | "removing">();
  const [error, setError] = useState<string>();
  const run = async (
    work: () => Promise<void>,
    kind: "saving" | "removing",
    done: string,
    close: () => void,
  ) => {
    setBusy(kind);
    setError(undefined);
    try {
      await work();
      close();
      onDone(done);
    } catch (failure) {
      setError(messageOf(failure, "The image could not be saved."));
    } finally {
      setBusy(undefined);
    }
  };
  return (
    <Popover
      label={`Project image: ${project.key}`}
      iconOnly
      variant="badge"
      align="start"
      button={
        <KeyBadge size="lg" image={project.image}>
          {project.key}
        </KeyBadge>
      }
    >
      {(close) => (
        <div className="flex w-64 max-w-[calc(100vw-2rem)] flex-col gap-2 p-1.5 text-xs">
          <p className="text-ink-muted">
            {project.image
              ? `${project.displayName}'s badge shows this image; its key is ${project.key}.`
              : `${project.displayName}'s badge shows its key, ${project.key}. Choose a JPEG or PNG to show instead; it is cropped to a square.`}
          </p>
          <div className="flex flex-wrap gap-2">
            <Button
              className="py-1 text-xs"
              disabled={busy !== undefined}
              onClick={() => input.current?.click()}
            >
              <Icon name="image" size={14} />
              {project.image ? "Replace image…" : "Choose image…"}
            </Button>
            {project.image ? (
              <Button
                variant="quiet"
                className="py-1 text-xs"
                disabled={busy !== undefined}
                onClick={() =>
                  void run(onClear, "removing", "Project image removed.", close)
                }
              >
                <Icon name="trash" size={14} />
                Remove image
              </Button>
            ) : null}
          </div>
          <input
            ref={input}
            type="file"
            accept="image/png,image/jpeg"
            hidden
            tabIndex={-1}
            aria-hidden="true"
            onChange={(event) => {
              const file = event.target.files?.[0];
              event.target.value = "";
              if (file)
                void run(
                  () => onSet(file),
                  "saving",
                  "Project image updated.",
                  close,
                );
            }}
          />
          {busy ? (
            <p role="status" className="text-ink-muted">
              {busy === "saving" ? "Saving the image…" : "Removing the image…"}
            </p>
          ) : null}
          {error ? (
            <p role="alert" className="text-danger">
              {error}
            </p>
          ) : null}
        </div>
      )}
    </Popover>
  );
}
