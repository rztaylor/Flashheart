// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes (cards with live run badges and
// session summaries, project summaries with run counts, ticket detail with
// its runs, workstreams, runs; runs, badges and questions carry the run's
// short id and runs and badges its typed Needs you reason, which the
// frontend words),
// long-polling for changes (LIFE-3), JSON errors and attachment response
// headers (SEC-4).
//
// Reads come from index snapshots and, for review and attachment files,
// store's checks. Writes (moves, field edits decoded into board's
// change-set and written by store.EditTicket, as agents' are, one section
// rewritten in place (board.SectionChange through store.EditTicket), raw edits,
// criteria, new tickets,
// archive and restore, permanent delete of archived tickets with its preview
// token (EDIT-8), project archive, restore and delete (PRJ-5), placement
// in manual order (EDIT-9), workstream order,
// project keys, preferences, answers to agents'
// questions, comments on a ticket, D47, files uploaded into a ticket,
// REV-7, whose limit it reads from the config) validate input, apply
// mdfile edits through store's locked, hash-checked writes, and answer a
// stale hash with 409 and the current file (STO-3, EDIT-7). A preference
// read waits for any preference save in flight (CFG-2). Each write that
// changes or comments on a ticket appends one event recording that the
// human did (EDIT-10), best effort once the ticket is saved; the Writer
// writes as the person, so the store appends the ticket's history. Ticket
// detail carries the ticket's comments and history. A request for changes
// on a ticket in review, and a reopened request, are queued in the inbox
// of the live session holding the ticket (index.Snapshot.Holder), if any
// (requests.go, REV-6); cards and detail count the change requests. An answer is
// appended to the event log, queued in the asking session's inbox and noted
// in the ticket (RUN-8, HOOK-5, STO-6). It owns no
// filesystem rules, parsing or blocking rules (board), run derivation (runs)
// or Singleserve
// lifecycle (app); authentication is applied by Singleserve before requests
// arrive here.
package api
