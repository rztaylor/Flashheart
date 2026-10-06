package board

import (
	"cmp"
	"slices"
	"strings"
)

// Manual card order (EDIT-9). A ticket's optional `rank` frontmatter field is
// a fractional index: a string of base-62 digits read as the fraction
// 0.d1d2…, compared as plain strings and never ending in '0', so a key fits
// between any two others. Within a column ranked tickets come first in rank
// order; unranked tickets follow in the default order (priority, created,
// id). Done keeps most recently finished first and ignores ranks.

const rankDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// maxRankLength bounds a rank read from a file.
const maxRankLength = 64

// ValidRank reports whether rank is a usable fractional index.
func ValidRank(rank string) bool {
	if rank == "" || len(rank) > maxRankLength || rank[len(rank)-1] == '0' {
		return false
	}
	for index := range len(rank) {
		if strings.IndexByte(rankDigits, rank[index]) < 0 {
			return false
		}
	}
	return true
}

// RankBetween returns a rank strictly between a and b, where "" is the
// start (for a) or the end (for b). a must sort before b.
func RankBetween(a, b string) string {
	if b != "" {
		// Keep the shared prefix; a is padded with zeros where it is shorter.
		n := 0
		for n < len(b) {
			digit := byte('0')
			if n < len(a) {
				digit = a[n]
			}
			if digit != b[n] {
				break
			}
			n++
		}
		if n > 0 {
			rest := ""
			if n < len(a) {
				rest = a[n:]
			}
			return b[:n] + RankBetween(rest, b[n:])
		}
	}
	low := 0
	if a != "" {
		low = strings.IndexByte(rankDigits, a[0])
	}
	high := len(rankDigits)
	if b != "" {
		high = strings.IndexByte(rankDigits, b[0])
	}
	if high-low > 1 {
		return string(rankDigits[(low+high)/2])
	}
	// The first digits are adjacent. b's first digit alone sorts below b
	// (whose further digits are not all zeros) and above a.
	if len(b) > 1 {
		return b[:1]
	}
	rest := ""
	if a != "" {
		rest = a[1:]
	}
	return string(rankDigits[low]) + RankBetween(rest, "")
}

// RanksBetween returns n increasing ranks between a and b, spread by
// bisection so they stay short.
func RanksBetween(a, b string, n int) []string {
	if n <= 0 {
		return nil
	}
	middle := RankBetween(a, b)
	half := n / 2
	return slices.Concat(RanksBetween(a, middle, half), []string{middle}, RanksBetween(middle, b, n-half-1))
}

var priorityOrder = map[string]int{"high": 0, "medium": 1, "low": 2}

// CompareOrder orders tickets for the board: by column in workflow order,
// then by rank (ranked first), then by priority, created date and id. Done
// lists the most recently modified first.
func CompareOrder(a, b Ticket) int {
	if c := cmp.Compare(slices.Index(Columns, a.Column), slices.Index(Columns, b.Column)); c != 0 {
		return c
	}
	if a.Column == Done {
		if c := b.Modified.Compare(a.Modified); c != 0 {
			return c
		}
		return CompareIDs(a.ID, b.ID)
	}
	if (a.Rank != "") != (b.Rank != "") {
		if a.Rank != "" {
			return -1
		}
		return 1
	}
	if c := cmp.Compare(a.Rank, b.Rank); c != 0 {
		return c
	}
	priority := func(t Ticket) int {
		if rank, ok := priorityOrder[t.Priority]; ok {
			return rank
		}
		return len(priorityOrder)
	}
	if c := cmp.Compare(priority(a), priority(b)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Created, b.Created); c != 0 {
		return c
	}
	return CompareIDs(a.ID, b.ID)
}

// CompareIDs orders ids by key, then number.
func CompareIDs(a, b string) int {
	ak, an, _ := ParseID(a)
	bk, bn, _ := ParseID(b)
	if c := cmp.Compare(ak, bk); c != 0 {
		return c
	}
	return cmp.Compare(an, bn)
}

// RankWrite is a rank to record on one ticket.
type RankWrite struct {
	ID, Rank string
}

// PlanPlacement ranks moving so it directly follows after ("" for the top)
// in column, the column's tickets in CompareOrder (moving may be among
// them). Usually only moving is written. When it lands inside the unranked
// tail, the unranked tickets above it are ranked first in their current
// order, and tied ranks above it are spread out, so no other ticket changes
// place. The writes come in column order, moving last. It returns false when
// after is not in column.
func PlanPlacement(column []Ticket, moving, after string) ([]RankWrite, bool) {
	others := slices.DeleteFunc(slices.Clone(column), func(t Ticket) bool { return t.ID == moving })
	at := 0
	if after != "" {
		found := slices.IndexFunc(others, func(t Ticket) bool { return t.ID == after })
		if found < 0 {
			return nil, false
		}
		at = found + 1
	}
	ranked := slices.IndexFunc(others, func(t Ticket) bool { return t.Rank == "" })
	if ranked < 0 {
		ranked = len(others)
	}

	// run is the tickets to rank, in order, ending with moving.
	var run []Ticket
	low, high := "", ""
	switch {
	case at > ranked:
		run = others[ranked:at]
		if ranked > 0 {
			low = others[ranked-1].Rank
		}
	default:
		if at > 0 {
			low = others[at-1].Rank
		}
		if at < ranked {
			high = others[at].Rank
		}
		if high != "" && low >= high {
			run, low = others[:at], ""
		}
	}
	ranks := RanksBetween(low, high, len(run)+1)
	// Ranks squeezed into one gap again and again grow long; past the limit
	// the whole column is spread out afresh, keeping its order.
	if slices.ContainsFunc(ranks, func(rank string) bool { return len(rank) > maxRankLength }) {
		run = slices.Insert(slices.Clone(others), at, Ticket{ID: moving})
		ranks = RanksBetween("", "", len(run))
		writes := make([]RankWrite, 0, len(run))
		for index, ticket := range run {
			if ticket.ID == moving || ticket.Rank != ranks[index] {
				writes = append(writes, RankWrite{ID: ticket.ID, Rank: ranks[index]})
			}
		}
		// The moving ticket is written last.
		slices.SortStableFunc(writes, func(a, b RankWrite) int {
			return boolOrder(a.ID == moving, b.ID == moving)
		})
		return writes, true
	}
	writes := make([]RankWrite, 0, len(ranks))
	for index, ticket := range run {
		writes = append(writes, RankWrite{ID: ticket.ID, Rank: ranks[index]})
	}
	return append(writes, RankWrite{ID: moving, Rank: ranks[len(ranks)-1]}), true
}

func boolOrder(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}
