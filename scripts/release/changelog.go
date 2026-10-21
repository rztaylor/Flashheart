package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Release is one released section of the changelog.
type Release struct {
	Version string // X.Y.Z, without the tag's "v"
	Date    string // YYYY-MM-DD
	Notes   string // the section's body, the release notes
}

// MaxNotes is the most characters GitHub accepts in a release's notes.
const MaxNotes = 125000

// ErrNoRelease means the changelog has no released section yet.
var ErrNoRelease = errors.New("the changelog has no released section (## [X.Y.Z] - YYYY-MM-DD)")

var (
	releasedHeading   = regexp.MustCompile(`^## \[((?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*))\] - ([0-9]{4}-[0-9]{2}-[0-9]{2})$`)
	unreleasedHeading = regexp.MustCompile(`^## (?:Unreleased|\[Unreleased\])$`)
	linkDefinition    = regexp.MustCompile(`^\[[^\]]+\]:\s*\S`)
)

// NewestRelease returns the first released section of a Keep a Changelog
// file: its version, date and notes. Every level-2 heading must be
// Unreleased or `## [X.Y.Z] - YYYY-MM-DD`; any other is an error naming its
// line, so a mistyped heading never lets an older version be picked. Link
// reference definitions are left out of the notes.
func NewestRelease(changelog string) (Release, error) {
	var (
		found    *Release
		notes    []string
		inFence  bool
		inNewest bool
	)
	lines := strings.Split(strings.ReplaceAll(changelog, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(strings.TrimLeft(line, " "), "```") {
			inFence = !inFence
		}
		if !inFence && (line == "##" || strings.HasPrefix(line, "## ")) {
			inNewest = false
			if unreleasedHeading.MatchString(line) {
				continue
			}
			match := releasedHeading.FindStringSubmatch(line)
			if match == nil {
				return Release{}, fmt.Errorf("line %d: heading %q is neither Unreleased nor ## [X.Y.Z] - YYYY-MM-DD", i+1, line)
			}
			if _, err := time.Parse("2006-01-02", match[2]); err != nil {
				return Release{}, fmt.Errorf("line %d: heading %q has an impossible date", i+1, line)
			}
			if found == nil {
				found = &Release{Version: match[1], Date: match[2]}
				inNewest = true
			}
			continue
		}
		if inNewest && (inFence || !linkDefinition.MatchString(line)) {
			notes = append(notes, line)
		}
	}
	if found == nil {
		return Release{}, ErrNoRelease
	}
	body := strings.Trim(strings.Join(notes, "\n"), "\n")
	if strings.TrimSpace(body) == "" {
		return Release{}, fmt.Errorf("section %s has no notes", found.Version)
	}
	if n := utf8.RuneCountInString(body) + 1; n > MaxNotes {
		return Release{}, fmt.Errorf("section %s is %d characters; a GitHub release's notes hold at most %d", found.Version, n, MaxNotes)
	}
	found.Notes = body + "\n"
	return *found, nil
}
