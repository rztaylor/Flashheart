package setup

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

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
	command = strings.TrimSpace(command)
	var program, rest string
	if quote := command[:min(1, len(command))]; quote == "'" || quote == `"` {
		end := strings.Index(command[1:], quote)
		if end < 0 {
			return false
		}
		program, rest = command[1:end+1], command[end+2:]
	} else {
		program, rest, _ = strings.Cut(command, " ")
	}
	fields := strings.Fields(rest)
	return ownBinary.MatchString(filepath.Base(program)) && len(fields) > 0 && fields[0] == "hook"
}

// Options describe one machine's Claude Code configuration.
type Options struct {
	// Home is the user's home directory; Claude Code's files are under
	// Home/.claude and Home/.claude.json.
	Home string
	// Binary is the absolute path of the flashheart binary (SET-3).
	Binary string
	// Root is passed to every command when it is not the default root.
	Root string
	Now  func() time.Time
	// Claude is the claude CLI that registers the MCP server; empty means
	// none was found and the commands are left for the user to run.
	Claude string
	// Run runs a command; nil runs it with os/exec.
	Run func(name string, args ...string) ([]byte, error)
}

type change struct {
	// path is the file written: for a symlinked settings.json, its target.
	path string
	// backup is its name inside the backup folder.
	backup        string
	before, after []byte // nil: the file is absent / is deleted
}

type move struct{ from, to string }

// Plan is what setup would change. Render shows it; Apply makes it so.
type Plan struct {
	options   Options
	uninstall bool
	backup    string
	files     []change
	moves     []move
	commands  [][]string
	notes     []string
}

func (o Options) claudeDir() string    { return filepath.Join(o.Home, ".claude") }
func (o Options) settingsPath() string { return filepath.Join(o.claudeDir(), "settings.json") }
func (o Options) skillPath() string {
	return filepath.Join(o.claudeDir(), "skills", protocol.SkillName, "SKILL.md")
}
func (o Options) replacedPath() string   { return filepath.Join(o.claudeDir(), "skills", replacedSkill) }
func (o Options) backupsPath() string    { return filepath.Join(o.claudeDir(), backupDir) }
func (o Options) claudeJSONPath() string { return filepath.Join(o.Home, ".claude.json") }

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

