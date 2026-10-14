package protocol

// ClaimReminder asks a session that changed files while holding no ticket,
// by claim or by branch, to claim one (D50, agent-protocol §9). Hooks give
// it once a session: as the reason its stop is held where the project
// enforces handoffs, otherwise with its next prompt or tool result. It
// names the tools to use.
const ClaimReminder = "[Flashheart] This session changed files but holds no ticket. Claim the ticket this work belongs to (find it with list_tickets, then claim), or create one with create_ticket and claim it, so the board links the work and its handoff. Asked once."
