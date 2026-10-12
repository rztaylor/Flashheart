// Package logfile owns Flashheart's append-only diagnostic logs under the
// root's .flashheart directory (serve.log and hook-errors.log) and their
// paths (HookErrorsPath).
//
// It creates the file and its directory only on the first write, prefixes
// each write with a UTC timestamp, and keeps one rotated generation. Callers
// decide what to log and must never pass credentials, prompts or tool data;
// board files and the event log belong to store and events. Recent reads a
// log's entries since a given time, for doctor (SET-4).
//
// It opens these two files directly rather than through store, an accepted
// exception to store owning the board root (architecture.md): the logs hold
// no board data and must record failures of the store itself.
package logfile
