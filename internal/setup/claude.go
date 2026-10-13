package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// claudeEvents are the Claude Code hook events Flashheart handles
// (agent-protocol §5.2), in the order setup registers them.
var claudeEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure",
	"PermissionRequest", "PermissionDenied", "Notification", "TaskCreated", "TaskCompleted",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "Stop", "SessionEnd",
}

const (
	// toolMatcher limits PreToolUse to Flashheart's own tools (§5.2).
	toolMatcher = "mcp__flashheart__.*"
	hookTimeout = 5
	serverName  = "flashheart"
	// replacedSkill is the skill the protocol skill replaces (SET-2).
	replacedSkill = "kanban-tracker"
	backupDir     = "flashheart-backup"
)

// ownBinary names a flashheart build: flashheart, flashheart-dev, …
var ownBinary = regexp.MustCompile(`^flashheart[-_.A-Za-z0-9]*$`)

// ownsCommand reports whether a hook command is setup's own: its program
// (the first word, quoted or not) is a flashheart binary and its first
// argument is hook (§5.4). Everything else is the user's.
func ownsCommand(command string) bool {
	program, rest, ok := splitProgram(command)
	fields := strings.Fields(rest)
	return ok && ownBinary.MatchString(filepath.Base(program)) && len(fields) > 0 && fields[0] == "hook"
}

// splitProgram splits a hook command into its program, unquoted as a
// shell would (single and double quotes, backslash escapes, so the
// '\” that shellQuote writes reads back), and the rest.
func splitProgram(command string) (program, rest string, ok bool) {
	command = strings.TrimSpace(command)
	var word strings.Builder
	var quote rune
	escaped := false
	for index, r := range command {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\\' && quote != '\'':
			escaped = true
		case quote == '"':
			if r == '"' {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			return word.String(), command[index+1:], true
		default:
			word.WriteRune(r)
		}
	}
	if quote != 0 || escaped {
		return "", "", false
	}
	return word.String(), "", true
}

// Claude is Claude Code as a setup Agent: the hooks of §5.2 in
// ~/.claude/settings.json, the MCP server registered through the claude
// CLI, and the protocol skill under ~/.claude/skills.
type Claude struct{}

var _ SkillRefresher = Claude{}

// Name returns "claude".
func (Claude) Name() string { return "claude" }

// Title returns "Claude Code".
func (Claude) Title() string { return "Claude Code" }

// claudeTarget is the plan engine's view of Claude Code on this machine.
func (o Options) claudeTarget() target {
	return target{title: Claude{}.Title(), command: Claude{}.Name(), backups: o.backupsPath(), options: o}
}

func (o Options) claudeDir() string    { return filepath.Join(o.Home, ".claude") }
func (o Options) settingsPath() string { return filepath.Join(o.claudeDir(), "settings.json") }
func (o Options) skillPath() string {
	return filepath.Join(o.claudeDir(), "skills", protocol.SkillName, "SKILL.md")
}
func (o Options) replacedPath() string   { return filepath.Join(o.claudeDir(), "skills", replacedSkill) }
func (o Options) backupsPath() string    { return filepath.Join(o.claudeDir(), backupDir) }
func (o Options) claudeJSONPath() string { return filepath.Join(o.Home, ".claude.json") }

// hookCommand is Claude Code's hook command for event.
func (o Options) hookCommand(event string) string { return o.agentHookCommand(Claude{}.Name(), event) }

// agentHookCommand runs this binary's hook handler for an agent's event
// (agent-protocol §5.1).
func (o Options) agentHookCommand(agent, event string) string {
	command := protocol.ShellQuote(o.Binary) + " hook " + agent + " " + event
	if o.Root != "" {
		command += " --root " + protocol.ShellQuote(o.Root)
	}
	return command
}

// serverArgs is the MCP server's argument list.
func (o Options) serverArgs() []string {
	args := []string{"mcp"}
	if o.Root != "" {
		args = append(args, "--root", o.Root)
	}
	return args
}

// skillBackup is the skill's name inside a backup folder.
var skillBackup = filepath.Join("skills", protocol.SkillName, "SKILL.md")

// readUserSettings reads settings.json and checks the hooks setup edits.
func (o Options) readUserSettings() (string, []byte, *value, error) {
	return readHooksFile(o.settingsPath())
}

