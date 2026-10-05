// Ticket vocabulary shared by the edit forms, in the board format's order
// (docs/dev/specs/board-format.md).
export const TICKET_TYPES = [
  "feature",
  "test",
  "bug",
  "refactor",
  "infra",
  "docs",
  "spike",
] as const;

export const PRIORITIES = ["high", "medium", "low"] as const;
