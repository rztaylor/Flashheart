package setup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// codexEvents are the Codex hook events Flashheart registers
// (agent-protocol §5.3): every event Codex sends that the adapter acts on,
// in the order setup registers them.
var codexEvents = []string{
	"SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "Stop", "Interrupt", "SessionEnd",
}

// codexIgnoredEvents are the events Codex sends that the adapter does
// nothing with, so setup does not register them: a hook there would only
// start Flashheart and need trusting. PreToolUse is one, since Flashheart's
// tool calls are attributed by the session in their _meta (D45), not by
// stamping; setup removes the PreToolUse hook earlier versions wrote.
var codexIgnoredEvents = []string{"PreToolUse"}

// trustNote says what Codex needs before it runs hooks setup wrote.
const trustNote = "Codex runs these hooks only once you trust them: start codex, type /hooks and trust each Flashheart hook " +
	"(again whenever setup changes them). The ChatGPT app uses the trust you give in the CLI."

// Codex is Codex (the CLI, the IDE extension and the ChatGPT app) as a
// setup Agent: the flashheart MCP server in config.toml, the hooks of
// §5.3 in hooks.json, and the section of the global AGENTS.md that points
// at the skill, all in $CODEX_HOME (~/.codex), and the protocol skill in
// ~/.agents/skills, where Codex reads the user's skills.
type Codex struct{}

// Name returns "codex".
func (Codex) Name() string { return "codex" }

// Title returns "Codex".
func (Codex) Title() string { return "Codex" }

// FindCLI returns "": setup edits Codex's files itself and runs no
// command.
func (Codex) FindCLI(string) string { return "" }

func (o Options) codexHome() string {
	if o.CodexHome != "" {
		return o.CodexHome
	}
	return filepath.Join(o.Home, ".codex")
}

func (o Options) codexConfigPath() string  { return filepath.Join(o.codexHome(), "config.toml") }
func (o Options) codexHooksPath() string   { return filepath.Join(o.codexHome(), "hooks.json") }
func (o Options) codexAgentsPath() string  { return filepath.Join(o.codexHome(), "AGENTS.md") }
func (o Options) codexBackupsPath() string { return filepath.Join(o.codexHome(), backupDir) }

func (o Options) codexTarget() target {
	return target{title: Codex{}.Title(), command: Codex{}.Name(), backups: o.codexBackupsPath(), options: o}
}

func (o Options) codexHookCommand(event string) string {
	return o.agentHookCommand(Codex{}.Name(), event)
}

func (o Options) installedCodexHooks(data []byte, tree *value) []byte {
	installHooks(tree, codexEvents, o.codexHookCommand)
	return encodeLike(tree, data)
}

// unreadable says a Codex file cannot be edited safely.
func (o Options) unreadable(path string, err error) error {
	return fmt.Errorf("%s: %v; fix it before running setup", o.display(path), err)
}

// Install plans Codex's configuration for Flashheart (SET-1, SET-2): the
// MCP server, every hook of §5.3, the protocol skill and the section of
// the global AGENTS.md.
func (Codex) Install(o Options) (*Plan, error) {
	p := newPlan(o.codexTarget(), false)
	config, err := readFile(o.codexConfigPath())
	if err != nil {
		return nil, err
	}
	after, err := o.installedConfig(config)
	if err != nil {
		return nil, o.unreadable(o.codexConfigPath(), err)
	}
	if !bytes.Equal(after, config) {
		p.files = append(p.files, change{path: o.codexConfigPath(), backup: "config.toml", before: config, after: after})
	}

	hooksPath, data, tree, err := readHooksFile(o.codexHooksPath())
	if err != nil {
		return nil, err
	}
	if after := o.installedCodexHooks(data, tree); !bytes.Equal(after, data) {
		p.files = append(p.files, change{path: hooksPath, backup: "hooks.json", before: data, after: after})
		p.notes = append(p.notes, trustNote)
	}

	if err := p.planSkill(o.codexSkillDir()); err != nil {
		return nil, err
	}
	if err := p.planSection(o.codexAgentsPath(), Codex{}.Name(), o.codexBackupsPath(), false); err != nil {
		return nil, err
	}
	return p, nil
}

// Uninstall plans the reverse of Install: only Flashheart's table, hooks,
// skill and section go. hooks.json is restored byte for byte when nothing
// changed since setup wrote it; a file setup created is deleted when
// nothing else is left in it.
func (Codex) Uninstall(o Options) (*Plan, error) {
	p := newPlan(o.codexTarget(), true)
	backups := o.codexBackupsPath()
	config, err := readFile(o.codexConfigPath())
	if err != nil {
		return nil, err
	}
	after, err := uninstalledConfig(config)
	if err != nil {
		return nil, o.unreadable(o.codexConfigPath(), err)
	}
	if config != nil && !bytes.Equal(after, config) {
		p.files = append(p.files, change{path: o.codexConfigPath(), backup: "config.toml", before: config, after: emptiedFile(backups, "config.toml", after)})
	}

	hooksPath, data, tree, err := readHooksFile(o.codexHooksPath())
	if err != nil {
		return nil, err
	}
	if after := uninstalledHooks(backups, "hooks.json", data, tree, o.installedCodexHooks); data != nil && !bytes.Equal(after, data) {
		p.files = append(p.files, change{path: hooksPath, backup: "hooks.json", before: data, after: after})
	}

	if err := p.planSharedSkillRemoval(o.codexSkillDir(), backups, Claude{}.Title(), o.claudeConfigured()); err != nil {
		return nil, err
	}
	if err := p.planSection(o.codexAgentsPath(), Codex{}.Name(), backups, true); err != nil {
		return nil, err
	}
	return p, nil
}

// emptiedFile is a file's content after uninstalling: nil (delete it) when
// nothing is left and setup created it, else the content, kept even when
// empty.
func emptiedFile(backups, name string, after []byte) []byte {
	if len(after) == 0 {
		if dir := latestBackupDir(backups, name); dir != "" {
			if _, err := os.Stat(filepath.Join(dir, name+".absent")); err == nil {
				return nil
			}
		}
		return []byte{}
	}
	return after
}

var _ ProtocolRefresher = Codex{}

// RefreshProtocol keeps what setup installed for Codex current at session
// start (agent-protocol §5.4, D53, D89): the skill folder, and the marked
// section of the global AGENTS.md when it has one. A section is setup's
// mark, so beside one the skill is written even when it is missing: an
// install from before the skill folder had the whole protocol in its
// section, and shrinking that to a pointer without the skill would lose
// it. The skill is written first, so a failure leaves the old section. It
// returns a line for each that it rewrote, for the session's context. The
// Codex session-start hook calls it; nothing else does.
func (Codex) RefreshProtocol(o Options) (string, error) {
	path, err := linkTarget(o.codexAgentsPath())
	if err != nil {
		return "", err
	}
	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	_, _, sectioned, _ := findSection(data)
	var notes []string
	updated, err := refreshSkill(o.codexSkillDir(), sectioned)
	if updated {
		notes = append(notes, protocol.SkillUpdatedNote)
	}
	if err != nil {
		return strings.Join(notes, "\n"), err
	}
	note, _, err := refreshSection(o.codexAgentsPath(), Codex{}.Name())
	if note != "" {
		notes = append(notes, note)
	}
	return strings.Join(notes, "\n"), err
}