func (o Options) hookCommand(event string) string {
	command := protocol.ShellQuote(o.Binary) + " hook claude " + event
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

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// skillBackup is the skill's name inside a backup folder.
var skillBackup = filepath.Join("skills", protocol.SkillName, "SKILL.md")

// readUserSettings reads settings.json, through a symlink to its target (a
// dotfiles manager's file stays a link), and checks the hooks setup edits.
func (o Options) readUserSettings() (string, []byte, *value, error) {
	path := o.settingsPath()
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if path, err = filepath.EvalSymlinks(path); err != nil {
			return "", nil, nil, fmt.Errorf("%s is a link that cannot be followed: %w", o.settingsPath(), err)
		}
	}
	data, tree, err := readSettings(path)
	if err != nil {
		return "", nil, nil, err
	}
	if err := checkHooks(tree); err != nil {
		return "", nil, nil, fmt.Errorf("%s: %w; fix it before running setup", o.settingsPath(), err)
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

// stripOwn removes setup's hooks from the hooks object. With prune, groups
// and events left empty by the removal go too (uninstall); without it they
// stay, so reinstalling keeps every event where it was.
func stripOwn(hooks *value, prune bool) {
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
		if prune && removedAny && len(keptGroups) == 0 {
			hooks.object = slices.Delete(hooks.object, index, index+1)
			index--
		}
	}
}

// installHooks replaces setup's hooks in tree with the current set.
func (o Options) installHooks(tree *value) {
	hooks := tree.get("hooks")
	if hooks == nil || !hooks.isObj {
		hooks = &value{isObj: true}
		tree.set("hooks", hooks)
	}
	stripOwn(hooks, false)
	for _, event := range claudeEvents {
		entry := &value{isObj: true}
		entry.set("type", stringValue("command"))
		entry.set("command", stringValue(o.hookCommand(event)))
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
	o.installHooks(tree)
	return encodeLike(tree, data)
}

// registered reads the user-scope flashheart MCP server from ~/.claude.json
// (read only: Claude Code owns that file).
func (o Options) registered() (command string, args []string, found bool) {
	data, err := readFile(o.claudeJSONPath())
	if err != nil || data == nil {
		return "", nil, false
	}
	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return "", nil, false
	}
	server, found := parsed.MCPServers[serverName]
	return server.Command, server.Args, found
}

func (o Options) newPlan() *Plan {
	suffix := make([]byte, 2)
	_, _ = rand.Read(suffix)
	return &Plan{options: o, backup: filepath.Join(o.backupsPath(), o.now().Format("20060102T150405Z")+"-"+hex.EncodeToString(suffix))}
}

// Install plans Claude Code's configuration for Flashheart (SET-1, SET-2):
// every hook of §5.2, the MCP server, the protocol skill, and moving the
// kanban-tracker skill aside.
func Install(o Options) (*Plan, error) {
	p := o.newPlan()
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
	command, args, found := o.registered()
	switch {
	case found && command == o.Binary && slices.Equal(args, o.serverArgs()):
	case found:
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
func Uninstall(o Options) (*Plan, error) {
	p := o.newPlan()
	p.uninstall = true
	settings, data, tree, err := o.readUserSettings()
	if err != nil {
		return nil, err
	}
	after, restored := o.restoredSettings(data)
	if !restored {
		hooks := tree.get("hooks")
		stripOwn(hooks, true)
		if hooks != nil && hooks.isObj && len(hooks.object) == 0 {
			tree.remove("hooks")
		}
		after = encodeLike(tree, data)
	}
	if data != nil && !bytes.Equal(after, data) {
		p.files = append(p.files, change{path: settings, backup: "settings.json", before: data, after: after})
	}
	// Only setup's own skill goes; one the user had before is put back.
	if skill, err := readFile(o.skillPath()); err != nil {
		return nil, err
	} else if skill != nil && bytes.Contains(skill, []byte("flashheart-protocol:")) {
		var theirs []byte
		if saved := o.latestBackup(skillBackup); saved != "" {
			if data, err := os.ReadFile(saved); err == nil && !bytes.Contains(data, []byte("flashheart-protocol:")) {
				theirs = data
			}
		}
		p.files = append(p.files, change{path: o.skillPath(), backup: skillBackup, before: skill, after: theirs})
	}
	if _, err := os.Stat(o.replacedPath()); errors.Is(err, fs.ErrNotExist) {
		if saved := o.latestBackup(filepath.Join("skills", replacedSkill)); saved != "" {
			p.moves = append(p.moves, move{from: saved, to: o.replacedPath()})
		}
	}
	if _, _, found := o.registered(); found {
		p.commands = append(p.commands, []string{"mcp", "remove", "--scope", "user", serverName})
	}
	return p, nil
}

// restoredSettings returns the backed-up settings when installing over
// them gives exactly the current file. A nil result with restored set means
// the file did not exist before setup.
func (o Options) restoredSettings(current []byte) ([]byte, bool) {
	dir := o.latestBackupDir("settings.json")
	if dir == "" {
		return nil, false
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json.absent")); err == nil {
		if bytes.Equal(o.installedSettings(nil, &value{isObj: true}), current) {
			return nil, true
		}
		return nil, false
	}
	backup, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		return nil, false
	}
	_, tree, err := readSettings(filepath.Join(dir, "settings.json"))
	if err != nil || !bytes.Equal(o.installedSettings(backup, tree), current) {
		return nil, false
	}
	// A backup from before an earlier setup (or a hand-made configuration)
	// may hold Flashheart hooks of its own; those go too.
	_, original, _ := readSettings(filepath.Join(dir, "settings.json"))
	hooks := original.get("hooks")
	stripOwn(hooks, true)
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

// latestBackupDir is the newest backup holding name (or its absent marker).
func (o Options) latestBackupDir(name string) string {
	entries, err := os.ReadDir(o.backupsPath())
	if err != nil {
		return ""
	}
	for index := len(entries) - 1; index >= 0; index-- {
		dir := filepath.Join(o.backupsPath(), entries[index].Name())
		for _, candidate := range []string{name, name + ".absent"} {
			if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
				return dir
			}
		}
	}
	return ""
}

func (o Options) latestBackup(name string) string {
	if dir := o.latestBackupDir(name); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

// Empty reports a plan with nothing to do.
func (p *Plan) Empty() bool { return len(p.files)+len(p.moves)+len(p.commands) == 0 }

func (p *Plan) display(path string) string {
	if rest, ok := strings.CutPrefix(path, p.options.Home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	return path
}

func (p *Plan) commandLine(args []string) string {
	quoted := []string{"claude"}
	for _, arg := range args {
		quoted = append(quoted, protocol.ShellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

// Render prints the plan: a diff per file, the moves and the commands.
func (p *Plan) Render(w io.Writer) {
	if p.Empty() {
		if p.uninstall {
			fmt.Fprintln(w, "Flashheart is not set up for Claude Code; nothing to remove.")
		} else {
			fmt.Fprintln(w, "Claude Code is already configured for Flashheart; nothing to change.")
		}
		return
	}
	for _, f := range p.files {
		switch {
		case f.before == nil:
			fmt.Fprintf(w, "Create %s:\n", p.display(f.path))
		case f.after == nil:
			fmt.Fprintf(w, "Delete %s.\n\n", p.display(f.path))
			continue
		default:
			fmt.Fprintf(w, "Change %s:\n", p.display(f.path))
		}
		fmt.Fprintln(w, unifiedDiff(p.display(f.path), f.before, f.after))
	}
	for _, m := range p.moves {
		fmt.Fprintf(w, "Move %s to %s.\n", p.display(m.from), p.display(m.to))
	}
	for _, args := range p.commands {
		fmt.Fprintf(w, "Run: %s\n", p.commandLine(args))
	}
	for _, note := range p.notes {
		fmt.Fprintln(w, note)
	}
}

// Apply makes the plan so: files are backed up, then replaced atomically;
// moves go into the backup; MCP commands run through the claude CLI, or
// are printed for the user when there is none.
func (p *Plan) Apply(w io.Writer) error {
	if p.Empty() {
		p.Render(w)
		return nil
	}
	// Nothing is written if a file changed since the plan was made (Claude
	// Code rewrites settings.json itself): the plan would undo that change.
	for _, f := range p.files {
		current, err := readFile(f.path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, f.before) {
			return fmt.Errorf("%s changed since the plan was made; run flashheart setup again", p.display(f.path))
		}
	}
	if err := os.MkdirAll(p.backup, 0o700); err != nil {
		return fmt.Errorf("create backup folder: %w", err)
	}
	for _, f := range p.files {
		saved := filepath.Join(p.backup, f.backup)
		if err := os.MkdirAll(filepath.Dir(saved), 0o700); err != nil {
			return err
		}
		if f.before == nil {
			if err := os.WriteFile(saved+".absent", nil, 0o600); err != nil {
				return err
			}
		} else if err := os.WriteFile(saved, f.before, 0o600); err != nil {
			return err
		}
	}
	for _, f := range p.files {
		if f.after == nil {
			if err := os.Remove(f.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if f.backup == skillBackup {
				_ = os.Remove(filepath.Dir(f.path)) // the skill's folder, when empty
			}
			fmt.Fprintf(w, "Deleted %s\n", p.display(f.path))
			continue
		}
		if err := writeAtomic(f.path, f.after); err != nil {
			return err
		}
		fmt.Fprintf(w, "Wrote %s\n", p.display(f.path))
	}
	for _, m := range p.moves {
		if err := os.MkdirAll(filepath.Dir(m.to), 0o755); err != nil {
			return err
		}
		if err := os.Rename(m.from, m.to); err != nil {
			return fmt.Errorf("move %s: %w", p.display(m.from), err)
		}
		fmt.Fprintf(w, "Moved %s to %s\n", p.display(m.from), p.display(m.to))
	}
	if len(p.files) > 0 {
		fmt.Fprintf(w, "Backups are in %s\n", p.display(p.backup))
	}
	if len(p.commands) == 0 {
		return nil
	}
	if p.options.Claude == "" {
		fmt.Fprintln(w, "The claude command was not found, so register the MCP server yourself:")
		for _, args := range p.commands {
			fmt.Fprintf(w, "  %s\n", p.commandLine(args))
		}
		return nil
	}
	run := p.options.Run
	if run == nil {
		run = func(name string, args ...string) ([]byte, error) { return exec.Command(name, args...).CombinedOutput() }
	}
	for index, args := range p.commands {
		if output, err := run(p.options.Claude, args...); err != nil {
			var rest []string
			for _, remaining := range p.commands[index:] {
				rest = append(rest, "  "+p.commandLine(remaining))
			}
			return fmt.Errorf("%s failed: %v\n%s\nRun the rest yourself:\n%s", p.commandLine(args), err, strings.TrimSpace(string(output)), strings.Join(rest, "\n"))
		}
		fmt.Fprintf(w, "Ran %s\n", p.commandLine(args))
	}
	fmt.Fprintln(w, "Start a new Claude Code session to pick up the changes.")
	return nil
}

func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".flashheart-*")
	if err != nil {
		return err
	}
	_, writeErr := temporary.Write(data)
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, syncErr, closeErr, os.Chmod(temporary.Name(), mode)); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}

// FindClaude returns the claude CLI: on PATH, else the newest copy bundled
// with the Claude desktop app on macOS, else "".
func FindClaude(home string) string {
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
