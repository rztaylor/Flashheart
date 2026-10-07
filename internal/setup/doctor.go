package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/protocol"
)

// Level is how much a doctor finding matters (SET-4).
type Level int

const (
	// OK is a check that passed.
	OK Level = iota
	// Warning works but is not what setup would write.
	Warning
	// Problem stops Flashheart from working for this agent.
	Problem
)

// Finding is one result of Diagnose, its text starting with what it checked.
type Finding struct {
	Level Level
	Text  string
}

const fix = "run flashheart setup claude --write"

// Diagnose checks Claude Code's configuration for Flashheart without
// changing anything (SET-4): the hooks of §5.2, the MCP server and the
// skill, each against what Install would write for o.
func Diagnose(o Options) []Finding {
	findings := o.diagnoseHooks()
	findings = append(findings, o.diagnoseServer()...)
	return append(findings, o.diagnoseSkill()...)
}

func (o Options) diagnoseHooks() []Finding {
	_, _, tree, err := o.readUserSettings()
	if err != nil {
		return []Finding{{Problem, fmt.Sprintf("Hooks: cannot read the settings: %v", err)}}
	}
	var missing, programs []string
	differ := false
	hooks := tree.get("hooks")
	for _, event := range claudeEvents {
		found := false
		var groups *value
		if hooks != nil {
			groups = hooks.get(event)
		}
		if groups != nil {
			for _, group := range groups.array {
				entries := group.get("hooks")
				if entries == nil {
					continue
				}
				for _, entry := range entries.array {
					command := entry.get("command").text()
					if !ownsCommand(command) {
						continue
					}
					found = true
					program, _, _ := splitProgram(command)
					program = o.resolve(program)
					if program != o.Binary && !slices.Contains(programs, program) {
						programs = append(programs, program)
					}
					differ = differ || program == o.Binary && strings.TrimSpace(command) != o.hookCommand(event)
				}
			}
		}
		if !found {
			missing = append(missing, event)
		}
	}
	var findings []Finding
	switch {
	case len(missing) == len(claudeEvents):
		return []Finding{{Problem, "Hooks: none installed; " + fix}}
	case len(missing) > 0:
		findings = append(findings, Finding{Problem, fmt.Sprintf("Hooks: missing for %s; %s", strings.Join(missing, ", "), fix)})
	}
	for _, program := range programs {
		findings = append(findings, o.diagnoseProgram("Hooks: run", program))
	}
	if differ {
		findings = append(findings, Finding{Warning, "Hooks: differ from what setup writes (another --root?); " + fix})
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{OK, fmt.Sprintf("Hooks: all %d installed", len(claudeEvents))})
	}
	return findings
}

// resolve finds the file a configured program names, as the shell would:
// ~ is the home directory, a bare name is looked up on PATH.
func (o Options) resolve(program string) string {
	if rest, ok := strings.CutPrefix(program, "~/"); ok {
		return filepath.Join(o.Home, rest)
	}
	if !strings.ContainsRune(program, filepath.Separator) {
		if found, err := exec.LookPath(program); err == nil {
			return found
		}
	}
	return program
}

// diagnoseProgram reports a configured flashheart program other than this
// binary: a problem when it is gone, else a warning.
func (o Options) diagnoseProgram(what, program string) Finding {
	if info, err := os.Stat(program); err != nil || info.IsDir() {
		return Finding{Problem, fmt.Sprintf("%s %s, which does not exist; %s", what, program, fix)}
	}
	return Finding{Warning, fmt.Sprintf("%s %s, not this binary (%s); %s", what, program, o.Binary, fix)}
}

func (o Options) diagnoseServer() []Finding {
	command, args, found, problem := o.registered()
	command = o.resolve(command)
	switch {
	case problem != "":
		return []Finding{{Warning, "MCP server: unknown. " + problem}}
	case !found:
		return []Finding{{Problem, "MCP server: not registered; " + fix}}
	case command != o.Binary:
		return []Finding{o.diagnoseProgram("MCP server: runs", command)}
	case !slices.Equal(args, o.serverArgs()):
		return []Finding{{Warning, fmt.Sprintf("MCP server: runs with %q, not %q; %s", strings.Join(args, " "), strings.Join(o.serverArgs(), " "), fix)}}
	}
	return []Finding{{OK, "MCP server: registered"}}
}

func (o Options) diagnoseSkill() []Finding {
	var findings []Finding
	skill, err := readFile(o.skillPath())
	switch {
	case err != nil:
		findings = append(findings, Finding{Problem, fmt.Sprintf("Skill: cannot read it: %v", err)})
	case skill == nil:
		findings = append(findings, Finding{Problem, "Skill: not installed; " + fix})
	case !bytes.Equal(skill, []byte(protocol.Skill())):
		findings = append(findings, Finding{Warning, "Skill: out of date; " + fix})
	}
	if info, err := os.Stat(o.replacedPath()); err == nil && info.IsDir() {
		findings = append(findings, Finding{Warning, "Skill: kanban-tracker is still installed beside it; " + fix + " to move it aside"})
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{OK, "Skill: up to date"})
	}
	return findings
}
