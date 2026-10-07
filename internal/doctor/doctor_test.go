package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/setup"
)

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

// fixture is a home with no Claude configuration and a board root.
func fixture(t *testing.T) Options {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "reports", "Kanban")
	if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Options{
		Root:   root,
		Claude: setup.Options{Home: home, Binary: filepath.Join(home, "bin", "flashheart")},
		Now:    func() time.Time { return now },
	}
}

func report(sections []Section) (string, int) {
	var out bytes.Buffer
	problems := Render(&out, sections)
	return out.String(), problems
}

func TestDoctorChecksTheRoot(t *testing.T) {
	t.Parallel()

	o := fixture(t)
	out, _ := report(Check(o))
	if !strings.Contains(out, "Board root "+o.Root+"\n  ok       exists and is writable\n") {
		t.Fatalf("report:\n%s", out)
	}

	missing := o
	missing.Root = filepath.Join(o.Root, "nope")
	out, problems := report(Check(missing))
	if problems == 0 || !strings.Contains(out, "  problem  does not exist") {
		t.Fatalf("missing root (%d problems):\n%s", problems, out)
	}

	if err := os.WriteFile(filepath.Join(o.Root, ".flashheart", "config.yaml"), []byte("quiet_after: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _ = report(Check(o)); !strings.Contains(out, "  problem  settings: ") {
		t.Fatalf("bad config:\n%s", out)
	}
}

func TestDoctorWarnsAboutARootOthersCanWrite(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are POSIX")
	}

	o := fixture(t)
	if err := os.Chmod(o.Root, 0o777); err != nil {
		t.Fatal(err)
	}
	if out, _ := report(Check(o)); !strings.Contains(out, "  warning  other users can write to it (mode 0777); chmod go-w "+o.Root) {
		t.Fatalf("report:\n%s", out)
	}
}

func TestDoctorReportsRecentHookErrors(t *testing.T) {
	t.Parallel()

	o := fixture(t)
	out, _ := report(Check(o))
	if !strings.Contains(out, "Hook errors\n  ok       none in the last 24 hours\n") {
		t.Fatalf("report:\n%s", out)
	}
	log := "2026-10-05T09:00:00Z claude Stop: too old to matter\n" +
		"2026-10-07T09:00:00Z claude PostToolUse: payload is larger than 1048576 bytes\n" +
		"2026-10-07T11:30:00Z claude Stop: read payload: unexpected EOF\n"
	if err := os.WriteFile(filepath.Join(o.Root, ".flashheart", "hook-errors.log"), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	out, problems := report(Check(o))
	for _, want := range []string{
		"  warning  2 in the last 24 hours, in " + filepath.Join(o.Root, ".flashheart", "hook-errors.log") + "\n",
		"           2026-10-07T09:00:00Z claude PostToolUse: payload is larger than 1048576 bytes\n",
		"           2026-10-07T11:30:00Z claude Stop: read payload: unexpected EOF\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "too old") {
		t.Fatalf("old entry shown:\n%s", out)
	}
	// Hooks fail open, so their errors are warnings; the unconfigured Claude
	// Code setup is what makes problems here.
	if problems != 3 {
		t.Fatalf("problems = %d", problems)
	}
}

func TestDoctorIncludesTheClaudeCodeSetup(t *testing.T) {
	t.Parallel()

	out, _ := report(Check(fixture(t)))
	if !strings.Contains(out, "Claude Code\n  problem  Hooks: none installed; run flashheart setup claude --write\n") {
		t.Fatalf("report:\n%s", out)
	}
}
