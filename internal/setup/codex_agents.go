package setup

import (
	"errors"
	"strings"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// The protocol text goes into Codex's global AGENTS.md between these
// markers (SET-2); the rest of the file is the user's.
const (
	agentsBegin = "<!-- BEGIN FLASHHEART: written by flashheart setup codex; edits here are replaced -->"
	agentsEnd   = "<!-- END FLASHHEART -->"
	// A line starting with a prefix is a marker, so a changed comment in
	// a later version still finds the section.
	beginPrefix = "<!-- BEGIN FLASHHEART"
	endPrefix   = "<!-- END FLASHHEART"
)

// agentsSection is the marked protocol section setup writes.
func agentsSection() string {
	return agentsBegin + "\n" + protocol.Body() + agentsEnd + "\n"
}

// findSection returns the marked section's span, from the start of its
// begin line to after its end line; found is false when there is none.
func findSection(data []byte) (start, end int, found bool, err error) {
	text := string(data)
	begin := -1
	for at := 0; at < len(text); {
		next := lineEnd(text, at)
		line := text[at:next]
		switch {
		case strings.HasPrefix(line, beginPrefix):
			if begin >= 0 || found {
				return 0, 0, false, errors.New("has more than one Flashheart section; keep one between the markers")
			}
			begin = at
		case strings.HasPrefix(line, endPrefix):
			if begin < 0 {
				return 0, 0, false, errors.New("has a Flashheart end marker without its begin marker")
			}
			start, end, found = begin, next, true
			begin = -1
		}
		at = next
	}
	if begin >= 0 {
		return 0, 0, false, errors.New("has a Flashheart begin marker without its end marker")
	}
	return start, end, found, nil
}

// installedAgents is AGENTS.md with the current protocol section: the old
// section replaced where it was, else the section appended after a blank
// line.
func installedAgents(data []byte) ([]byte, error) {
	start, end, found, err := findSection(data)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if found {
		return []byte(text[:start] + agentsSection() + text[end:]), nil
	}
	if text == "" {
		return []byte(agentsSection()), nil
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + "\n" + agentsSection()), nil
}

// uninstalledAgents is AGENTS.md without the protocol section and the
// blank line before it.
func uninstalledAgents(data []byte) ([]byte, error) {
	start, end, found, err := findSection(data)
	if err != nil || !found {
		return data, err
	}
	before := string(data[:start])
	if strings.HasSuffix(before, "\n\n") {
		before = before[:len(before)-1]
	}
	return []byte(before + string(data[end:])), nil
}
