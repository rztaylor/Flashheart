// messageOf is the words for a failure: what an Error says, or the caller's
// fallback for anything else (a thrown string, a missing message).
export function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}
