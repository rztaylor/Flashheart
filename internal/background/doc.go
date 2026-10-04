// Package background starts Flashheart's server as a detached child process
// and carries its one-line startup handshake back to the launching process.
//
// It owns re-executing the binary in a new session with discarded standard
// streams, the inherited handshake descriptor, and the startup timeout. It
// knows nothing about Singleserve, flags or user-facing messages: app owns the
// server lifecycle and cli decides when to detach and what to print.
package background
