// Package api owns Flashheart's HTTP JSON endpoints under /api/: routing,
// request validation, response shapes and JSON errors.
//
// It serves values handed to it by app and owns no filesystem rules, board
// parsing or Singleserve lifecycle; authentication is applied by Singleserve
// before requests arrive here.
package api
