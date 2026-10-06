package board

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRankBetweenStaysOrderedAndValid(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ a, b string }{
		{"", ""}, {"V", ""}, {"", "V"}, {"V", "W"}, {"V", "V1"}, {"z", ""}, {"", "01"}, {"Az", "B"}, {"zzz", ""},
	} {
		got := RankBetween(tc.a, tc.b)
		if !ValidRank(got) {
			t.Errorf("RankBetween(%q, %q) = %q, not a valid rank", tc.a, tc.b, got)
		}
		if tc.a != "" && got <= tc.a || tc.b != "" && got >= tc.b {
			t.Errorf("RankBetween(%q, %q) = %q, not strictly between", tc.a, tc.b, got)
		}
	}

	// Many insertions at random places keep a strict order with short keys.
	random := rand.New(rand.NewPCG(1, 2))
	ranks := []string{RankBetween("", "")}
	for range 500 {
		at := random.IntN(len(ranks) + 1)
		low, high := "", ""
		if at > 0 {
			low = ranks[at-1]
		}
		if at < len(ranks) {
			high = ranks[at]
		}
		ranks = slices.Insert(ranks, at, RankBetween(low, high))
	}
	if !slices.IsSorted(ranks) || len(slices.Compact(slices.Clone(ranks))) != len(ranks) {
		t.Fatal("ranks are not strictly increasing")
	}
	for _, rank := range ranks {
		if len(rank) > 12 {
			t.Errorf("rank %q is longer than expected", rank)
		}
	}

	// Always inserting at the top still gives valid keys.
	top := "V"
	for range 200 {
		next := RankBetween("", top)
		if !ValidRank(next) || next >= top {
			t.Fatalf("RankBetween(\"\", %q) = %q", top, next)
		}
		top = next
	}
}

func TestRanksBetweenSpreadsKeys(t *testing.T) {
	t.Parallel()

	ranks := RanksBetween("a", "", 40)
	if len(ranks) != 40 || !slices.IsSorted(ranks) || ranks[0] <= "a" {
		t.Fatalf("RanksBetween = %v", ranks)
	}
	for _, rank := range ranks {
		if !ValidRank(rank) || len(rank) > 4 {
			t.Errorf("rank %q", rank)
		}
	}
	if got := RanksBetween("", "", 0); len(got) != 0 {
		t.Errorf("no keys asked, got %v", got)
	}
}

func TestValidRank(t *testing.T) {
	t.Parallel()

	for _, rank := range []string{"V", "a0V", "zz", "0001"} {
		if !ValidRank(rank) {
			t.Errorf("%q should be valid", rank)
		}
	}
	for _, rank := range []string{"", "0", "a0", "a b", "é", "a-b", strings.Repeat("V", 65)} {
		if ValidRank(rank) {
			t.Errorf("%q should be invalid", rank)
		}
	}
}

func TestParseTicketReadsRank(t *testing.T) {
	t.Parallel()

	ticket := ParseTicket("FH-1-a", []byte("---\nid: FH-1\nstatus: backlog\ntype: bug\npriority: low\ncreated: 2026-10-01\nrank: a0V\n---\n# A\n"))
	if ticket.Rank != "a0V" || len(ticket.Warnings) != 0 {
		t.Errorf("rank = %q, warnings %v", ticket.Rank, ticket.Warnings)
	}
	bad := ParseTicket("FH-1-a", []byte("---\nid: FH-1\nstatus: backlog\ntype: bug\npriority: low\ncreated: 2026-10-01\nrank: not a rank\n---\n# A\n"))
	if bad.Rank != "" || !slices.ContainsFunc(bad.Warnings, func(w string) bool { return strings.Contains(w, "rank") }) {
		t.Errorf("an invalid rank should be ignored with a warning: %q %v", bad.Rank, bad.Warnings)
	}
}

func ticket(id string, column Column, priority, created, rank string) Ticket {
	return Ticket{ID: id, Column: column, Priority: priority, Created: created, Rank: rank}
}

func ids(tickets []Ticket) []string {
	out := make([]string, len(tickets))
	for index, ticket := range tickets {
		out[index] = ticket.ID
	}
	return out
}

