package board

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

// HandoffNote is what a ## Handoff section says (board-format §Handoff
// section): when, by which run and on which branch it was written, and its
// lists. RenderHandoff and ParseHandoff round-trip: an empty Files or
// Questions list is written as the item None and read back as empty, and
// empty Done and Next lists are left out. A hand-written section without
// the header line reads with a zero time and no run or branch.
type HandoffNote struct {
	// Updated is when it was written, to the minute, in UTC.
	Updated time.Time
	// By is the writing run's short id; Branch the branch it worked on.
	By, Branch                   string
	Done, Next, Files, Questions []string
}

// handoffLists are the section's lists in order, with their labels.
func (h *HandoffNote) handoffLists() []struct {
	label string
	items *[]string
	none  bool
} {
	return []struct {
		label string
		items *[]string
		none  bool
	}{{"Done", &h.Done, false}, {"Next", &h.Next, false}, {"Files", &h.Files, true}, {"Open questions", &h.Questions, true}}
}

const (
	handoffTime = "2006-01-02 15:04"
	handoffNone = "None"
)

var handoffHeader = regexp.MustCompile(`^_Updated (\d{4}-\d{2}-\d{2} \d{2}:\d{2}) UTC by (.+?) \(run\)(?: on (.+?))?\._$`)

// RenderHandoff is the content of a ## Handoff section.
func RenderHandoff(h HandoffNote) string {
	header := "_Updated " + h.Updated.UTC().Format(handoffTime) + " UTC by " + h.By + " (run)"
	if h.Branch != "" {
		header += " on " + h.Branch
	}
	lines := []string{header + "._", ""}
	for _, list := range h.handoffLists() {
		items := *list.items
		if len(items) == 0 && list.none {
			items = []string{handoffNone}
		}
		if len(items) == 0 {
			continue
		}
		lines = append(lines, "**"+list.label+"**", "")
		for _, item := range items {
			lines = append(lines, "- "+item)
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ParseHandoff reads a ## Handoff section's content.
func ParseHandoff(content string) HandoffNote {
	var h HandoffNote
	for _, line := range strings.Split(content, "\n") {
		if match := handoffHeader.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			h.Updated, _ = time.Parse(handoffTime, match[1])
			h.By, h.Branch = match[2], match[3]
			break
		}
	}
	for _, list := range h.handoffLists() {
		items := handoffList(content, list.label)
		if list.none && slices.Equal(items, []string{handoffNone}) {
			items = nil
		}
		*list.items = items
	}
	return h
}

var boldLabel = regexp.MustCompile(`^\*\*([^*]+)\*\*\s*$`)

// handoffList returns the bullet items under a **Label** line.
func handoffList(content, label string) []string {
	var items []string
	inList := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := boldLabel.FindStringSubmatch(trimmed); match != nil {
			inList = strings.EqualFold(strings.TrimSpace(match[1]), label)
			continue
		}
		if inList && (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")) {
			items = append(items, strings.TrimSpace(trimmed[2:]))
		}
	}
	return items
}
