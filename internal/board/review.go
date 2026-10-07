package board

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Review checklists (REV-3): a review's How to Verify steps are ticked by
// the human who verifies the work. The steps are the first list in the
// section (under ### Steps when it has one); a tick is a task box written
// into the step's own line, so review.md stays the record.

// ReviewStep is one How to Verify step: its text (continuation lines
// joined) and whether it is ticked.
type ReviewStep struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// ReviewParts is a review split around its How to Verify steps, so a reader can
// show the steps as a checklist: Before is everything up to the section's
// heading, Intro the section before the steps, Outro the section after
// them, After the rest. With no steps, Steps is empty and only Before is
// set (to the whole review).
type ReviewParts struct {
	Before, Intro, Outro, After string
	Steps                       []ReviewStep
}

var (
	verifyHeading = regexp.MustCompile(`(?i)^##\s+how to verify\s*$`)
	stepsHeading  = regexp.MustCompile(`(?i)^###\s+steps\s*$`)
	// sectionEnd is a heading of level 1 or 2.
	sectionEnd = regexp.MustCompile(`^#{1,2}(\s|$)`)
	reviewItem = regexp.MustCompile(`^(\d+[.)]|[-*+])(\s+)(\[( |x|X)\](?:\s+|$))?(.*)$`)
)

func isFence(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func indented(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

// reviewLines splits markdown into lines without their \r, leaving out the
// empty line after a final newline.
func reviewLines(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// layout is where a review's How to Verify parts are: its heading, the
// ### Steps heading (-1 without one), the steps block [start, end) and the
// section's end.
type layout struct {
	heading, stepsAt, start, end, section int
}

// reviewLayout finds the How to Verify section and its steps: the first
// list in the section (after ### Steps when it has one), outside code
// fences. Like CommonMark, a list goes on across blank lines when the next
// line is an item or indented, and takes unindented lines straight after
// its text as lazy continuations; a fence or heading ends it.
func reviewLayout(raw []string) (layout, bool) {
	lines := make([]string, len(raw))
	for index, line := range raw {
		lines[index] = strings.TrimRight(line, "\r")
	}
	at := layout{heading: -1, stepsAt: -1, start: -1, section: len(lines)}
	fenced := false
	for index, line := range lines {
		if isFence(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if at.heading < 0 {
			if verifyHeading.MatchString(line) {
				at.heading = index
			}
			continue
		}
		if sectionEnd.MatchString(line) {
			at.section = index
			break
		}
	}
	if at.heading < 0 {
		return at, false
	}
	// The list starts after ### Steps when there is one, else anywhere in
	// the section; either way outside fences.
	for _, stepsOnly := range []bool{true, false} {
		fenced = false
		from := at.heading + 1
		for index := from; index < at.section; index++ {
			line := lines[index]
			if isFence(line) {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			if stepsOnly && at.stepsAt < 0 {
				if stepsHeading.MatchString(line) {
					at.stepsAt = index
				}
				continue
			}
			if reviewItem.MatchString(line) {
				at.start = index
				break
			}
		}
		if at.start >= 0 || at.stepsAt >= 0 {
			break
		}
	}
	if at.start < 0 {
		return at, false
	}
	index, fenced := at.start, false
	for index < at.section {
		line := lines[index]
		switch {
		case fenced:
			fenced = !isFence(line)
		case reviewItem.MatchString(line):
		case strings.TrimSpace(line) == "":
			next := index + 1
			for next < at.section && strings.TrimSpace(lines[next]) == "" {
				next++
			}
			if next == at.section || !reviewItem.MatchString(lines[next]) && !indented(lines[next]) {
				at.end = index
				return at, true
			}
			index = next
			continue
		case indented(line):
			fenced = isFence(line)
		case isFence(line) || strings.HasPrefix(line, "#") || strings.TrimSpace(lines[index-1]) == "":
			at.end = index
			return at, true
		}
		index++
	}
	at.end = index
	return at, true
}

// ParseReview splits a review around its How to Verify steps. A step's
// continuation lines join its text with spaces. The ### Steps heading is
// left out of Intro: it is the checklist's own title.
func ParseReview(markdown string) ReviewParts {
	lines := reviewLines(markdown)
	at, ok := reviewLayout(lines)
	if !ok {
		return ReviewParts{Before: markdown}
	}
	join := func(part []string) string {
		if len(part) == 0 {
			return ""
		}
		return strings.Join(part, "\n") + "\n"
	}
	intro := lines[at.heading+1 : at.start]
	if at.stepsAt >= 0 {
		intro = append(slices.Clone(lines[at.heading+1:at.stepsAt]), lines[at.stepsAt+1:at.start]...)
	}
	review := ReviewParts{
		Before: join(lines[:at.heading]),
		Intro:  join(intro),
		Outro:  join(lines[at.end:at.section]),
		After:  join(lines[at.section:]),
	}
	if !strings.HasSuffix(markdown, "\n") {
		review.After = strings.TrimSuffix(review.After, "\n")
	}
	fenced := false
	for _, raw := range lines[at.start:at.end] {
		line := strings.TrimRight(raw, "\r")
		if match := reviewItem.FindStringSubmatch(line); match != nil && !fenced {
			review.Steps = append(review.Steps, ReviewStep{Text: strings.TrimSpace(match[5]), Done: strings.EqualFold(match[4], "x")})
			continue
		}
		if isFence(line) {
			fenced = !fenced
		}
		if text := strings.TrimSpace(line); text != "" {
			last := &review.Steps[len(review.Steps)-1]
			last.Text = strings.TrimSpace(last.Text + " " + text)
		}
	}
	return review
}

// SetReviewStep ticks or clears step index (from 0) of a review's How to
// Verify, writing a task box into that step's line.
func SetReviewStep(markdown string, index int, done bool) (string, error) {
	if index < 0 {
		return "", fmt.Errorf("the review has no such step")
	}
	lines := strings.Split(markdown, "\n")
	at, ok := reviewLayout(reviewLines(markdown))
	if !ok {
		return "", fmt.Errorf("the review has no How to Verify steps")
	}
	box := "[ ]"
	if done {
		box = "[x]"
	}
	step, fenced := -1, false
	for row := at.start; row < at.end; row++ {
		raw := lines[row]
		line := strings.TrimRight(raw, "\r")
		match := reviewItem.FindStringSubmatch(line)
		if match == nil || fenced {
			if isFence(line) {
				fenced = !fenced
			}
			continue
		}
		if step++; step != index {
			continue
		}
		rewritten := match[1] + match[2] + box
		if match[5] != "" {
			rewritten += " " + match[5]
		}
		lines[row] = rewritten + raw[len(line):]
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("the review has no step %d", index+1)
}
