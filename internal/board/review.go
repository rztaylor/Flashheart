package board

import (
	"fmt"
	"regexp"
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
	reviewItem    = regexp.MustCompile(`^(\d+[.)]|[-*+])(\s+)(\[( |x|X)\]\s+)?(.*)$`)
)

// reviewLayout finds the How to Verify section's heading, the steps block
// [start, end) and the section's end; ok is false without steps.
func reviewLayout(lines []string) (heading, start, end, section int, ok bool) {
	heading = -1
	fenced := false
	for index, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if heading < 0 {
			if verifyHeading.MatchString(line) {
				heading, section = index, len(lines)
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = index
			break
		}
	}
	if heading < 0 {
		return 0, 0, 0, 0, false
	}
	from := heading + 1
	for index := from; index < section; index++ {
		if stepsHeading.MatchString(strings.TrimRight(lines[index], "\r")) {
			from = index + 1
			break
		}
	}
	start = -1
	for index := from; index < section; index++ {
		if reviewItem.MatchString(strings.TrimRight(lines[index], "\r")) {
			start = index
			break
		}
	}
	if start < 0 {
		return 0, 0, 0, 0, false
	}
	end = start
	for end < section {
		line := strings.TrimRight(lines[end], "\r")
		continuation := strings.TrimSpace(line) != "" && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"))
		if !reviewItem.MatchString(line) && !continuation {
			break
		}
		end++
	}
	return heading, start, end, section, true
}

// ParseReview splits a review around its How to Verify steps.
func ParseReview(markdown string) ReviewParts {
	lines := strings.Split(markdown, "\n")
	heading, start, end, section, ok := reviewLayout(lines)
	if !ok {
		return ReviewParts{Before: markdown}
	}
	join := func(part []string) string {
		if len(part) == 0 {
			return ""
		}
		return strings.Join(part, "\n") + "\n"
	}
	review := ReviewParts{
		Before: join(lines[:heading]),
		Intro:  join(lines[heading+1 : start]),
		Outro:  join(lines[end:section]),
		After:  strings.Join(lines[section:], "\n"),
	}
	for _, raw := range lines[start:end] {
		line := strings.TrimRight(raw, "\r")
		if match := reviewItem.FindStringSubmatch(line); match != nil && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			review.Steps = append(review.Steps, ReviewStep{Text: strings.TrimSpace(match[5]), Done: strings.EqualFold(match[4], "x")})
			continue
		}
		last := &review.Steps[len(review.Steps)-1]
		last.Text += "\n" + strings.TrimSpace(line)
	}
	return review
}

// SetReviewStep ticks or clears step index (from 0) of a review's How to
// Verify, writing a task box into that step's line.
func SetReviewStep(markdown string, index int, done bool) (string, error) {
	lines := strings.Split(markdown, "\n")
	_, start, end, _, ok := reviewLayout(lines)
	if !ok {
		return "", fmt.Errorf("the review has no How to Verify steps")
	}
	box := "[ ]"
	if done {
		box = "[x]"
	}
	step := -1
	for at := start; at < end; at++ {
		raw := lines[at]
		line := strings.TrimRight(raw, "\r")
		match := reviewItem.FindStringSubmatch(line)
		if match == nil || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if step++; step != index {
			continue
		}
		rewritten := match[1] + match[2] + box + " " + match[5]
		lines[at] = rewritten + raw[len(line):]
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("the review has no step %d", index+1)
}
