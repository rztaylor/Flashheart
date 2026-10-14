// Ticket vocabulary, in the board format's order
// (docs/dev/specs/board-format.md): the only copy in the frontend, matched
// to the server's board.TicketTypes and board.Priorities by
// api/vocabulary.test.ts (FH-97).
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
