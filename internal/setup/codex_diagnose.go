package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/health"
)

// codexRemedy is the fix Codex findings name.
var codexRemedy = "run flashheart setup " + Codex{}.Name() + " --write"

// Diagnose checks Codex's configuration for Flashheart without changing
// anything (SET-4): the MCP server, the hooks of §5.3 and the protocol
// text, each against what Install would write for o. Without a Codex
// home there is nothing to check.
func (Codex) Diagnose(o Options) []health.Finding {
	if _, err := os.Stat(o.codexHome()); errors.Is(err, fs.ErrNotExist) {
		return []health.Finding{{Level: health.OK, Text: fmt.Sprintf("Codex is not installed here (no %s); %s if you install it", o.display(o.codexHome()), codexRemedy)}}
	}
	config, configErr := readFile(o.codexConfigPath())
	var entries []tomlEntry
	if configErr == nil {
		entries, configErr = scanTOML(config)
	}
	findings := o.diagnoseCodexServer(config, configErr)
	findings = append(findings, o.diagnoseCodexHooks(entries, configErr)...)
	return append(findings, o.diagnoseCodexAgents()...)
}

func (o Options) diagnoseCodexServer(config []byte, configErr error) []health.Finding {
	where := o.display(o.codexConfigPath())
	problem := func(text string) []health.Finding {
		return []health.Finding{{Level: health.Problem, Text: "MCP server: " + text}}
	}
	if configErr != nil {
		return problem(fmt.Sprintf("cannot read %s: %v; fix it, then %s", where, configErr, codexRemedy))
	}
	server, err := findServer(config)
	switch {
	case err != nil:
		return problem(fmt.Sprintf("cannot read it in %s: %v", where, err))
	case server.header == nil:
		return problem(fmt.Sprintf("not in %s; %s", where, codexRemedy))
	}
	command := o.configuredCommand(server.command)
	args, ok := configuredArgs(server.args)
	switch {
	case command == "":
		return problem(fmt.Sprintf("its command in %s is missing or not a plain string; %s", where, codexRemedy))
	case command != o.Binary:
		return []health.Finding{o.diagnoseProgram("MCP server: runs", command, codexRemedy)}
	case !ok || !slices.Equal(args, o.serverArgs()):
		return []health.Finding{{Level: health.Warning, Text: fmt.Sprintf("MCP server: runs with %q, not %q; %s", strings.Join(args, " "), strings.Join(o.serverArgs(), " "), codexRemedy)}}
	}
	return []health.Finding{{Level: health.OK, Text: "MCP server: configured in " + where}}
}

// diagnoseCodexHooks checks hooks.json and, from config.toml, whether
// Codex runs them: hooks can be turned off, and Codex skips hooks nobody
// trusted. Trust is recorded per hook as [hooks.state."<file>:<event>:
// <group>:<handler>"] with a hash of the hook; how Codex computes the
// hash is not known, so only whether any trust exists for hooks.json is
// checked (agent-protocol §5.3).
func (o Options) diagnoseCodexHooks(config []tomlEntry, configErr error) []health.Finding {
	_, _, tree, err := readHooksFile(o.codexHooksPath())
	if err != nil {
		return []health.Finding{{Level: health.Problem, Text: fmt.Sprintf("Hooks: cannot read them: %v", err)}}
	}
	findings, installed := o.diagnoseHookEvents(tree.get("hooks"), codexEvents, o.codexHookCommand, codexRemedy)
	if !installed || configErr != nil {
		return findings
	}
	hooksFile := o.codexHooksPath()
	trusted := []string{hooksFile + ":"}
	if real, err := filepath.EvalSymlinks(o.codexHome()); err == nil {
		trusted = append(trusted, filepath.Join(real, "hooks.json")+":")
	}
	off, trust := false, false
	for _, entry := range config {
		switch {
		case !entry.header && slices.Equal(entry.path, []string{"features", "hooks"}):
			off = entry.value == "false"
		case len(entry.path) >= 3 && entry.path[0] == "hooks" && entry.path[1] == "state":
			for _, prefix := range trusted {
				trust = trust || strings.HasPrefix(entry.path[2], prefix)
			}
		}
	}
	switch {
	case off:
		findings = append(findings, health.Finding{Level: health.Warning, Text: fmt.Sprintf("Hooks: turned off in %s ([features] hooks = false), so Codex runs none; remove that line", o.display(o.codexConfigPath()))})
	case !trust:
		findings = append(findings, health.Finding{Level: health.Warning, Text: "Hooks: not trusted yet, so Codex skips them; start codex, type /hooks and trust each Flashheart hook"})
	}
	return findings
}

func (o Options) diagnoseCodexAgents() []health.Finding {
	where := o.display(o.codexAgentsPath())
	data, err := readFile(o.codexAgentsPath())
	var start, end int
	found := false
	if err == nil {
		start, end, found, err = findSection(data)
	}
	switch {
	case err != nil:
		return []health.Finding{{Level: health.Problem, Text: fmt.Sprintf("Protocol text: cannot read %s: %v; fix it, then %s", where, err, codexRemedy)}}
	case !found:
		return []health.Finding{{Level: health.Problem, Text: fmt.Sprintf("Protocol text: not in %s; %s", where, codexRemedy)}}
	case strings.TrimSuffix(string(data[start:end]), "\n") != strings.TrimSuffix(agentsSection(), "\n"):
		return []health.Finding{{Level: health.Warning, Text: fmt.Sprintf("Protocol text: out of date in %s; %s", where, codexRemedy)}}
	}
	return []health.Finding{{Level: health.OK, Text: "Protocol text: up to date in " + where}}
}
