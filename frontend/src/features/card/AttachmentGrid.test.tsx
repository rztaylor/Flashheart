import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Attachment } from "../../api/board";
import { Lightbox } from "../../components/Lightbox";
import { AttachmentGrid, isImage } from "./AttachmentGrid";

const shot = (file: string, caption: string): Attachment => ({
  file,
  caption,
  kind: "screenshot",
  run: "claude:s",
  added: "2026-10-06T10:00:00Z",
  url: `/api/projects/fh/tickets/FH-1/files/${file}`,
});
const files: Attachment[] = [
  shot("a.png", "Board, light"),
  { ...shot("run.log", "Test run"), kind: "log" },
  shot("b.png", ""),
];

describe("AttachmentGrid", () => {
  it("opens screenshots in the lightbox and links other files", () => {
    const html = renderToStaticMarkup(
      <AttachmentGrid attachments={files} onOpen={() => undefined} />,
    );
    expect(html).toContain('aria-label="Open Board, light"');
    expect(html).toContain('aria-label="Open b.png"');
    expect(html).toContain(
      'href="/api/projects/fh/tickets/FH-1/files/run.log"',
    );
    expect(html).toContain("Test run");
    expect(html).toContain(">log<");
  });

  it("knows an image by kind or type", () => {
    expect(isImage(files[0] as Attachment)).toBe(true);
    expect(isImage(files[1] as Attachment)).toBe(false);
    expect(
      isImage({ ...(files[1] as Attachment), file: "x.webp", kind: "other" }),
    ).toBe(true);
  });
});

describe("Lightbox", () => {
  it("shows one image large with its caption and place", () => {
    const images = [files[0], files[2]] as Attachment[];
    const html = renderToStaticMarkup(
      <Lightbox
        items={images.map((item) => ({
          src: item.url,
          caption: item.caption,
          name: item.file,
        }))}
        index={1}
        onIndex={() => undefined}
        onClose={() => undefined}
      />,
    );
    expect(html).toContain('src="/api/projects/fh/tickets/FH-1/files/b.png"');
    expect(html).toContain("2 of 2");
    expect(html).toContain(">Previous<");
    expect(html).toContain("Next");
  });
});
