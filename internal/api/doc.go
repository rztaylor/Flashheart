// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes (cards, project summaries, ticket
// detail with its runs, comments and history, workstreams and runs, with
// each run's short id and typed Needs you reason for the frontend to
// word), long-polling for changes (LIFE-3), JSON errors and attachment
// response headers (SEC-4).
//
// Reads come from index snapshots and, for review and attachment files,
// store's checks. Writes (moves, field and section edits, criteria, new
// tickets and workstreams, workstream fields, sections, order and slug,
// given or made from a new title (FH-158, FH-167), archive, restore and permanent delete of tickets and projects,
// placement, keys, preferences, answers, comments and their delivery,
// change requests, uploads) decode input into board's change-sets, write
// through store's locked, hash-checked writes as the person, so the store
// appends the ticket's history, and answer a stale hash with 409 and the
// current file (STO-3, EDIT-7). Each ticket write appends one event that
// the human did it (EDIT-10), best effort. An answer is queued in the
// asking session's inbox (RUN-8); a change request goes to the ticket's
// holder and a comment to a run the person chose among its live claimants
// (index.Snapshot; REV-6, FH-127); a deleted comment is withdrawn from
// those inboxes (FH-140).
//
// It owns no filesystem rules, parsing or blocking rules (board), run
// derivation (runs) or Singleserve lifecycle (app); authentication is
// applied by Singleserve before requests arrive here.
package api
