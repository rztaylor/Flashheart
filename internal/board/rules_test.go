package board

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// TestNotesAreDatedBullets: every note Flashheart writes, from the board or
// an agent, is "- YYYY-MM-DD · text", or "- YYYY-MM-DD · by: text" when it
// is written for someone (board-format §Notes written by Flashheart; FH-81,
// FH-85).
func TestNotesAreDatedBullets(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 4, 23, 30, 0, 0, time.UTC)
	cases := []struct{ by, text, want string }{
		{"", "Question from claude:3f2a9c1e — \"Ship?\" Answer (Robert): \"Yes\".", "- 2026-10-04 · Question from claude:3f2a9c1e — \"Ship?\" Answer (Robert): \"Yes\"."},
		{"claude:3f2a9c1e", "Claimed from codex:77aa (ended): gone", "- 2026-10-04 · claude:3f2a9c1e: Claimed from codex:77aa (ended): gone"},
		{"unknown", "Plan changed.", "- 2026-10-04 · unknown: Plan changed."},
	}
	for _, tc := range cases {
		if got := NoteLine(day, tc.by, tc.text); got != tc.want {
			t.Errorf("NoteLine(%q, %q) = %q, want %q", tc.by, tc.text, got, tc.want)
		}
	}
	if got := BlockedStartNote(day, "  Hotfix\nneeded now "); got != "- 2026-10-04 · Started while blocked: Hotfix needed now" {
		t.Errorf("BlockedStartNote = %q", got)
	}
}

func TestValidateField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		key, value string
		ok         bool
	}{
		{"title", "Fix the header", true},
		{"title", " ", false},
		{"title", strings.Repeat("x", 301), false},
		{"status", "up-next", true},
		{"status", "todo", false},
		{"type", "bug", true},
		{"type", "epic", false},
		{"priority", "high", true},
		{"priority", "urgent", false},
		{"created", "2026-10-05", true},
		{"created", "yesterday", false},
		{"workstream", "", true},
		{"workstream", "board-ui", true},
		{"workstream", "Board UI", false},
		{"depends-on", "FH-42", true},
		{"depends-on", "fh-42", false},
		{"branch", "feature/x", true},
		{"branch", "two\nlines", false},
		{"pr", "https://github.com/o/r/pull/7", true},
		{"pr", "", true},
		{"pr", "two\nlines", false},
		{"tags", strings.Repeat("t", 201), false},
	}
	for _, tc := range cases {
		if err := ValidateField(tc.key, tc.value); (err == nil) != tc.ok {
			t.Errorf("ValidateField(%q, %q) = %v, want ok=%v", tc.key, tc.value, err, tc.ok)
		}
	}
	if !slices.Contains(ScalarFields, "branch") || !slices.Contains(ScalarFields, "pr") || !slices.Contains(ListFields, "tags") || slices.Contains(ScalarFields, "id") {
		t.Errorf("editable fields: scalar %v list %v", ScalarFields, ListFields)
	}
}

func TestReviewWarnings(t *testing.T) {
	t.Parallel()

	ticket := Ticket{Criteria: []Criterion{{Text: "a", Done: true}, {Text: "b"}, {Text: "c"}}}
	got := ReviewWarnings(ticket, "", false, nil)
	want := []string{"There is no review file yet.", "2 acceptance criteria are not ticked."}
	if !slices.Equal(got, want) {
		t.Fatalf("warnings = %q, want %q", got, want)
	}
	ticket.Criteria = ticket.Criteria[:2]
	if got := ReviewWarnings(ticket, shown, true, nil); !slices.Equal(got, []string{"1 acceptance criterion is not ticked."}) {
		t.Fatalf("warnings = %q", got)
	}
	ticket.Criteria = nil
	if got := ReviewWarnings(ticket, shown, true, nil); len(got) != 0 {
		t.Fatalf("warnings = %q", got)
	}
	// A review with no screenshot and no stated reason warns, never refuses.
	if got := ReviewWarnings(ticket, "# Review: X\n\n## Summary\nDone.\n", true, nil); len(got) != 1 || !strings.Contains(got[0], "no evidence") {
		t.Fatalf("warnings = %q", got)
	}
}

const shown = "# Review: X\n\n## Evidence\n![Board, light](files/board.png)\n"

func TestReviewHasEvidence(t *testing.T) {
	t.Parallel()

	for review, want := range map[string]bool{
		shown: true,
		"## How to Verify\n![Panel](/tmp/panel.png)\n":                               true,
		"## Evidence\nNo visible change: the store's lock order only.\n":             true,
		"## Evidence\n<screenshots, or why there are none>\n\n## Tests\nAll pass.\n": false,
		"## Evidence\n\n## Tests\nAll pass.\n":                                       false,
		"## Summary\nSee [the log](files/run.log).\n":                                false,
		"": false,
	} {
		if got := ReviewHasEvidence(review); got != want {
			t.Errorf("ReviewHasEvidence(%q) = %v, want %v", review, got, want)
		}
	}
}

func TestOneLine(t *testing.T) {
	t.Parallel()

	if got := OneLine("  a\n b\tc  ", 10); got != "a b c" {
		t.Fatalf("OneLine = %q", got)
	}
	if got := OneLine("héllo world", 4); got != "héll" {
		t.Fatalf("OneLine = %q", got)
	}
}
