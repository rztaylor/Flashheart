package cli

import (
	"strings"

	"github.com/rztaylor/flashheart/internal/hooks"
	"github.com/rztaylor/flashheart/internal/hooks/claude"
	"github.com/rztaylor/flashheart/internal/hooks/codex"
	"github.com/rztaylor/flashheart/internal/setup"
)

// agent is a coding agent Flashheart works with: how its hook payloads are
// read (HOOK-7) and how it is configured and checked (SET-1–SET-4). An
// agent without setup has hooks only: setup and doctor leave it out.
type agent struct {
	hooks hooks.Adapter
	setup setup.Agent
}

// agents is the one registry hook, setup, doctor and their usage errors
// read, in doctor's section order.
var agents = []agent{
	{hooks: claude.Adapter{}, setup: setup.Claude{}},
	// Codex's setup and doctor checks are FH-75.
	{hooks: codex.Adapter{}},
}

// name is the agent's name on the command line and in run ids.
func (a agent) name() string { return a.hooks.Agent() }

// findAgent returns the registered agent with this command-line name.
func findAgent(name string) (agent, bool) {
	for _, a := range agents {
		if a.name() == name {
			return a, true
		}
	}
	return agent{}, false
}

// knownAgents lists the registered agents for usage errors: every agent,
// or with configurable only those setup configures.
func knownAgents(configurable bool) string {
	var names []string
	for _, a := range agents {
		if !configurable || a.setup != nil {
			names = append(names, a.name())
		}
	}
	return strings.Join(names, ", ")
}
