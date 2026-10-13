package setup

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// target is the agent a plan configures, in the terms the engine needs.
type target struct {
	// title names the agent in output.
	title string
	// command is the agent's CLI as the user types it; the plan's commands
	// run through Options.CLI.
	command string
	// backups is the folder that holds a timestamped backup per apply.
	backups string
	options Options
}

type change struct {
	// path is the file written: for a symlinked file, its target.
	path string
	// backup is its name inside the backup folder.
	backup        string
	before, after []byte // nil: the file is absent / is deleted
	// removeDir also removes the file's folder, when empty, on delete.
	removeDir bool
}

type move struct{ from, to string }

// Plan is what setup would change for an agent. Render shows it; Apply
// makes it so.
type Plan struct {
	target    target
	uninstall bool
	backup    string
	files     []change
	moves     []move
	commands  [][]string
	notes     []string
}

func newPlan(t target, uninstall bool) *Plan {
	suffix := make([]byte, 2)
	_, _ = rand.Read(suffix)
	return &Plan{target: t, uninstall: uninstall, backup: filepath.Join(t.backups, t.options.now().Format("20060102T150405Z")+"-"+hex.EncodeToString(suffix))}
}

// Empty reports a plan with nothing to do.
func (p *Plan) Empty() bool { return len(p.files)+len(p.moves)+len(p.commands) == 0 }

func (p *Plan) display(path string) string { return p.target.options.display(path) }

// display shows a path under the home directory as ~/….
func (o Options) display(path string) string {
	if rest, ok := strings.CutPrefix(path, o.Home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	return path
}

func (p *Plan) commandLine(args []string) string {
	quoted := []string{p.target.command}
	for _, arg := range args {
		quoted = append(quoted, protocol.ShellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

// Render prints the plan: a diff per file, the moves, the commands and
// the notes.
func (p *Plan) Render(w io.Writer) {
	p.RenderChanges(w)
	for _, note := range p.notes {
		fmt.Fprintln(w, note)
	}
}

// RenderChanges prints the plan without its notes, for showing it before
// Apply, which prints them once it has written.
func (p *Plan) RenderChanges(w io.Writer) {
	if p.Empty() {
		if p.uninstall {
			fmt.Fprintf(w, "Flashheart is not set up for %s; nothing to remove.\n", p.target.title)
		} else {
			fmt.Fprintf(w, "%s is already configured for Flashheart; nothing to change.\n", p.target.title)
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
}

// Apply makes the plan so: files are backed up, then replaced atomically;
// moves go into the backup; commands run through the agent's CLI, or are
// printed for the user when there is none. The plan's notes follow.
func (p *Plan) Apply(w io.Writer) error {
	if p.Empty() {
		p.Render(w)
		return nil
	}
	// Nothing is written if a file changed since the plan was made (an
	// agent may rewrite its settings itself): the plan would undo that.
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
			if f.removeDir {
				_ = os.Remove(filepath.Dir(f.path)) // only when empty
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
	if err := p.runCommands(w); err != nil {
		return err
	}
	for _, note := range p.notes {
		fmt.Fprintln(w, note)
	}
	fmt.Fprintf(w, "Start a new %s session to pick up the changes.\n", p.target.title)
	return nil
}

// runCommands runs the plan's commands through the agent's CLI, or prints
// them for the user when there is none.
func (p *Plan) runCommands(w io.Writer) error {
	options := p.target.options
	if len(p.commands) > 0 && options.CLI == "" {
		fmt.Fprintf(w, "The %s command was not found, so register the MCP server yourself:\n", p.target.command)
		for _, args := range p.commands {
			fmt.Fprintf(w, "  %s\n", p.commandLine(args))
		}
		return nil
	}
	for index, args := range p.commands {
		if output, err := options.run(options.CLI, args...); err != nil {
			var rest []string
			for _, remaining := range p.commands[index:] {
				rest = append(rest, "  "+p.commandLine(remaining))
			}
			return fmt.Errorf("%s failed: %v\n%s\nRun the rest yourself:\n%s", p.commandLine(args), err, strings.TrimSpace(string(output)), strings.Join(rest, "\n"))
		}
		fmt.Fprintf(w, "Ran %s\n", p.commandLine(args))
	}
	return nil
}

// latestBackupDir is the newest backup in backups holding name (or its
// absent marker).
func latestBackupDir(backups, name string) string {
	entries, err := os.ReadDir(backups)
	if err != nil {
		return ""
	}
	for index := len(entries) - 1; index >= 0; index-- {
		dir := filepath.Join(backups, entries[index].Name())
		for _, candidate := range []string{name, name + ".absent"} {
			if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
				return dir
			}
		}
	}
	return ""
}

// latestBackup is the newest backed-up copy of name, or "".
func latestBackup(backups, name string) string {
	if dir := latestBackupDir(backups, name); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
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
