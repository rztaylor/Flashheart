// Files sent into a ticket from the browser (FH-18, REV-2): the bytes go
// as the request body, the name and kind as query parameters; the server
// judges the type by content and stores the file in the ticket's files/.
// A project's image goes the same way (FH-151): its bytes as the body of
// a PUT, judged, cropped and resized by the server.
import { segment } from "./board";
import { type AuthenticatedFetch, apiError, isRecord } from "./client";
import { isSaved } from "./edit";

// Uploaded is a file the server stored in the ticket's files/.
export interface Uploaded {
  file: string;
  kind: string;
  // image says it shows as an image (PNG, JPEG, GIF or WebP).
  image: boolean;
  // reused says the ticket already had these bytes.
  reused: boolean;
}

// isUploaded checks the upload response (api.UploadResponse), which also
// carries the board revision that lists the file.
export function isUploaded(
  value: unknown,
): value is Uploaded & { revision: number } {
  return (
    isRecord(value) &&
    typeof value.file === "string" &&
    typeof value.kind === "string" &&
    typeof value.image === "boolean" &&
    typeof value.reused === "boolean" &&
    typeof value.revision === "number"
  );
}

// uploadFile stores file in ticket id's files/ under name (sanitised and
// given the extension of what it is by the server). A refusal is an
// ApiError whose message names the file and why (type_not_allowed,
// too_large).
export async function uploadFile(
  fetcher: AuthenticatedFetch,
  id: string,
  file: Blob,
  name: string,
  kind?: string,
): Promise<Uploaded> {
  const query = new URLSearchParams({ name });
  if (kind) query.set("kind", kind);
  const response = await fetcher(`/api/tickets/${segment(id)}/files?${query}`, {
    method: "POST",
    cache: "no-store",
    headers: { "Content-Type": "application/octet-stream" },
    body: file,
  });
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) throw apiError(response, payload);
  if (!isUploaded(payload)) throw new Error("Upload response was invalid");
  return {
    file: payload.file,
    kind: payload.kind,
    image: payload.image,
    reused: payload.reused,
  };
}

const projectImagePath = (project: string) =>
  `/api/projects/${segment(project)}/image`;

// setProjectImage makes file the project's badge image (FH-151). The
// server judges it by its bytes and refuses what is not a JPEG or PNG, or
// is too large, with an ApiError that says why (type_not_allowed,
// too_large); the project is then unchanged.
export async function setProjectImage(
  fetcher: AuthenticatedFetch,
  project: string,
  file: Blob,
): Promise<void> {
  const response = await fetcher(projectImagePath(project), {
    method: "PUT",
    cache: "no-store",
    headers: { "Content-Type": "application/octet-stream" },
    body: file,
  });
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) throw apiError(response, payload);
  if (!isSaved(payload)) throw new Error("Project image response was invalid");
}

// clearProjectImage removes the project's badge image and its field, so
// the badge shows the key again.
export async function clearProjectImage(
  fetcher: AuthenticatedFetch,
  project: string,
): Promise<void> {
  const response = await fetcher(projectImagePath(project), {
    method: "DELETE",
    cache: "no-store",
  });
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) throw apiError(response, payload);
  if (!isSaved(payload)) throw new Error("Project image response was invalid");
}
