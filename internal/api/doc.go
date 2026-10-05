// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes (cards, summaries, ticket detail,
// workstreams), long-polling for changes (LIFE-3), JSON errors and
// attachment response headers (SEC-4).
//
// Reads come from index snapshots and, for review and attachment files,
// store's checks. Writes (moves, field and raw edits, criteria, new tickets,
// archive, workstream order, project keys, preferences) validate input, apply
// mdfile edits through store's locked, hash-checked writes, and answer a
// stale hash with 409 and the current file (STO-3, EDIT-7). It owns no
// filesystem rules, parsing or blocking rules (board) or Singleserve
// lifecycle (app); authentication is applied by Singleserve before requests
// arrive here.
package api
