// Package doctor checks a machine's Flashheart installation without
// changing it (SET-4): the board root (exists, is writable, is not
// writable by other users, its settings parse), each agent's configuration
// (setup.Diagnose) and the hook errors logged in the last day, and renders
// the findings as a report.
//
// It reads the root and the agent's configuration only (on systems without
// access(2) it proves the root writable with a probe file it removes).
// Flags and exit codes belong to cli.
package doctor
