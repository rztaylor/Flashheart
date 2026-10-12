import { useState } from "react";
import type { Attachment } from "../../api/board";
import { Lightbox } from "../../components/Lightbox";
import { AttachmentGrid, isImage } from "./AttachmentGrid";

// Attachments shows a set of a ticket's files in the grid, opening
// screenshots in the lightbox (REV-1, ui-layout.md §3).
export function Attachments({ attachments }: { attachments: Attachment[] }) {
  const [open, setOpen] = useState<number | null>(null);
  const images = attachments.filter(isImage);
  return (
    <>
      <AttachmentGrid attachments={attachments} onOpen={setOpen} />
      {open !== null ? (
        <Lightbox
          items={images.map((image) => ({
            src: image.url,
            caption: image.caption,
            name: image.file,
          }))}
          index={open}
          onIndex={setOpen}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </>
  );
}
