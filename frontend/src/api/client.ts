// Shared request helpers for the authenticated local API.

export type AuthenticatedFetch = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(message: string, status: number, code: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
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
  if (!response.ok) {
    const error =
      isRecord(payload) && isRecord(payload.error) ? payload.error : undefined;
    throw new ApiError(
      typeof error?.message === "string"
        ? error.message
        : `Request failed (${response.status})`,
      response.status,
      typeof error?.code === "string" ? error.code : "",
    );
  }
  if (!isValid(payload)) throw new Error(invalidMessage);
  return payload;
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
