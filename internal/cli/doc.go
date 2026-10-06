// Package cli owns Flashheart's command line: subcommands, flags (which may
// follow positional arguments), help and version output, board-root
// resolution and exit codes.
//
// It chooses between a detached, foreground or background-child serve,
// presents the launch outcome, and picks the hook adapter by agent name.
// Usage errors exit 2 and failures exit 1, except `hook`, which always exits
// 0 and reports problems on stderr and in hook-errors.log (HOOK-1).
// Singleserve composition belongs to app, detaching to background, hook
// handling to hooks, and signals to cmd/flashheart.
package cli