// readHooksFile reads a JSON file of hooks (Claude Code's settings.json,
// Codex's hooks.json), through a symlink to its target (a dotfiles
// manager's file stays a link), and checks the hooks setup edits. It
// returns the path to write, the content and its tree.
func readHooksFile(name string) (string, []byte, *value, error) {
	path := name
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if path, err = filepath.EvalSymlinks(path); err != nil {
			return "", nil, nil, fmt.Errorf("%s is a link that cannot be followed: %w", name, err)
		}
	}
	data, tree, err := readSettings(path)
	if err != nil {
		return "", nil, nil, err
	}
	if err := checkHooks(tree); err != nil {
		return "", nil, nil, fmt.Errorf("%s: %w; fix it before running setup", name, err)
	}
	return path, data, tree, nil
}

// checkHooks refuses hook settings in a shape setup does not understand,
// rather than replacing them.
func checkHooks(tree *value) error {
	hooks := tree.get("hooks")
	if hooks == nil {
		return nil
	}
	if !hooks.isObj {
		return errors.New("hooks is not an object")
	}
	for _, event := range hooks.object {
		if !event.value.isArray {
			return fmt.Errorf("hooks.%s is not a list", event.key)
		}
		for _, group := range event.value.array {
			if !group.isObj {
				return fmt.Errorf("hooks.%s has an entry that is not an object", event.key)
			}
			if entries := group.get("hooks"); entries != nil && !entries.isArray {
				return fmt.Errorf("hooks.%s has an entry whose hooks is not a list", event.key)
			}
		}
	}
	return nil
}

func readSettings(path string) ([]byte, *value, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, nil, err
	}
	data, tree, err := parseSettings(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %w", path, err)
	}
	return data, tree, nil
}

// parseSettings parses settings content; empty content is an empty object.
func parseSettings(data []byte) ([]byte, *value, error) {
	if data == nil || len(bytes.TrimSpace(data)) == 0 {
		return data, &value{isObj: true}, nil
	}
	tree, err := parseJSON(data)
	if err == nil && !tree.isObj {
		err = errors.New("not an object")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("is not a JSON object; fix it before running setup: %w", err)
	}
	return data, tree, nil
}

// encodeLike encodes tree in the style of the file it came from.
func encodeLike(tree *value, original []byte) []byte {
	out := tree.encode(indentOf(original))
	if len(original) > 0 && !bytes.HasSuffix(original, []byte("\n")) {
		out = bytes.TrimSuffix(out, []byte("\n"))
	}
	if bytes.Contains(original, []byte("\r\n")) {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	}
	return out
}

// stripOwn removes setup's hooks from the hooks object. Groups left empty
// by the removal go, and so do events, except those in keep: reinstalling
// keeps every event it registers where it was, and drops one it no longer
// registers (uninstall keeps none).
func stripOwn(hooks *value, keep []string) {
	if hooks == nil || !hooks.isObj {
		return
	}
	for index := 0; index < len(hooks.object); index++ {
		groups := hooks.object[index].value
		if !groups.isArray {
			continue
		}
		removedAny := false
		var keptGroups []*value
		for _, group := range groups.array {
			entries := group.get("hooks")
			if entries == nil || !entries.isArray {
				keptGroups = append(keptGroups, group)
				continue
			}
			var kept []*value
			for _, entry := range entries.array {
				if ownsCommand(entry.get("command").text()) {
					removedAny = true
					continue
				}
				kept = append(kept, entry)
			}
			if len(kept) == 0 && len(entries.array) > 0 {
				continue // a group that held only Flashheart's hooks
			}
			entries.array = kept
			keptGroups = append(keptGroups, group)
		}
		groups.array = keptGroups
		if removedAny && len(keptGroups) == 0 && !slices.Contains(keep, hooks.object[index].key) {
			hooks.object = slices.Delete(hooks.object, index, index+1)
			index--
		}
	}
}

// installHooks replaces setup's hooks in tree with one for each event,
// running command(event); PreToolUse is limited to Flashheart's tools.
func installHooks(tree *value, events []string, command func(string) string) {
	hooks := tree.get("hooks")
	if hooks == nil || !hooks.isObj {
		hooks = &value{isObj: true}
		tree.set("hooks", hooks)
	}
	stripOwn(hooks, events)
	for _, event := range events {
		entry := &value{isObj: true}
		entry.set("type", stringValue("command"))
		entry.set("command", stringValue(command(event)))
		entry.set("timeout", numberValue(hookTimeout))
		group := &value{isObj: true}
		if event == "PreToolUse" {
			group.set("matcher", stringValue(toolMatcher))
		}
		group.set("hooks", &value{isArray: true, array: []*value{entry}})
		groups := hooks.get(event)
		if groups == nil || !groups.isArray {
			groups = &value{isArray: true}
			hooks.set(event, groups)
		}
		groups.array = append(groups.array, group)
	}
}

