package setup

import (
	"bytes"
	"errors"
	"io/fs"
	"os"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// RefreshSkill keeps an installed protocol skill current (agent-protocol
// §5.4): when ~/.claude/skills/flashheart/SKILL.md exists, carries
// Flashheart's frontmatter and differs from this binary's skill, it is
// rewritten atomically. The skill is Flashheart's own, so edits are
// overwritten as setup overwrites them; a missing skill is never created, so
// an uninstall stays uninstalled. It reports whether it wrote. The Claude
// Code session-start hook calls it; nothing else does.
func RefreshSkill(home string) (bool, error) {
	path := Options{Home: home}.skillPath()
	current, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	want := []byte(protocol.Skill())
	if bytes.Equal(current, want) || !ownSkill(current) {
		return false, nil
	}
	if err := writeAtomic(path, want); err != nil {
		return false, err
	}
	return true, nil
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
