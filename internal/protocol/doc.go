// Package protocol owns the agent protocol version and the text Flashheart
// gives agents: the recovery note returned at session start (HOOK-3,
// agent-protocol §8) and, later, the protocol instructions shared by MCP and
// setup.
//
// It performs no I/O and renders only what it is given; gathering the
// ticket, handoff and previous run belongs to hooks, storage and transport
// to other packages.
package protocol
