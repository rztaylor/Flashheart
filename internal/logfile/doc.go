// Package logfile owns Flashheart's append-only diagnostic logs under the
// root's .flashheart directory (serve.log and hook-errors.log).
//
// It creates the file and its directory only on the first write, prefixes
// each write with a UTC timestamp, and keeps one rotated generation. Callers
// decide what to log and must never pass credentials, prompts or tool data;
// board files and the event log belong to store and events. Recent reads a
// log's entries since a given time, for doctor (SET-4).
package logfile
