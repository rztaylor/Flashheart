// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes (cards with live run badges,
// summaries with run counts, ticket detail with its runs, workstreams, runs),
// long-polling for changes (LIFE-3), JSON errors and attachment response
// headers (SEC-4).
//
// Reads come from index snapshots and, for review and attachment files,
// store's checks. Writes (moves, field and raw edits, criteria, new tickets,
// archive and restore, permanent delete of archived tickets with its preview
// token (EDIT-8), placement in manual order (EDIT-9), workstream order,
// project keys, preferences, answers to agents'
// questions) validate input, apply
// mdfile edits through store's locked, hash-checked writes, and answer a
// stale hash with 409 and the current file (STO-3, EDIT-7). An answer is
// appended to the event log, queued in the asking session's inbox and noted
// in the ticket (RUN-8, HOOK-5, STO-6). It owns no
// filesystem rules, parsing or blocking rules (board), run derivation (runs)
// or Singleserve
// lifecycle (app); authentication is applied by Singleserve before requests
// arrive here.
package api
