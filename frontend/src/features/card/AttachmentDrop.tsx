import { type ReactNode, useRef } from "react";

import {
  AttachButton,
  FileDropArea,
  UploadNotes,
  useUploadNotes,
} from "../../components/FileDrop";
import { Icon } from "../../components/Icon";
import { ALLOWED_WORDS, type Upload, uploadAll } from "../../model/uploads";

// AttachmentDrop makes a ticket's files a place to add more (FH-18): the
// panel's Attachments tab and the full page's Files. Files dropped
// anywhere on it, or chosen with Attach files, are stored in the ticket
// one at a time; the ticket's text is not touched. Below the files, a
// dashed slot says so and lights up while files are dragged over.
export function AttachmentDrop({
  upload,
  children,
}: {
  upload: Upload;
  children?: ReactNode;
}) {
  const notes = useUploadNotes();
  const button = useRef<HTMLButtonElement>(null);
  const take = (files: File[]) => {
    notes.start(files.length);
    void uploadAll(files, upload, false).then(notes.finish);
  };
  return (
    <FileDropArea onFiles={take} className="group flex min-w-0 flex-col gap-3">
      {children}
      <div
        data-drop-slot
        className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card border border-dashed border-rule px-4 py-3 transition-colors group-data-dropping:border-solid group-data-dropping:border-select group-data-dropping:bg-select-surface group-data-dropping:ring-2 group-data-dropping:ring-select"
      >
        <Icon name="attachment" size={18} className="shrink-0 text-ink-muted" />
        <div className="min-w-0 flex-1">
          <p className="text-sm text-ink">Drop files here to attach them</p>
          <p className="text-xs text-ink-muted">{ALLOWED_WORDS}</p>
        </div>
        <AttachButton onFiles={take} buttonRef={button} />
      </div>
      <UploadNotes
        refusals={notes.refusals}
        status={notes.status}
        onDismiss={() => {
          notes.dismiss();
          button.current?.focus();
        }}
      />
    </FileDropArea>
  );
}
