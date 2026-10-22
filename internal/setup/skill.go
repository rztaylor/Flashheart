package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rztaylor/flashheart/internal/health"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// The protocol skill is a folder (D89): SKILL.md, the guide, and the
// reference pages it names (protocol.SkillFiles). Claude Code reads it
// from ~/.claude/skills/flashheart, Codex from ~/.agents/skills/flashheart;
// both get the same files.

func (o Options) claudeSkillDir() string {
	return filepath.Join(o.claudeDir(), "skills", protocol.SkillName)
}

func (o Options) codexSkillDir() string {
	return filepath.Join(o.Home, ".agents", "skills", protocol.SkillName)
}

// skillBackup is a skill file's name inside a backup folder.
func skillBackup(path string) string {
	return filepath.Join("skills", protocol.SkillName, path)
}

// ownSkill reports whether a SKILL.md is the protocol skill: frontmatter
// naming it, with Flashheart's protocol version in its metadata.
func ownSkill(data []byte) bool {
	rest, found := bytes.CutPrefix(data, []byte("---\nname: "+protocol.SkillName+"\n"))
	if !found {
		return false
	}
	frontmatter, _, closed := bytes.Cut(rest, []byte("\n---\n"))
	return closed && bytes.Contains(frontmatter, []byte("\n  flashheart-protocol: "))
}

// planSkill adds to p the changes that write every skill file in dir that
// differs from this binary's.
func (p *Plan) planSkill(dir string) error {
	for _, file := range protocol.SkillFiles() {
		path := filepath.Join(dir, file.Path)
		current, err := readFile(path)
		if err != nil {
			return err
		}
		if want := []byte(file.Text); !bytes.Equal(current, want) {
			p.files = append(p.files, change{path: path, backup: skillBackup(file.Path), before: current, after: want})
		}
	}
	return nil
}

// planSkillRemoval adds to p the changes that remove the skill in dir when
// it is Flashheart's: its pages, then SKILL.md, each folder going once
// empty. A SKILL.md of the user's that setup replaced is put back from
// the newest backup in backups.
func (p *Plan) planSkillRemoval(dir, backups string) error {
	skill, err := readFile(filepath.Join(dir, "SKILL.md"))
	if err != nil || skill == nil || !bytes.Contains(skill, []byte("flashheart-protocol:")) {
		return err
	}
	files := protocol.SkillFiles()
	for _, file := range files[1:] {
		path := filepath.Join(dir, file.Path)
		page, err := readFile(path)
		if err != nil {
			return err
		}
		if page != nil {
			p.files = append(p.files, change{path: path, backup: skillBackup(file.Path), before: page, removeDir: true})
		}
	}
	var theirs []byte
	if saved := latestBackup(backups, skillBackup("SKILL.md")); saved != "" {
		if data, err := os.ReadFile(saved); err == nil && !bytes.Contains(data, []byte("flashheart-protocol:")) {
			theirs = data
		}
	}
	p.files = append(p.files, change{path: filepath.Join(dir, "SKILL.md"), backup: skillBackup("SKILL.md"), before: skill, after: theirs, removeDir: true})
	return nil
}

// sameSkillFolder reports whether Claude Code's and Codex's skill folders
// are one, as when ~/.claude/skills and ~/.agents/skills link to the same
// folder of the user's.
func (o Options) sameSkillFolder() bool {
	claude, errClaude := filepath.EvalSymlinks(filepath.Dir(o.claudeSkillDir()))
	codex, errCodex := filepath.EvalSymlinks(filepath.Dir(o.codexSkillDir()))
	return errClaude == nil && errCodex == nil && claude == codex
}

// claudeConfigured reports whether setup's hooks or section are in Claude
// Code's files.
func (o Options) claudeConfigured() bool {
	if _, _, tree, err := o.readUserSettings(); err == nil && len(ownedEventsBesides(tree.get("hooks"), nil)) > 0 {
		return true
	}
	return o.hasSection(o.claudeMDPath())
}

// codexConfigured reports whether setup's server or section are in
// Codex's files.
func (o Options) codexConfigured() bool {
	if config, err := readFile(o.codexConfigPath()); err == nil {
		if server, err := findServer(config); err == nil && server.header != nil {
			return true
		}
	}
	return o.hasSection(o.codexAgentsPath())
}

