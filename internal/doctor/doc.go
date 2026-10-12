// Package doctor checks a machine's Flashheart installation without
// changing it (SET-4): the board root (exists, is writable, is not
// writable by other users, its settings parse), each agent it is given (a
// section per agent, from the agent's own checks) and the hook errors
// logged in the last day, and renders the findings as a report.
//
// It reads only: it stats the root folder itself, reads the settings
// through store (store.ReadConfig) and the hook errors through logfile
// (logfile.HookErrorsPath, logfile.Recent); agents read their own
// configuration. On systems without access(2) it proves the root writable
// with a probe file it removes. Which agents exist belongs to cli's
// registry, their checks to setup, and flags and exit codes to cli.
package doctor
