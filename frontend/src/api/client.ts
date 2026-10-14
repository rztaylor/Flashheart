// Shared request helpers for the authenticated local API.

export type AuthenticatedFetch = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

// ERROR_CODES are the error codes the browser branches on (error.code),
// each one the server writes (api.ErrorCodes; api/vocabulary.test.ts
// checks both, and that the source compares codes only from this list).
export const ERROR_CODES = [
  "conflict",
  "confirm_blocked",
  "key_taken",
  "title_taken",
  "too_large",
  "type_not_allowed",
] as const;

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

// apiError is the ApiError for an error response and its parsed body.
export function apiError(response: Response, payload: unknown): ApiError {
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
// body (undefined for 204), throwing ApiError for error statuses. keepalive
// lets the request outlive the page (a save flushed as the page unloads);
// browsers cap such bodies at 64 KiB.
export async function sendJSON(
  fetcher: AuthenticatedFetch,
  method: "POST" | "PUT" | "PATCH" | "DELETE",
  path: string,
  body?: unknown,
  options: { keepalive?: boolean } = {},
): Promise<unknown> {
  const response = await fetcher(path, {
    method,
    cache: "no-store",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    ...(options.keepalive ? { keepalive: true } : {}),
  });
  if (response.status === 204) return undefined;
  const payload: unknown = await response.json().catch(() => undefined);
  if (!response.ok) throw apiError(response, payload);
  return payload;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export const isString = (value: unknown): value is string =>
  typeof value === "string";
export const isNumber = (value: unknown): value is number =>
  typeof value === "number";
export const isBoolean = (value: unknown): value is boolean =>
  typeof value === "boolean";
export const isStringArray = (value: unknown): value is string[] =>
  Array.isArray(value) && value.every(isString);

// isArrayOf checks an array whose every item passes guard.
export const isArrayOf =
  <T>(guard: (item: unknown) => item is T) =>
  (value: unknown): value is T[] =>
    Array.isArray(value) && value.every(guard);

// isOptional checks a field the server leaves out when it is empty.
export const isOptional =
  <T>(guard: (item: unknown) => item is T) =>
  (value: unknown): value is T | undefined =>
    value === undefined || guard(value);

// ErrorBody is every error response (api.writeError); some errors add
// fields to error or beside it (edit.ts reads those).
export interface ErrorBody {
  error: { code: string; message: string; [field: string]: unknown };
  [field: string]: unknown;
}

export const isErrorBody = (value: unknown): value is ErrorBody =>
  isRecord(value) &&
  isRecord(value.error) &&
  isString(value.error.code) &&
  isString(value.error.message);
