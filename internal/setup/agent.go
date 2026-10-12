package setup

import (
	"os/exec"
	"time"

	"github.com/rztaylor/flashheart/internal/health"
)

// Agent is a coding agent setup configures for Flashheart. cli lists the
// agents in one registry that hook, setup and doctor read.
type Agent interface {
	// Name is the agent's name on the command line and in run ids.
	Name() string
	// Title is its name in output and doctor's section heading.
	Title() string
	// FindCLI returns the agent's own command-line program, or "" when
	// there is none; a plan's commands run through it.
	FindCLI(home string) string
	// Install plans the agent's configuration for Flashheart.
	Install(Options) (*Plan, error)
	// Uninstall plans its reverse.
	Uninstall(Options) (*Plan, error)
	// Diagnose checks the configuration without changing it (SET-4).
	Diagnose(Options) []health.Finding
}

// SkillRefresher is an Agent with an installed protocol skill that the
// session-start hook keeps current (agent-protocol §5.4).
type SkillRefresher interface {
	RefreshSkill(home string) (bool, error)
}

// Options describe one machine for an agent's configuration.
type Options struct {
	// Home is the user's home directory.
	Home string
	// CodexHome is $CODEX_HOME, Codex's own folder; empty means ~/.codex.
	CodexHome string
	// Binary is the absolute path of the flashheart binary (SET-3).
	Binary string
	// Root is passed to every command when it is not the default root.
	Root string
	Now  func() time.Time
	// CLI is the agent's command-line program (Agent.FindCLI); empty
	// means none was found and its commands are left for the user to run.
	CLI string
	// Run runs a command; nil runs it with os/exec.
	Run func(name string, args ...string) ([]byte, error)
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// run runs a command through Options.Run, or os/exec.
func (o Options) run(name string, args ...string) ([]byte, error) {
	if o.Run != nil {
		return o.Run(name, args...)
	}
	return exec.Command(name, args...).CombinedOutput()
}
