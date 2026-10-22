package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rztaylor/flashheart/internal/health"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// The short always-read section (protocol.AlwaysRead) goes into the global
// instructions file of each agent, Claude Code's CLAUDE.md and Codex's
// AGENTS.md, between these markers (SET-2, D89); the rest of the file is
// the user's.
const (
	sectionEnd = "<!-- END FLASHHEART -->"
	// A line starting with a prefix is a marker, so a changed comment in
	// a later version still finds the section.
	beginPrefix = "<!-- BEGIN FLASHHEART"
	endPrefix   = "<!-- END FLASHHEART"
)

// sectionBegin is the begin marker setup writes for agent.
func sectionBegin(agent string) string {
	return beginPrefix + ": written by flashheart setup " + agent + "; edits here are replaced -->"
}

// section is the marked section setup writes for agent.
func section(agent string) string {
	return sectionBegin(agent) + "\n" + protocol.AlwaysRead() + sectionEnd + "\n"
}

// linkTarget is the file to read and write for path: path itself, or the
// target of a symlink there, so a dotfiles manager's link stays a link.
func linkTarget(path string) (string, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("%s is a link that cannot be followed: %w", path, err)
		}
		return target, nil
	}
	return path, nil
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

// installedSection is a global instructions file with agent's current
// section: the old section replaced where it was, else the section
// appended after a blank line.
func installedSection(data []byte, agent string) ([]byte, error) {
	start, end, found, err := findSection(data)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if found {
		return []byte(text[:start] + section(agent) + text[end:]), nil
	}
	if text == "" {
		return []byte(section(agent)), nil
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + "\n" + section(agent)), nil
}

// uninstalledSection is a global instructions file without the section and
// the blank line before it.
func uninstalledSection(data []byte) ([]byte, error) {
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

// sameSection reports whether a marked section is agent's current one,
// ignoring a missing newline at the end of the file.
func sameSection(found, agent string) bool {
	return strings.TrimSuffix(found, "\n") == strings.TrimSuffix(section(agent), "\n")
}

// planSection adds to p the change that installs (or, uninstalling,
// removes) agent's section in the global instructions file at path,
// written through a symlink to its target; backups is the agent's backup
// folder.
func (p *Plan) planSection(path, agent, backups string, uninstall bool) error {
	o := p.target.options
	target, err := linkTarget(path)
	if err != nil {
		return err
	}
	data, err := readFile(target)
	if err != nil {
		return err
	}
	name := filepath.Base(path)
	var after []byte
	if uninstall {
		after, err = uninstalledSection(data)
	} else {
		after, err = installedSection(data, agent)
	}
	switch {
	case err != nil:
		return o.unreadable(path, err)
	case uninstall && data == nil, string(after) == string(data):
		return nil
	case uninstall:
		after = emptiedFile(backups, name, after)
	}
	p.files = append(p.files, change{path: target, backup: name, before: data, after: after})
	return nil
}

// refreshSection keeps agent's marked section of the global instructions
// file at path current (agent-protocol §5.4, D53): when the file has one
// marked section and it differs from this binary's, only that section is
// rewritten, atomically, through a symlink to its target. A file without a
// section is never given one, so an uninstall stays uninstalled, and a
// file whose markers do not pair is left for setup and doctor to report.
// It returns the note for the session's context when it wrote, and
// whether the file has a section at all.
func refreshSection(path, agent string) (note string, present bool, err error) {
	target, err := linkTarget(path)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	start, end, found, err := findSection(data)
	if err != nil || !found {
		return "", false, nil
	}
	if sameSection(string(data[start:end]), agent) {
		return "", true, nil
	}
	if err := writeAtomic(target, []byte(string(data[:start])+section(agent)+string(data[end:]))); err != nil {
		return "", true, err
	}
	return protocol.SectionUpdatedNote(path), true, nil
}

// diagnoseSection checks agent's section in the global instructions file
// at path. A missing section is a warning: the skill still works, but a
// session may not think to use it.
func (o Options) diagnoseSection(path, agent, remedy string) health.Finding {
	where := o.display(path)
	label := "Global " + filepath.Base(path) + ": "
	target, err := linkTarget(path)
	var data []byte
	if err == nil {
		data, err = readFile(target)
	}
	var start, end int
	found := false
	if err == nil {
		start, end, found, err = findSection(data)
	}
	switch {
	case err != nil:
		return health.Finding{Level: health.Problem, Text: fmt.Sprintf("%scannot read %s: %v; fix it, then %s", label, where, err, remedy)}
	case !found:
		return health.Finding{Level: health.Warning, Text: fmt.Sprintf("%sno Flashheart section in %s, so sessions may overlook the flashheart skill; %s", label, where, remedy)}
	case !sameSection(string(data[start:end]), agent):
		return health.Finding{Level: health.Warning, Text: fmt.Sprintf("%sFlashheart section out of date in %s; %s", label, where, remedy)}
	}
	return health.Finding{Level: health.OK, Text: fmt.Sprintf("%sFlashheart section up to date in %s", label, where)}
}
