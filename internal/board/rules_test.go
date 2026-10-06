package board

import (
	"slices"
	"strings"
	"testing"
)

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
		{"tags", strings.Repeat("t", 201), false},
	}
	for _, tc := range cases {
		if err := ValidateField(tc.key, tc.value); (err == nil) != tc.ok {
			t.Errorf("ValidateField(%q, %q) = %v, want ok=%v", tc.key, tc.value, err, tc.ok)
		}
	}
	if !slices.Contains(ScalarFields, "branch") || !slices.Contains(ListFields, "tags") || slices.Contains(ScalarFields, "id") {
		t.Errorf("editable fields: scalar %v list %v", ScalarFields, ListFields)
	}
}

func TestReviewWarnings(t *testing.T) {
	t.Parallel()

	ticket := Ticket{Criteria: []Criterion{{Text: "a", Done: true}, {Text: "b"}, {Text: "c"}}}
	got := ReviewWarnings(ticket, false)
	want := []string{"There is no review file yet.", "2 acceptance criteria are not ticked."}
	if !slices.Equal(got, want) {
		t.Fatalf("warnings = %q, want %q", got, want)
	}
	ticket.Criteria = ticket.Criteria[:2]
	if got := ReviewWarnings(ticket, true); !slices.Equal(got, []string{"1 acceptance criterion is not ticked."}) {
		t.Fatalf("warnings = %q", got)
	}
	ticket.Criteria = nil
	if got := ReviewWarnings(ticket, true); len(got) != 0 {
		t.Fatalf("warnings = %q", got)
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