func TestCompareOrderPutsRankedTicketsFirst(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	tickets := []Ticket{
		ticket("FH-1", Backlog, "high", "2026-10-01", ""),
		ticket("FH-2", Backlog, "low", "2026-10-01", "b"),
		ticket("FH-3", Backlog, "medium", "2026-09-01", ""),
		ticket("FH-4", Backlog, "low", "2026-10-02", "a"),
		ticket("FH-10", Backlog, "high", "2026-10-01", ""),
		ticket("FH-5", UpNext, "low", "2026-10-01", ""),
		ticket("FH-6", Backlog, "low", "2026-10-01", "a"),
		{ID: "FH-7", Column: Done, Rank: "a", Modified: now.Add(-time.Hour)},
		{ID: "FH-8", Column: Done, Modified: now},
	}
	slices.SortStableFunc(tickets, CompareOrder)
	// Tied ranks fall back to the default order (FH-6 is older than FH-4).
	want := []string{"FH-6", "FH-4", "FH-2", "FH-1", "FH-10", "FH-3", "FH-5", "FH-8", "FH-7"}
	if got := ids(tickets); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// place applies a plan to column and returns the new order with moving in it.
func place(t *testing.T, column []Ticket, moving Ticket, after string) []string {
	t.Helper()
	writes, ok := PlanPlacement(column, moving.ID, after)
	if !ok {
		t.Fatalf("placement after %q refused", after)
	}
	next := slices.DeleteFunc(slices.Clone(column), func(item Ticket) bool { return item.ID == moving.ID })
	moving.Column = Backlog
	next = append(next, moving)
	for _, write := range writes {
		for index := range next {
			if next[index].ID == write.ID {
				next[index].Rank = write.Rank
			}
		}
	}
	slices.SortStableFunc(next, CompareOrder)
	return ids(next)
}

func TestPlanPlacement(t *testing.T) {
	t.Parallel()

	ranked := []Ticket{
		ticket("FH-1", Backlog, "low", "2026-10-01", "a"),
		ticket("FH-2", Backlog, "low", "2026-10-01", "b"),
		ticket("FH-3", Backlog, "high", "2026-10-01", ""),
		ticket("FH-4", Backlog, "medium", "2026-10-01", ""),
		ticket("FH-5", Backlog, "low", "2026-10-01", ""),
	}
	moving := ticket("FH-9", UpNext, "low", "2026-10-01", "")

	cases := []struct {
		name   string
		column []Ticket
		after  string
		want   []string
		writes int
	}{
		{"top", ranked, "", []string{"FH-9", "FH-1", "FH-2", "FH-3", "FH-4", "FH-5"}, 1},
		{"between ranked", ranked, "FH-1", []string{"FH-1", "FH-9", "FH-2", "FH-3", "FH-4", "FH-5"}, 1},
		{"after the last ranked", ranked, "FH-2", []string{"FH-1", "FH-2", "FH-9", "FH-3", "FH-4", "FH-5"}, 1},
		// Inside the unranked tail, the cards above are ranked in their
		// current order so nothing else moves.
		{"inside the unranked tail", ranked, "FH-4", []string{"FH-1", "FH-2", "FH-3", "FH-4", "FH-9", "FH-5"}, 3},
		{"top of an unranked column", ranked[2:], "", []string{"FH-9", "FH-3", "FH-4", "FH-5"}, 1},
		{"bottom of an unranked column", ranked[2:], "FH-5", []string{"FH-3", "FH-4", "FH-5", "FH-9"}, 4},
		{"empty column", nil, "", []string{"FH-9"}, 1},
	}
	for _, tc := range cases {
		column := slices.Clone(tc.column)
		slices.SortStableFunc(column, CompareOrder)
		if got := place(t, column, moving, tc.after); !slices.Equal(got, tc.want) {
			t.Errorf("%s: order = %v, want %v", tc.name, got, tc.want)
		}
		writes, _ := PlanPlacement(column, moving.ID, tc.after)
		if len(writes) != tc.writes {
			t.Errorf("%s: %d writes, want %d: %v", tc.name, len(writes), tc.writes, writes)
		}
		if writes[len(writes)-1].ID != moving.ID {
			t.Errorf("%s: the moving ticket is written last", tc.name)
		}
	}

	// Reordering within the column ignores the ticket's own place.
	column := slices.Clone(ranked)
	if got := place(t, column, column[0], "FH-2"); !slices.Equal(got, []string{"FH-2", "FH-1", "FH-3", "FH-4", "FH-5"}) {
		t.Errorf("down one = %v", got)
	}

	// Tied ranks (from hand edits) are spread out so the ticket lands between them.
	tied := []Ticket{
		ticket("FH-1", Backlog, "low", "2026-10-01", "b"),
		ticket("FH-2", Backlog, "low", "2026-10-01", "b"),
		ticket("FH-3", Backlog, "low", "2026-10-01", "c"),
	}
	if got := place(t, tied, moving, "FH-1"); !slices.Equal(got, []string{"FH-1", "FH-9", "FH-2", "FH-3"}) {
		t.Errorf("between tied ranks = %v", got)
	}

	// A ticket to follow that is not in the column refuses the placement.
	if _, ok := PlanPlacement(ranked, moving.ID, "FH-404"); ok {
		t.Error("placing after a missing ticket should be refused")
	}
	if _, ok := PlanPlacement(ranked, moving.ID, moving.ID); ok {
		t.Error("placing a ticket after itself should be refused")
	}
}

func TestPlanPlacementRespreadsWhenARankWouldGrowTooLong(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("z", maxRankLength)
	column := []Ticket{
		ticket("FH-1", Backlog, "low", "2026-10-01", "V"),
		ticket("FH-2", Backlog, "low", "2026-10-01", long),
		ticket("FH-3", Backlog, "low", "2026-10-01", ""),
	}
	writes, ok := PlanPlacement(column, "FH-9", "FH-2")
	if !ok {
		t.Fatal("refused")
	}
	for _, write := range writes {
		if !ValidRank(write.Rank) {
			t.Errorf("%s rank %q is not valid", write.ID, write.Rank)
		}
	}
	got := place(t, column, ticket("FH-9", UpNext, "low", "2026-10-01", ""), "FH-2")
	if !slices.Equal(got, []string{"FH-1", "FH-2", "FH-9", "FH-3"}) {
		t.Errorf("order = %v", got)
	}
}
