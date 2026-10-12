package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/health"
	"github.com/rztaylor/flashheart/internal/protocol"
)

// fix is the remedy doctor findings name.
var fix = "run flashheart setup " + Claude{}.Name() + " --write"

// Diagnose checks Claude Code's configuration for Flashheart without
// changing anything (SET-4): the hooks of §5.2, the MCP server and the
// skill, each against what Install would write for o.
func (Claude) Diagnose(o Options) []health.Finding {
	findings := o.diagnoseHooks()
	findings = append(findings, o.diagnoseServer()...)
	return append(findings, o.diagnoseSkill()...)
}

func (o Options) diagnoseHooks() []health.Finding {
	_, _, tree, err := o.readUserSettings()
	if err != nil {
		return []health.Finding{{Level: health.Problem, Text: fmt.Sprintf("Hooks: cannot read the settings: %v", err)}}
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
	var findings []health.Finding
	switch {
	case len(missing) == len(claudeEvents):
		return []health.Finding{{Level: health.Problem, Text: "Hooks: none installed; " + fix}}
	case len(missing) > 0:
		findings = append(findings, health.Finding{Level: health.Problem, Text: fmt.Sprintf("Hooks: missing for %s; %s", strings.Join(missing, ", "), fix)})
	}
	for _, program := range programs {
		findings = append(findings, o.diagnoseProgram("Hooks: run", program))
	}
	if differ {
		findings = append(findings, health.Finding{Level: health.Warning, Text: "Hooks: differ from what setup writes (another --root?); " + fix})
	}
	if len(findings) == 0 {
		findings = append(findings, health.Finding{Level: health.OK, Text: fmt.Sprintf("Hooks: all %d installed", len(claudeEvents))})
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
func (o Options) diagnoseProgram(what, program string) health.Finding {
	if info, err := os.Stat(program); err != nil || info.IsDir() {
		return health.Finding{Level: health.Problem, Text: fmt.Sprintf("%s %s, which does not exist; %s", what, program, fix)}
	}
	return health.Finding{Level: health.Warning, Text: fmt.Sprintf("%s %s, not this binary (%s); %s", what, program, o.Binary, fix)}
}

func (o Options) diagnoseServer() []health.Finding {
	server := o.registered()
	command := o.resolve(server.command)
	switch {
	case server.problem != "":
		return []health.Finding{{Level: health.Warning, Text: "MCP server: unknown. " + server.problem}}
	case server.scope != "":
		// The user's own registration works in its project; setup's
		// user-scope one would serve every project (FH-35).
		return []health.Finding{{Level: health.Warning, Text: fmt.Sprintf("MCP server: registered at %s scope, not the user scope setup writes; %s to register it for every project", server.scope, fix)}}
	case !server.found:
		return []health.Finding{{Level: health.Problem, Text: "MCP server: not registered; " + fix}}
	case command != o.Binary:
		return []health.Finding{o.diagnoseProgram("MCP server: runs", command)}
	case !slices.Equal(server.args, o.serverArgs()):
		return []health.Finding{{Level: health.Warning, Text: fmt.Sprintf("MCP server: runs with %q, not %q; %s", strings.Join(server.args, " "), strings.Join(o.serverArgs(), " "), fix)}}
	}
	return []health.Finding{{Level: health.OK, Text: "MCP server: registered"}}
}

func (o Options) diagnoseSkill() []health.Finding {
	var findings []health.Finding
	skill, err := readFile(o.skillPath())
	switch {
	case err != nil:
		findings = append(findings, health.Finding{Level: health.Problem, Text: fmt.Sprintf("Skill: cannot read it: %v", err)})
	case skill == nil:
		findings = append(findings, health.Finding{Level: health.Problem, Text: "Skill: not installed; " + fix})
	case !bytes.Equal(skill, []byte(protocol.Skill())):
		findings = append(findings, health.Finding{Level: health.Warning, Text: "Skill: out of date; " + fix})
	}
	if info, err := os.Stat(o.replacedPath()); err == nil && info.IsDir() {
		findings = append(findings, health.Finding{Level: health.Warning, Text: "Skill: kanban-tracker is still installed beside it; " + fix + " to move it aside"})
	}
	if len(findings) == 0 {
		findings = append(findings, health.Finding{Level: health.OK, Text: "Skill: up to date"})
	}
	return findings
}
