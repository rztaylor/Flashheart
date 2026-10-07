import type { Attachment } from "../../api/board";
import { Icon } from "../../components/Icon";

const imageFile = /\.(png|jpe?g|gif|webp)$/i;

// isImage tells a screenshot (shown as a thumbnail, opened in the
// lightbox) from a file tile.
export function isImage(attachment: Attachment): boolean {
  return attachment.kind === "screenshot" || imageFile.test(attachment.file);
}

// AttachmentGrid is a ticket's files as a two-column grid
// (ui-layout.md §3): screenshots as thumbnails that open the lightbox at
// their place among the images, other files as tiles that open the file,
// each with its caption, file name and kind (REV-1).
export function AttachmentGrid({
  attachments,
  onOpen,
}: {
  attachments: Attachment[];
  // onOpen opens the lightbox at an index among the images.
  onOpen(index: number): void;
}) {
  const images = attachments.filter(isImage);
  return (
    <ul className="grid grid-cols-2 gap-3">
      {attachments.map((attachment) => {
        const name = attachment.caption || attachment.file;
        const label = (
          <>
            <span className="mt-1.5 block text-xs font-medium text-ink">
              {name}
            </span>
            <span className="flex items-center gap-1.5 text-2xs text-ink-faint">
              <span className="truncate font-mono">{attachment.file}</span>
              <span className="shrink-0 rounded-mark border border-rule px-1 leading-4">
                {attachment.kind || "other"}
              </span>
            </span>
          </>
        );
        return (
          <li key={attachment.file} className="min-w-0">
            {isImage(attachment) ? (
              <button
                type="button"
                aria-label={`Open ${name}`}
                onClick={() => onOpen(images.indexOf(attachment))}
                className="group block w-full rounded-card text-left"
              >
                <img
                  src={attachment.url}
                  alt=""
                  loading="lazy"
                  className="aspect-video w-full rounded-card border border-rule bg-well object-cover group-hover:border-ink-muted"
                />
                {label}
              </button>
            ) : (
              <a
                href={attachment.url}
                target="_blank"
                rel="noopener noreferrer"
                className="group block rounded-card"
              >
                <span className="flex aspect-video items-center justify-center rounded-card border border-rule bg-well text-ink-muted group-hover:border-ink-muted">
                  <Icon name="attachment" size={20} />
                </span>
                {label}
              </a>
            )}
          </li>
        );
      })}
    </ul>
  );
}