func (o Options) hasSection(path string) bool {
	target, err := linkTarget(path)
	if err != nil {
		return false
	}
	data, err := readFile(target)
	_, _, found, _ := findSection(data)
	return err == nil && found
}

// planSharedSkillRemoval is planSkillRemoval for one agent's folder, except
// that a folder the other agent shares stays while that agent is set up,
// with a note saying so.
func (p *Plan) planSharedSkillRemoval(dir, backups, other string, otherConfigured bool) error {
	o := p.target.options
	if o.sameSkillFolder() && otherConfigured {
		if skill, err := readFile(filepath.Join(dir, "SKILL.md")); err == nil && skill != nil {
			p.notes = append(p.notes, fmt.Sprintf("The skill in %s is also %s's (the two skills folders are one), so it stays until you uninstall %s too.", o.display(dir), other, other))
		}
		return nil
	}
	return p.planSkillRemoval(dir, backups)
}

// refreshSkill keeps an installed protocol skill current (agent-protocol
// §5.4): when dir's SKILL.md has Flashheart's frontmatter, every file of
// the skill that differs from this binary's, or is missing, is written
// atomically. The skill is Flashheart's own, so edits are overwritten as
// setup overwrites them. A missing SKILL.md is never created unless
// create is set, so an uninstall stays uninstalled; one that is not
// Flashheart's is never touched. It reports whether it wrote.
func refreshSkill(dir string, create bool) (bool, error) {
	current, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if !create {
			return false, nil
		}
	case err != nil:
		return false, err
	case !ownSkill(current):
		return false, nil
	}
	wrote := false
	for _, file := range protocol.SkillFiles() {
		path := filepath.Join(dir, file.Path)
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return wrote, err
		}
		if string(data) == file.Text {
			continue
		}
		if err := writeAtomic(path, []byte(file.Text)); err != nil {
			return wrote, err
		}
		wrote = true
	}
	return wrote, nil
}

// RefreshSkill keeps Claude Code's installed skill current (refreshSkill),
// never installing it. It reports whether it wrote.
func (Claude) RefreshSkill(home string) (bool, error) {
	return refreshSkill(Options{Home: home}.claudeSkillDir(), false)
}

// RefreshProtocol keeps what setup installed for Claude Code current at
// session start (agent-protocol §5.4): the skill folder, and the marked
// section of the global CLAUDE.md when it has one. It returns a line for
// each that it rewrote, for the session's context. The session-start hook
// calls it; nothing else does.
func (c Claude) RefreshProtocol(o Options) (string, error) {
	var notes []string
	updated, err := c.RefreshSkill(o.Home)
	if updated {
		notes = append(notes, protocol.SkillUpdatedNote)
	}
	if err != nil {
		return strings.Join(notes, "\n"), err
	}
	note, _, err := refreshSection(o.claudeMDPath(), Claude{}.Name())
	if note != "" {
		notes = append(notes, note)
	}
	return strings.Join(notes, "\n"), err
}

// diagnoseSkill checks the skill folder in dir against this binary's
// skill: a missing SKILL.md is a problem, any file missing or different a
// warning naming them.
func (o Options) diagnoseSkill(dir, remedy string) health.Finding {
	where := o.display(dir)
	var stale []string
	for index, file := range protocol.SkillFiles() {
		data, err := readFile(filepath.Join(dir, file.Path))
		switch {
		case err != nil:
			return health.Finding{Level: health.Problem, Text: fmt.Sprintf("Skill: cannot read %s: %v", o.display(filepath.Join(dir, file.Path)), err)}
		case data == nil && index == 0:
			return health.Finding{Level: health.Problem, Text: fmt.Sprintf("Skill: not installed in %s; %s", where, remedy)}
		case string(data) != file.Text:
			stale = append(stale, file.Path)
		}
	}
	if len(stale) > 0 {
		return health.Finding{Level: health.Warning, Text: fmt.Sprintf("Skill: out of date in %s (%s); %s", where, strings.Join(stale, ", "), remedy)}
	}
	return health.Finding{Level: health.OK, Text: "Skill: up to date in " + where}
}
