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
var codexRemedy = "run flashheart setup " + Codex{}.Name()

// Diagnose checks Codex's configuration for Flashheart without changing
// anything (SET-4): the MCP server, the hooks of §5.3, the skill and the
// section of the global AGENTS.md, each against what Install would write
// for o. Without a Codex
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
	return append(findings, o.diagnoseSkill(o.codexSkillDir(), codexRemedy), o.diagnoseSection(o.codexAgentsPath(), Codex{}.Name(), codexRemedy))
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
// Codex runs them: hooks can be turned off, and Codex skips a hook unless
// the trust saved for it holds the hash of the hook as it is now
// (agent-protocol §5.4, codex_trust.go). It names the events whose
// Flashheart hook was never trusted or was trusted with another command.
func (o Options) diagnoseCodexHooks(config []tomlEntry, configErr error) []health.Finding {
	_, _, tree, err := readHooksFile(o.codexHooksPath())
	if err != nil {
		return []health.Finding{{Level: health.Problem, Text: fmt.Sprintf("Hooks: cannot read them: %v", err)}}
	}
	findings, installed := o.diagnoseHookEvents(tree.get("hooks"), codexEvents, o.codexHookCommand, codexRemedy)
	if !installed || configErr != nil {
		return findings
	}
	off := false
	trust := map[string]string{}
	for _, entry := range config {
		switch {
		case !entry.header && slices.Equal(entry.path, []string{"features", "hooks"}):
			off = entry.value == "false"
		case !entry.header && len(entry.path) == 4 && entry.path[0] == "hooks" && entry.path[1] == "state" && entry.path[3] == "trusted_hash":
			trust[entry.path[2]], _ = parseTOMLString(entry.value)
		}
	}
	if off {
		return append(findings, health.Finding{Level: health.Warning, Text: fmt.Sprintf("Hooks: turned off in %s ([features] hooks = false), so Codex runs none; remove that line", o.display(o.codexConfigPath()))})
	}
	trusted, untrusted, changed := o.codexHookTrust(tree.get("hooks"), trust)
	switch {
	case len(untrusted) > 0 && len(trusted)+len(changed) == 0:
		findings = append(findings, health.Finding{Level: health.Warning, Text: "Hooks: not trusted yet, so Codex skips them; start codex, type /hooks and trust each Flashheart hook"})
	case len(untrusted) > 0:
		findings = append(findings, health.Finding{Level: health.Warning, Text: fmt.Sprintf("Hooks: not trusted for %s, so Codex skips them; start codex, type /hooks and trust them", strings.Join(untrusted, ", "))})
	}
	if len(changed) > 0 {
		findings = append(findings, health.Finding{Level: health.Warning, Text: fmt.Sprintf("Hooks: changed since you trusted them for %s, so Codex skips them; start codex, type /hooks and trust them again", strings.Join(changed, ", "))})
	}
	return findings
}

// codexHookTrust sorts the events of Flashheart's hooks, in codexEvents
// order, by the trust saved for them: trusted for the hook as it is now,
// not trusted at all, or trusted with another hash (changed). Codex keys
// trust by $CODEX_HOME's real path when CODEX_HOME is set and by ~/.codex
// as it is otherwise, so either key counts, the one Codex uses first. A
// hook Codex would not load is left out.
func (o Options) codexHookTrust(hooks *value, saved map[string]string) (trusted, untrusted, changed []string) {
	files := []string{o.codexHooksPath()}
	if real, err := filepath.EvalSymlinks(o.codexHome()); err == nil {
		files = append(files, filepath.Join(real, "hooks.json"))
		if o.CodexHome != "" {
			slices.Reverse(files)
		}
	}
	add := func(list *[]string, event string) {
		if !slices.Contains(*list, event) {
			*list = append(*list, event)
		}
	}
	for _, event := range codexEvents {
		for group, groupValue := range hooks.get(event).arrayItems() {
			for handler, entry := range groupValue.get("hooks").arrayItems() {
				if !ownsCommand(entry.get("command").text()) {
					continue
				}
				hash := codexHookHash(event, groupValue, entry)
				trust, found := "", false
				for _, file := range files {
					if trust, found = saved[codexHookKey(file, event, group, handler)]; found {
						break
					}
				}
				switch {
				case hash == "":
				case !found:
					add(&untrusted, event)
				case trust != hash:
					add(&changed, event)
				default:
					add(&trusted, event)
				}
			}
		}
	}
	return trusted, untrusted, changed
}
