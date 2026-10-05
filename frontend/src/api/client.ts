// Shared request helpers for the authenticated local API.

export type AuthenticatedFetch = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  // payload is the whole error response, for errors that carry data (a
  // conflict's current file, the keys in use, blocking reasons).
  readonly payload: Record<string, unknown>;

  constructor(
    message: string,
    status: number,
    code: string,
    payload: Record<string, unknown> = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.payload = payload;
  }
}

function apiError(response: Response, payload: unknown): ApiError {
  const body = isRecord(payload) ? payload : {};
  const error = isRecord(body.error) ? body.error : undefined;
  return new ApiError(
    typeof error?.message === "string"
      ? error.message
      : `Request failed (${response.status})`,
    response.status,
    typeof error?.code === "string" ? error.code : "",
    body,
  );
}

// getJSON performs a GET and returns the parsed body after validating it.
export async function getJSON<T>(
  fetcher: AuthenticatedFetch,
  path: string,
  isValid: (value: unknown) => value is T,
  invalidMessage: string,
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetcher(path, { cache: "no-store", signal });
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) throw apiError(response, payload);
  if (!isValid(payload)) throw new Error(invalidMessage);
  return payload;
}

// sendJSON performs a write with a JSON body and returns the parsed response
// body (undefined for 204), throwing ApiError for error statuses.
export async function sendJSON(
  fetcher: AuthenticatedFetch,
  method: "POST" | "PUT" | "PATCH",
  path: string,
  body?: unknown,
): Promise<unknown> {
  const response = await fetcher(path, {
    method,
    cache: "no-store",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (response.status === 204) return undefined;
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) throw apiError(response, payload);
  return payload;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
