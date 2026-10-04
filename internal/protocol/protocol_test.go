package protocol

import "testing"

func TestVersionMatchesAgentProtocolSpec(t *testing.T) {
	t.Parallel()

	// docs/dev/specs/agent-protocol.md §14 declares PROTOCOL_VERSION = 1.
	if Version != 1 {
		t.Fatalf("Version = %d, want 1; bump only with a non-additive protocol change", Version)
	}
}
