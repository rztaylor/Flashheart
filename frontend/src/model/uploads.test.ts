import { describe, expect, it } from "vitest";

import { ApiError } from "../api/client";
import {
  clipboardFiles,
  MAX_UPLOAD_BYTES,
  referenceFor,
  refusalWords,
  tooLarge,
  type Uploaded,
  uploadAll,
  uploadedWords,
  uploadName,
} from "./uploads";

const png = (name = "shot.png", size = 10) =>
  new File([new Uint8Array(size)], name, { type: "image/png" });

const uploaded = (file: string, image: boolean): Uploaded => ({
  file,
  kind: image ? "screenshot" : "other",
  image,
  reused: false,
});

describe("uploads", () => {
  it("references an image as an image and anything else as a link", () => {
    expect(
      referenceFor(
        uploaded("20261013T1432-screenshot.png", true),
        "Screenshot",
      ),
    ).toBe("![Screenshot](files/20261013T1432-screenshot.png)");
    expect(
      referenceFor(uploaded("20261013T1432-spec.pdf", false), "spec.pdf"),
    ).toBe("[spec.pdf](files/20261013T1432-spec.pdf)");
    // Brackets and backslashes in a name cannot end the text early.
    expect(referenceFor(uploaded("x-a-b.png", true), "a]b[c\\.png")).toBe(
      "![a\\]b\\[c\\\\](files/x-a-b.png)",
    );
  });

  it("names a pasted image a screenshot and keeps a dropped file's name", () => {
    expect(uploadName(png("image.png"), true)).toBe("screenshot.png");
    expect(uploadName(new File(["x"], "", { type: "image/jpeg" }), true)).toBe(
      "screenshot.jpg",
    );
    expect(uploadName(png("Bug report.png"), false)).toBe("Bug report.png");
    expect(uploadName(new File(["x"], ""), false)).toBe("file");
  });

  it("refuses a file over 20 MB before sending it", () => {
    expect(tooLarge(png("ok.png", 10))).toBeUndefined();
    expect(MAX_UPLOAD_BYTES).toBe(20 * 1024 * 1024);
    const big = { name: "big.png", size: MAX_UPLOAD_BYTES + 1 };
    expect(tooLarge(big)).toBe(
      "big.png is larger than 20 MB, the limit for an attachment.",
    );
  });

  it("takes files from the clipboard only when it holds no text", () => {
    const image = png();
    const clipboard = (text: string, files: File[]) => ({
      getData: (type: string) => (type === "text/plain" ? text : ""),
      files,
    });
    expect(
      clipboardFiles(clipboard("a table copied from a sheet", [image])),
    ).toEqual([]);
    expect(clipboardFiles(clipboard("", [image]))).toEqual([image]);
    expect(clipboardFiles(clipboard("words", []))).toEqual([]);
    expect(clipboardFiles(null)).toEqual([]);
  });

  it("says why an upload was refused", () => {
    expect(
      refusalWords(
        "drawing.png",
        new ApiError(
          "drawing.png was not attached because it is an SVG image.",
          415,
          "type_not_allowed",
        ),
      ),
    ).toBe("drawing.png was not attached because it is an SVG image.");
    expect(refusalWords("a.png", new Error("Network down"))).toBe(
      "a.png was not attached: Network down",
    );
    expect(refusalWords("a.png", "boom")).toBe(
      "a.png was not attached: the upload failed",
    );
  });

  it("says what a batch attached", () => {
    const done = { name: "a.png", uploaded: uploaded("x-a.png", true) };
    expect(uploadedWords([done])).toBe("Attached a.png.");
    expect(uploadedWords([done, { name: "b.png", refusal: "no" }, done])).toBe(
      "Attached 2 files.",
    );
    expect(uploadedWords([{ name: "b.png", refusal: "no" }])).toBe("");
  });

  it("uploads one file at a time and reports each, refusing oversized ones unsent", async () => {
    const sent: string[] = [];
    const big = new File([new Uint8Array(1)], "big.png");
    Object.defineProperty(big, "size", { value: MAX_UPLOAD_BYTES + 1 });
    const results = await uploadAll(
      [png("a.png"), big, png("bad.png"), png("b.png")],
      async (_file, name) => {
        sent.push(name);
        if (name === "bad.png") throw new Error("Network down");
        return uploaded(`x-${name}`, true);
      },
      false,
    );
    expect(sent).toEqual(["a.png", "bad.png", "b.png"]);
    expect(
      results.map((result) => result.uploaded?.file ?? result.refusal),
    ).toEqual([
      "x-a.png",
      "big.png is larger than 20 MB, the limit for an attachment.",
      "bad.png was not attached: Network down",
      "x-b.png",
    ]);
  });
});
