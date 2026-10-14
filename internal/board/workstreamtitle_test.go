package board

import "testing"

// TestSameWorkstreamTitle: two workstream titles are the same when they
// match ignoring letter case and runs of spaces, which is how a person
// reads them in a picker (FH-163, D73).
func TestSameWorkstreamTitle(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"Workstream editing", "Workstream editing", true},
		{"Workstream editing", "workstream EDITING", true},
		{"Workstream editing", "  Workstream \t editing ", true},
		{"Équipe", "équipe", true},
		{"Workstream editing", "Workstream editing!", false},
		{"Workstream editing", "Workstream-editing", false},
		{"Offline", "Offline mode", false},
	} {
		if got := SameWorkstreamTitle(tc.a, tc.b); got != tc.same {
			t.Errorf("SameWorkstreamTitle(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.same)
		}
		if (WorkstreamTitleKey(tc.a) == WorkstreamTitleKey(tc.b)) != tc.same {
			t.Errorf("keys of %q and %q disagree with SameWorkstreamTitle", tc.a, tc.b)
		}
	}
}
