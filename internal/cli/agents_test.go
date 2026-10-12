package cli

import (
	"strings"
	"testing"
)

// hook, setup and doctor know the agents of one registry (FH-89).
func TestTheAgentRegistryIsTheKnownAgentList(t *testing.T) {
	t.Parallel()

	known := "(known: " + knownAgents() + ")"
	for _, registered := range agents {
		if registered.hooks.Agent() != registered.setup.Name() {
			t.Errorf("hook adapter %q is registered as setup agent %q", registered.hooks.Agent(), registered.setup.Name())
		}
		if !strings.Contains(known, registered.setup.Name()) {
			t.Errorf("%s missing from %s", registered.setup.Name(), known)
		}
	}

	h := newHarness(t)
	root := t.TempDir()
	if _, _, stderr := h.run("hook", "gemini", "SessionStart", "--root", root); !strings.Contains(stderr, known) {
		t.Errorf("hook: stderr %q lacks %s", stderr, known)
	}
	for _, args := range [][]string{{"setup"}, {"setup", "gemini"}} {
		if _, _, stderr := h.run(args...); !strings.Contains(stderr, known) {
			t.Errorf("%v: stderr %q lacks %s", args, stderr, known)
		}
	}
	_, stdout, _ := h.run("doctor", "--root", root)
	for _, registered := range agents {
		if !strings.Contains(stdout, "\n"+registered.setup.Title()+"\n") {
			t.Errorf("doctor has no %s section:\n%s", registered.setup.Title(), stdout)
		}
	}
}
