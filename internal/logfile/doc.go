// Package logfile owns Flashheart's append-only diagnostic logs under the
// root's .flashheart directory (serve.log now, hook-errors.log later).
//
// It creates the file and its directory only on the first write, prefixes
// each write with a UTC timestamp, and keeps one rotated generation. Callers
// decide what to log and must never pass credentials, prompts or tool data;
// board files and the event log belong to store and events.
package logfile
