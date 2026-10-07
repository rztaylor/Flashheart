package board

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// Editable ticket fields shared by the UI and the MCP tools (EDIT-6). The
// title is the H1; the rest are frontmatter keys.
var (
	ListFields   = []string{"depends-on", "depends-on-workstreams", "tags"}
	ScalarFields = []string{"title", "status", "type", "priority", "created", "branch", "workstream"}
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)

// ValidateField checks one value of an editable field; list fields are
// checked item by item.
func ValidateField(key, value string) error {
	switch key {
	case "title":
		if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 300 {
			return errors.New("title must be 1 to 300 characters")
		}
	case "status":
		if _, ok := ParseColumn(value); !ok {
			return fmt.Errorf("%q is not a column", value)
		}
	case "type":
		if !slices.Contains(TicketTypes, value) {
			return fmt.Errorf("type must be one of %s", strings.Join(TicketTypes, ", "))
		}
	case "priority":
		if !slices.Contains(Priorities, value) {
			return fmt.Errorf("priority must be one of %s", strings.Join(Priorities, ", "))
		}
	case "created":
		if !datePattern.MatchString(value) {
			return errors.New("created must be a date like 2026-10-05")
		}
	case "workstream", "depends-on-workstreams":
		if value != "" && !slugPattern.MatchString(value) {
			return fmt.Errorf("%q is not a workstream slug", value)
		}
	case "depends-on":
		if _, _, ok := ParseID(value); !ok {
			return fmt.Errorf("%q is not a ticket id like FH-42", value)
		}
	case "branch", "tags":
		if strings.ContainsAny(value, "\n\r") || utf8.RuneCountInString(value) > 200 {
			return fmt.Errorf("%s must be one line of at most 200 characters", key)
		}
	}
	return nil
}

// ReviewWarnings are what a move into Ready to review warns about: a
// missing review file, a review with no evidence (agent-protocol §11) and
// unticked criteria (EDIT-3). They never prevent the move.
func ReviewWarnings(ticket Ticket, review string, hasReview bool) []string {
	var warnings []string
	switch {
	case !hasReview:
		warnings = append(warnings, "There is no review file yet.")
	case !ReviewHasEvidence(review):
		warnings = append(warnings, "The review shows no evidence: add screenshots of what changed, or say under ## Evidence why there are none.")
	}
	open := 0
	for _, criterion := range ticket.Criteria {
		if !criterion.Done {
			open++
		}
	}
	switch {
	case open == 1:
		warnings = append(warnings, "1 acceptance criterion is not ticked.")
	case open > 1:
		warnings = append(warnings, fmt.Sprintf("%d acceptance criteria are not ticked.", open))
	}
	return warnings
}

// BlockedStartNote is the ## Notes line recording why a blocked ticket was
// started anyway (EDIT-2).
func BlockedStartNote(reason string) string {
	return "Started while blocked: " + OneLine(reason, 500)
}

// OneLine joins whitespace and keeps at most limit characters.
func OneLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if runes := []rune(value); len(runes) > limit {
		value = string(runes[:limit])
	}
	return value
}

var (
	reviewImage     = regexp.MustCompile(`!\[[^\]]*\]\([^)\s]+\)`)
	reviewTemplated = regexp.MustCompile(`^<[^>]*>$`)
)

// ReviewHasEvidence reports whether a review shows what changed: an image
// anywhere, or an ## Evidence section that says something beyond the
// template's placeholder (such as why there is no visible change).
func ReviewHasEvidence(review string) bool {
	if reviewImage.MatchString(review) {
		return true
	}
	section, ok := mdfile.FindSection(review, "Evidence")
	if !ok {
		return false
	}
	for _, line := range strings.Split(section.Content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !reviewTemplated.MatchString(line) {
			return true
		}
	}
	return false
}
