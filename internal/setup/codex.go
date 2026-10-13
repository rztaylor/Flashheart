package setup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// codexEvents are the Codex hook events Flashheart registers
// (agent-protocol §5.3): every event Codex sends, in the order setup
// registers them. PreToolUse is limited to Flashheart's own tools.
var codexEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PermissionRequest", "PostToolUse",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "Stop", "Interrupt", "SessionEnd",
}

// trustNote says what Codex needs before it runs hooks setup wrote.
const trustNote = "Codex runs these hooks only once you trust them: start codex, type /hooks and trust each Flashheart hook " +
	"(again whenever setup changes them). The ChatGPT app uses the trust you give in the CLI."

// Codex is Codex (the CLI, the IDE extension and the ChatGPT app) as a
// setup Agent: the flashheart MCP server in config.toml, the hooks of
// §5.3 in hooks.json, and the protocol text in the global AGENTS.md, all
// in $CODEX_HOME (~/.codex).
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
// MCP server, every hook of §5.3 and the protocol text.
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

	agents, err := readFile(o.codexAgentsPath())
	if err != nil {
		return nil, err
	}
	if after, err = installedAgents(agents); err != nil {
		return nil, o.unreadable(o.codexAgentsPath(), err)
	}
	if !bytes.Equal(after, agents) {
		p.files = append(p.files, change{path: o.codexAgentsPath(), backup: "AGENTS.md", before: agents, after: after})
	}
	return p, nil
}

// Uninstall plans the reverse of Install: only Flashheart's table, hooks
// and section go. hooks.json is restored byte for byte when nothing
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

	agents, err := readFile(o.codexAgentsPath())
	if err != nil {
		return nil, err
	}
	if after, err = uninstalledAgents(agents); err != nil {
		return nil, o.unreadable(o.codexAgentsPath(), err)
	}
	if agents != nil && !bytes.Equal(after, agents) {
		p.files = append(p.files, change{path: o.codexAgentsPath(), backup: "AGENTS.md", before: agents, after: emptiedFile(backups, "AGENTS.md", after)})
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