func (o Options) installedSettings(data []byte, tree *value) []byte {
	installHooks(tree, claudeEvents, o.hookCommand)
	return encodeLike(tree, data)
}

// registration is the flashheart MCP server as Claude Code has it.
type registration struct {
	// found is set for a user-scope server, setup's own, which runs
	// command with args.
	found   bool
	command string
	args    []string
	// scope names the other scope the server was found in instead (local
	// or project): the user's own registration, which setup leaves alone.
	scope string
	// problem says why the registration could not be read.
	problem string
}

// registered reads the flashheart MCP server through the claude CLI
// (claude mcp get), so setup never parses ~/.claude.json, which Claude
// Code owns. Without the CLI it reads that file, read only, which tells
// only the user scope.
func (o Options) registered() registration {
	if o.CLI == "" {
		return o.registeredInFile()
	}
	output, err := o.run(o.CLI, "mcp", "get", serverName)
	text := string(output)
	if err != nil {
		if strings.Contains(text, "No MCP server named") {
			return registration{}
		}
		return registration{problem: fmt.Sprintf("Could not ask claude for the MCP registration (%v); setup registers the server again.", err)}
	}
	var r registration
	scope := ""
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		value = strings.TrimSpace(value)
		switch {
		case !ok:
		case key == "Scope":
			scope = value
		case key == "Command":
			r.command, r.found = value, true
		case key == "Args":
			// The CLI prints the arguments joined by spaces, so ours are
			// compared as printed: a root with spaces stays one argument.
			if value == strings.Join(o.serverArgs(), " ") {
				r.args = o.serverArgs()
			} else {
				r.args = strings.Fields(value)
			}
		}
	}
	// Only the user-scope registration is setup's; one in a project or
	// local scope is the user's own and is left alone. The CLI names the
	// scope first, as in "Local config (private to you in this project)".
	if !strings.HasPrefix(scope, "User") {
		name, _, _ := strings.Cut(scope, " ")
		return registration{scope: strings.ToLower(name)}
	}
	return r
}

// registeredInFile reads the registration from ~/.claude.json when the
// claude CLI is not available.
func (o Options) registeredInFile() registration {
	data, err := readFile(o.claudeJSONPath())
	if err != nil {
		return registration{problem: fmt.Sprintf("Could not read ~/.claude.json (%v), so the MCP registration is unknown.", err)}
	}
	if data == nil {
		return registration{}
	}
	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return registration{problem: "Could not read ~/.claude.json (it is not valid JSON), so the MCP registration is unknown."}
	}
	server, found := parsed.MCPServers[serverName]
	return registration{found: found, command: server.Command, args: server.Args}
}

// Install plans Claude Code's configuration for Flashheart (SET-1, SET-2):
// every hook of §5.2, the MCP server, the protocol skill, and moving the
// kanban-tracker skill aside.
func (Claude) Install(o Options) (*Plan, error) {
	p := newPlan(o.claudeTarget(), false)
	settings, data, tree, err := o.readUserSettings()
	if err != nil {
		return nil, err
	}
	if after := o.installedSettings(data, tree); !bytes.Equal(after, data) {
		p.files = append(p.files, change{path: settings, backup: "settings.json", before: data, after: after})
	}
	skill, err := readFile(o.skillPath())
	if err != nil {
		return nil, err
	}
	if want := []byte(protocol.Skill()); !bytes.Equal(skill, want) {
		p.files = append(p.files, change{path: o.skillPath(), backup: skillBackup, before: skill, after: want})
	}
	if info, err := os.Stat(o.replacedPath()); err == nil && info.IsDir() {
		p.moves = append(p.moves, move{from: o.replacedPath(), to: filepath.Join(p.backup, "skills", replacedSkill)})
		p.notes = append(p.notes, "The Flashheart skill replaces kanban-tracker; --uninstall puts it back.")
	}
	config, _ := json.Marshal(map[string]any{"type": "stdio", "command": o.Binary, "args": o.serverArgs()})
	server := o.registered()
	if server.problem != "" {
		p.notes = append(p.notes, server.problem)
	}
	switch {
	case server.found && server.command == o.Binary && slices.Equal(server.args, o.serverArgs()):
	case server.found:
		p.commands = append(p.commands, []string{"mcp", "remove", "--scope", "user", serverName})
		fallthrough
	default:
		p.commands = append(p.commands, []string{"mcp", "add-json", "--scope", "user", serverName, string(config)})
	}
	return p, nil
}

