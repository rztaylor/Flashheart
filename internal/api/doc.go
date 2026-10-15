// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes (a run's short id and typed Needs
// you reason, which the frontend words), long-polling for changes
// (LIFE-3), JSON errors (ErrorCodes) and attachment headers (SEC-4).
// Responses and request bodies are this package's own types (board's,
// config's, runs' and events' are mapped at the boundary, never sent or
// decoded as they are), each response with a golden fixture in
// testdata/api that the frontend's guards are tested against (FH-97,
// FH-175).
//
// Reads come from index snapshots and, for review and attachment files,
// store's checks. Writes (ticket moves, fields, sections, criteria,
// placement and new tickets; workstreams, their fields, sections, order
// and slug, given or made from a new title; archive, restore and delete;
// keys, preferences, answers, comments, change requests and uploads)
// decode input into board's change-sets, write through store's locked,
// hash-checked writes as the person, so the store appends the ticket's
// history, and answer a stale hash with 409 and the current file (STO-3,
// EDIT-7). Each ticket write appends one event that the human did it
// (EDIT-10), best effort. An answer is queued in the asking session's
// inbox (RUN-8); a change request goes to the ticket's holder and a
// comment to a run the person chose among its live claimants
// (index.Snapshot; REV-6, FH-127); a deleted comment is withdrawn from
// those inboxes (FH-140).
//
// It owns no filesystem rules, parsing or blocking rules (board), run
// derivation (runs) or Singleserve lifecycle (app); authentication is
// applied by Singleserve before requests arrive here.
package api
