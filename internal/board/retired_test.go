package board

import "testing"

// A deleted ticket's id, or any id of a deleted project's key, blocks
// nothing, even where a reference to it survived (an archived project
// restored after the delete, or a hand edit).
func TestRetiredIDsCountAsDone(t *testing.T) {
	t.Parallel()

	b := Board{
		Projects: []Project{{
			Name: "beta", Key: "BE", Retired: []string{"BE-7"},
			Tickets: []Ticket{
				{ID: "BE-1", Column: Backlog, DependsOn: []string{"BE-7", "AL-3"}},
				{ID: "BE-2", Column: Backlog, DependsOn: []string{"NO-1"}},
			},
		}},
		RetiredKeys: []string{"AL"},
	}
	analysis := Analyze(b)
	if reasons := analysis.Blocked[Ref{Project: "beta", ID: "BE-1"}]; len(reasons) != 0 {
		t.Errorf("BE-1 blocked by %+v", reasons)
	}
	if reasons := analysis.Blocked[Ref{Project: "beta", ID: "BE-2"}]; len(reasons) != 1 || !reasons[0].Missing {
		t.Errorf("BE-2 should still wait on the missing NO-1: %+v", reasons)
	}
}