// Uninstall plans the reverse of Install. When settings.json is just as
// setup left it, the backup taken then is restored byte for byte;
// otherwise only setup's own hooks are removed.
func (Claude) Uninstall(o Options) (*Plan, error) {
	p := newPlan(o.claudeTarget(), true)
	settings, data, tree, err := o.readUserSettings()
	if err != nil {
		return nil, err
	}
	after := uninstalledHooks(o.backupsPath(), "settings.json", data, tree, o.installedSettings)
	if data != nil && !bytes.Equal(after, data) {
		p.files = append(p.files, change{path: settings, backup: "settings.json", before: data, after: after})
	}
	// Only setup's own skill goes; one the user had before is put back.
	if skill, err := readFile(o.skillPath()); err != nil {
		return nil, err
	} else if skill != nil && bytes.Contains(skill, []byte("flashheart-protocol:")) {
		var theirs []byte
		if saved := latestBackup(o.backupsPath(), skillBackup); saved != "" {
			if data, err := os.ReadFile(saved); err == nil && !bytes.Contains(data, []byte("flashheart-protocol:")) {
				theirs = data
			}
		}
		p.files = append(p.files, change{path: o.skillPath(), backup: skillBackup, before: skill, after: theirs, removeDir: true})
	}
	if _, err := os.Stat(o.replacedPath()); errors.Is(err, fs.ErrNotExist) {
		if saved := latestBackup(o.backupsPath(), filepath.Join("skills", replacedSkill)); saved != "" {
			p.moves = append(p.moves, move{from: saved, to: o.replacedPath()})
		}
	}
	server := o.registered()
	if server.problem != "" {
		p.notes = append(p.notes, server.problem)
	}
	if server.found {
		p.commands = append(p.commands, []string{"mcp", "remove", "--scope", "user", serverName})
	}
	return p, nil
}

// uninstalledHooks is a hooks file without setup's hooks. When the file is
// just as setup left it (installing over the newest backup, with install,
// gives exactly data), the backup is restored byte for byte, or nil when
// the file did not exist before setup; otherwise only setup's own hooks
// are removed.
func uninstalledHooks(backups, name string, data []byte, tree *value, install func([]byte, *value) []byte) []byte {
	if after, restored := restoredHooks(backups, name, data, install); restored {
		return after
	}
	hooks := tree.get("hooks")
	stripOwn(hooks, nil)
	if hooks != nil && hooks.isObj && len(hooks.object) == 0 {
		tree.remove("hooks")
	}
	return encodeLike(tree, data)
}

// restoredHooks returns the backed-up file when installing over it gives
// exactly the current one. A nil result with restored set means the file
// did not exist before setup.
func restoredHooks(backups, name string, current []byte, install func([]byte, *value) []byte) ([]byte, bool) {
	dir := latestBackupDir(backups, name)
	if dir == "" {
		return nil, false
	}
	if _, err := os.Stat(filepath.Join(dir, name+".absent")); err == nil {
		if bytes.Equal(install(nil, &value{isObj: true}), current) {
			return nil, true
		}
		return nil, false
	}
	backup, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, false
	}
	_, tree, err := readSettings(filepath.Join(dir, name))
	if err != nil || !bytes.Equal(install(backup, tree), current) {
		return nil, false
	}
	// A backup from before an earlier setup (or a hand-made configuration)
	// may hold Flashheart hooks of its own; those go too.
	_, original, _ := readSettings(filepath.Join(dir, name))
	hooks := original.get("hooks")
	stripOwn(hooks, nil)
	if hooks != nil && hooks.isObj && len(hooks.object) == 0 {
		original.remove("hooks")
	}
	if stripped := encodeLike(original, backup); !original.equal(mustParse(backup)) {
		return stripped, true
	}
	return backup, true
}

func mustParse(data []byte) *value {
	_, tree, _ := parseSettings(data)
	return tree
}

// FindCLI returns the claude CLI: on PATH, else the newest copy bundled
// with the Claude desktop app on macOS, else "".
func (Claude) FindCLI(home string) string {
	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}
	matches, _ := filepath.Glob(filepath.Join(home, "Library", "Application Support", "Claude", "claude-code", "*", "*", "claude.app", "Contents", "MacOS", "claude"))
	if len(matches) == 0 {
		return ""
	}
	slices.Sort(matches)
	return matches[len(matches)-1]
}
