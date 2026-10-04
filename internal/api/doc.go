// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes (cards, summaries, ticket detail,
// workstreams), JSON errors and attachment response headers (SEC-4).
//
// It reads board snapshots from index and review or attachment files through
// store's checks, and owns no filesystem rules, parsing or blocking rules
// (board) or Singleserve lifecycle (app); authentication is applied by
// Singleserve before requests arrive here.
package api
