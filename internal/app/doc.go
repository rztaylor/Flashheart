// Package app composes Flashheart's HTTP surface with its Singleserve browser
// lifetime for one serve process.
//
// It owns router assembly (wiring store, events, index and api), security
// headers, the shutdown guard (no quit while a write is in flight, LIFE-2),
// expiring old files at startup and daily (event files and answers inboxes
// after event_retention_days, STO-5; hook activity state after stale_hours,
// FH-82), browser opening
// with a manual-URL report, and graceful drain. It prints --debug lifecycle
// summaries and diagnostics such as a failed expiry (serve.log in the
// background); CLI parsing and
// launch presentation belong to cli, detaching to background, endpoints to
// api, static assets to webui and settings to config (read through
// store.ReadConfig).
package app
