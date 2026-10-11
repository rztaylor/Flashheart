package events

import (
	"slices"
	"time"
)

// Span is a stretch of time, ends included: a shell command's window.
type Span struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Contains reports whether t falls inside the span, ends included.
func (s Span) Contains(t time.Time) bool { return !t.Before(s.From) && !t.After(s.To) }

// AddSpan appends s to spans, which are in order, merging it into the last
// span when they meet; beyond limit (when positive) the oldest two merge.
// It is the one merge of shell windows, for hooks' pending activity and
// for runs.
func AddSpan(spans []Span, s Span, limit int) []Span {
	if last := len(spans) - 1; last >= 0 && !s.From.After(spans[last].To) {
		if s.To.After(spans[last].To) {
			spans[last].To = s.To
		}
		return spans
	}
	spans = append(spans, s)
	if limit > 0 && len(spans) > limit {
		spans[1].From = spans[0].From
		spans = slices.Delete(spans, 0, 1)
	}
	return spans
}

// MergeSpans sorts spans and merges those that meet.
func MergeSpans(spans []Span) []Span {
	sorted := slices.SortedFunc(slices.Values(spans), func(a, b Span) int { return a.From.Compare(b.From) })
	var merged []Span
	for _, s := range sorted {
		merged = AddSpan(merged, s, 0)
	}
	return merged
}
