// Package archtest holds architecture tests that read the module's source
// rather than run it. It has no code outside its tests.
//
// TestOnlyAllowedPackagesTouchFiles enforces that store is the only package
// that opens files under the board root (FH-108): only the packages in its
// allowlist may call the os, io/ioutil and path/filepath functions that
// open, create, list, stat, change or remove files, each with a reason
// (a path outside the root, or an accepted exception). Everything else
// reaches the filesystem through store.
//
// Editing tests (FH-85) keep the ticket-editing rules defined once: every
// Notes bullet is board.NoteLine's, and api and mcpserver write typed field
// edits only through board's change-set and store.EditTicket; mcpserver
// edits a workstream only through board.NewWorkstreamChanges and
// store.EditWorkstream (FH-46).
//
// TestLeanPackagesDoNotLinkTheWebServer (FH-106, FH-169) reads the import
// graph with go list -deps: cli, mcpserver, hooks, await, setup and doctor
// must not depend on index, api, app, webui or net/http, except net/http
// through the MCP SDK (D72).
package archtest
