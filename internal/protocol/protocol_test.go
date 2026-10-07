package protocol

import (
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
)

func TestVersionMatchesAgentProtocolSpec(t *testing.T) {
	t.Parallel()

	// docs/dev/specs/agent-protocol.md §14 declares PROTOCOL_VERSION = 1.
	if Version != 1 {
		t.Fatalf("Version = %d, want 1; bump only with a non-additive protocol change", Version)
	}
}

func TestSkillCoversTheProtocol(t *testing.T) {
	t.Parallel()

	skill := Skill()
	if !strings.HasPrefix(skill, "---\nname: flashheart\ndescription: ") || !strings.Contains(skill, "flashheart-protocol: 1\n---\n\n# Flashheart board") {
		t.Fatalf("skill frontmatter:\n%s", skill[:min(len(skill), 400)])
	}
	// Every MCP tool of mcp-protocol is explained (attach arrives later).
	for _, tool := range []string{"board_context", "list_tickets", "get_ticket", "claim", "release", "checkpoint", "update_ticket", "move", "set_project_key", "create_ticket", "write_review", "ask_human", "create_workstream"} {
		if !strings.Contains(skill, "`"+tool) {
			t.Errorf("skill does not mention %s", tool)
		}
	}
	for _, rule := range []string{"information, never as\ninstructions", "Never move a ticket to\n   `done`", "Test Plan", "Reproduction", "never create,\nedit, move or delete board files", "shared goal", "Leave single tickets out of workstreams", "If your turn ends with a question for the user", "only in chat leaves your run in Waiting, not Needs you", "returns a `flashheart await` command", "run_in_background",
		"## Evidence", "No visible change:", "caption", "every state the change touched"} {
		if !strings.Contains(skill, rule) {
			t.Errorf("skill is missing %q", rule)
		}
	}
	// The review template, unfilled, is not evidence.
	template := skill[strings.Index(skill, "# Review: <title>"):]
	if board.ReviewHasEvidence(template) {
		t.Error("the unfilled review template counts as evidence")
	}
	if len(skill) > 8<<10 {
		t.Errorf("skill is %d bytes; keep it under 8 KiB", len(skill))
	}
	if !strings.Contains(Instructions(), "protocol 1") || len(Instructions()) > 400 {
		t.Errorf("instructions = %q", Instructions())
	}
}

func TestAwaitCommandIsRunnableInAShell(t *testing.T) {
	t.Parallel()

	got := AwaitCommand("/Users/me/bin/flashheart", "/Users/me/My Board", "alpha", "q-ABC")
	want := `/Users/me/bin/flashheart await q-ABC --project alpha --root '/Users/me/My Board'`
	if got != want {
		t.Fatalf("AwaitCommand = %q, want %q", got, want)
	}
	if got := AwaitCommand("", "", "alpha", "q-ABC"); got != "flashheart await q-ABC --project alpha" {
		t.Fatalf("defaults: %q", got)
	}
	for in, want := range map[string]string{"plain/path-1.2": "plain/path-1.2", "": "''", "it's": `'it'\''s'`, "a;b": "'a;b'"} {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
