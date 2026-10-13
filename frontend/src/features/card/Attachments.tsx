import { useState } from "react";
import type { Attachment } from "../../api/board";
import { Lightbox } from "../../components/Lightbox";
import { AttachmentGrid, isImage } from "./AttachmentGrid";

// Attachments shows a set of a ticket's files in the grid, opening
// screenshots in the lightbox (REV-1, ui-layout.md §3): the panel's
// Attachments tab, and on the full page the review's Evidence gallery and
// the Files the review does not show.
export function Attachments({
  attachments,
  wide,
}: {
  attachments: Attachment[];
  wide?: boolean;
}) {
  const [open, setOpen] = useState<number | null>(null);
  const images = attachments.filter(isImage);
  return (
    <>
      <AttachmentGrid attachments={attachments} onOpen={setOpen} wide={wide} />
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
