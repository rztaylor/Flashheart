// Package app composes Flashheart's HTTP surface with its Singleserve browser
// lifetime for one serve process.
//
// It owns router assembly, security headers, the shutdown guard (no quit while
// a write is in flight, LIFE-2), browser opening with a manual-URL report, and
// graceful drain. It prints only --debug lifecycle summaries: CLI parsing and
// launch presentation belong to cli, detaching to background, endpoints to
// api, static assets to webui and settings to config.
package app
