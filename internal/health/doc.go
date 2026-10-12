// Package health holds the vocabulary of a configuration check (SET-4): a
// Finding and the Level saying how much it matters. setup's agents produce
// findings about their configuration and doctor produces its own about the
// board root and hook errors; doctor renders them all.
//
// It has no behaviour of its own and imports nothing from Flashheart.
package health
