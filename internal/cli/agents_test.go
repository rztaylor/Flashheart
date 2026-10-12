package cli

import (
	"strings"
	"testing"
)

// hook, setup and doctor know the agents of one registry (FH-89); an agent
// with hooks only would be known to hook alone.
func TestTheAgentRegistryIsTheKnownAgentList(t *testing.T) {
	t.Parallel()

	hookKnown := "(known: " + knownAgents(false) + ")"
	setupKnown := "(known: " + knownAgents(true) + ")"
	for _, registered := range agents {
		if !strings.Contains(hookKnown, registered.name()) {
			t.Errorf("%s missing from %s", registered.name(), hookKnown)
		}
		if registered.setup == nil {
			if strings.Contains(setupKnown, registered.name()) {
				t.Errorf("%s has no setup but is in %s", registered.name(), setupKnown)
			}
			continue
		}
		if registered.hooks.Agent() != registered.setup.Name() {
			t.Errorf("hook adapter %q is registered as setup agent %q", registered.hooks.Agent(), registered.setup.Name())
		}
		if !strings.Contains(setupKnown, registered.name()) {
			t.Errorf("%s missing from %s", registered.name(), setupKnown)
		}
	}
	if _, found := findAgent("codex"); !found || !strings.Contains(hookKnown, "codex") {
		t.Errorf("codex hooks are not registered: %s", hookKnown)
	}

	h := newHarness(t)
	root := t.TempDir()
	if _, _, stderr := h.run("hook", "gemini", "SessionStart", "--root", root); !strings.Contains(stderr, hookKnown) {
		t.Errorf("hook: stderr %q lacks %s", stderr, hookKnown)
	}
	for _, args := range [][]string{{"setup"}, {"setup", "gemini"}} {
		if _, _, stderr := h.run(args...); !strings.Contains(stderr, setupKnown) {
			t.Errorf("%v: stderr %q lacks %s", args, stderr, setupKnown)
		}
	}
	_, stdout, _ := h.run("doctor", "--root", root)
	for _, registered := range agents {
		if registered.setup != nil && !strings.Contains(stdout, "\n"+registered.setup.Title()+"\n") {
			t.Errorf("doctor has no %s section:\n%s", registered.setup.Title(), stdout)
		}
	}
}
