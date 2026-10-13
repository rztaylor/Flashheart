import type { Uploaded } from "../api/files";
import { messageOf } from "./errors";

// Files put into a ticket from the browser (FH-18, REV-2): what a pasted
// or dropped file is called, the markdown that refers to it, the words for
// a refusal, and sending a batch one file at a time. The server judges each
// file by its bytes; only the size is checked here, so an oversized file is
// never sent.

export type { Uploaded };

// Upload sends one file's bytes under name; a pasted image goes as kind
// screenshot.
export type Upload = (
  file: Blob,
  name: string,
  kind?: string,
) => Promise<Uploaded>;

// MAX_UPLOAD_BYTES is REV-2's limit for an attachment.
export const MAX_UPLOAD_BYTES = 20 * 1024 * 1024;

// ALLOWED_WORDS names what may be attached, for a hint.
export const ALLOWED_WORDS =
  "PNG, JPEG, GIF, WebP, PDF, text, markdown, JSON or log files up to 20 MB";

const imageExtensions: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
};

// uploadName is what a file is sent as: a pasted image is a screenshot
// (browsers call every clipboard image "image.png"); a dropped or chosen
// file keeps its own name.
export function uploadName(
  file: { name: string; type: string },
  pasted: boolean,
): string {
  if (pasted && file.type in imageExtensions)
    return `screenshot.${imageExtensions[file.type]}`;
  return file.name || "file";
}

// tooLarge is the refusal for a file over the limit, or undefined.
export function tooLarge(file: {
  name: string;
  size: number;
}): string | undefined {
  return file.size > MAX_UPLOAD_BYTES
    ? `${file.name || "The file"} is larger than 20 MB, the limit for an attachment.`
    : undefined;
}

// referenceFor is the markdown a ticket links a stored file with: an image
// for an image, a link for anything else, the text being the name it was
// given (without its extension for an image).
export function referenceFor(uploaded: Uploaded, name: string): string {
  const target = `files/${uploaded.file}`;
  if (uploaded.image) {
    const alt = name.replace(/\.[^.]*$/, "") || "Screenshot";
    return `![${escapeText(alt)}](${target})`;
  }
  return `[${escapeText(name)}](${target})`;
}

// escapeText keeps a name from ending a link's text early.
function escapeText(text: string): string {
  return text.replace(/[\\[\]]/g, "\\$&").replace(/\s+/g, " ");
}

// clipboardFiles are the files a paste carries when it carries no text: a
// screenshot. Copied text that also comes with a picture of itself (a
// table from a sheet) pastes as text.
export function clipboardFiles(
  data: { getData(type: string): string; files: ArrayLike<File> } | null,
): File[] {
  if (!data || data.getData("text/plain")) return [];
  return Array.from(data.files);
}

// refusalWords says why a file was not attached: the server's words for a
// refused type or size (they name the file), else the failure's.
export function refusalWords(name: string, error: unknown): string {
  const code =
    error instanceof Error ? (error as Error & { code?: unknown }).code : "";
  if (code === "type_not_allowed" || code === "too_large")
    return messageOf(error, name);
  return `${name} was not attached: ${messageOf(error, "the upload failed")}`;
}

// UploadResult is one file of a batch: stored, or refused with the words.
export interface UploadResult {
  name: string;
  uploaded?: Uploaded;
  refusal?: string;
}

// uploadAll sends files one at a time, in order, so their references
// follow the order they were dropped in; an oversized file is refused
// without being sent. onResult follows each file.
export async function uploadAll(
  files: File[],
  upload: Upload,
  pasted: boolean,
  onResult?: (result: UploadResult, index: number) => void,
): Promise<UploadResult[]> {
  const results: UploadResult[] = [];
  for (const [index, file] of files.entries()) {
    const name = uploadName(file, pasted);
    const refusal = tooLarge({ name, size: file.size });
    let result: UploadResult;
    if (refusal) result = { name, refusal };
    else {
      try {
        result = {
          name,
          uploaded: await upload(file, name, pasted ? "screenshot" : undefined),
        };
      } catch (error) {
        result = { name, refusal: refusalWords(name, error) };
      }
    }
    results.push(result);
    onResult?.(result, index);
  }
  return results;
}

// uploadedWords says what a batch attached, for the status line: "" when
// nothing was (the refusals say why).
export function uploadedWords(results: UploadResult[]): string {
  const attached = results.filter((result) => result.uploaded);
  const [first] = attached;
  if (!first) return "";
  return attached.length === 1
    ? `Attached ${first.name}.`
    : `Attached ${attached.length} files.`;
}
