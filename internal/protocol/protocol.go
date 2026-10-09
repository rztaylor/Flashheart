package protocol

// Version is PROTOCOL_VERSION from docs/dev/specs/agent-protocol.md §14.
// Additive changes keep it; removing or changing meaning bumps it.
// Version 2 (FH-55): hooks record throttled activity records instead of one
// tool.used per tool call.
const Version = 2
