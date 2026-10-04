// Package cli owns Flashheart's command line: subcommands, flags, help and
// version output, board-root resolution and exit codes.
//
// It chooses between a detached, foreground or background-child serve and
// presents the launch outcome (manual URL, debug address). Singleserve
// composition belongs to app, detached process mechanics to background, and
// signal handling to cmd/flashheart. Usage errors exit 2; failures exit 1.
package cli
