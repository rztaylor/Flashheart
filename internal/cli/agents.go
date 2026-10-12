package cli

import (
	"strings"

	"github.com/rztaylor/flashheart/internal/hooks"
	"github.com/rztaylor/flashheart/internal/hooks/claude"
	"github.com/rztaylor/flashheart/internal/setup"
)

// agent is a coding agent Flashheart works with: how its hook payloads are
// read (HOOK-7) and how it is configured and checked (SET-1–SET-4).
type agent struct {
	hooks hooks.Adapter
	setup setup.Agent
}

// agents is the one registry hook, setup, doctor and their usage errors
// read, in doctor's section order.
var agents = []agent{
	{hooks: claude.Adapter{}, setup: setup.Claude{}},
}

// findAgent returns the registered agent with this command-line name.
func findAgent(name string) (agent, bool) {
	for _, a := range agents {
		if a.setup.Name() == name {
			return a, true
		}
	}
	return agent{}, false
}

// knownAgents lists the registered agents for usage errors.
func knownAgents() string {
	names := make([]string, len(agents))
	for index, a := range agents {
		names[index] = a.setup.Name()
	}
	return strings.Join(names, ", ")
}
