package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/health"
	"github.com/rztaylor/flashheart/internal/logfile"
)

// RecentErrors is how far back hook errors are reported.
const RecentErrors = 24 * time.Hour

// maxShownErrors bounds the hook errors quoted in the report.
const maxShownErrors = 5

// Options name what to check.
type Options struct {
	Root string
	// Agents are checked in order, a section each.
	Agents []Agent
	Now    func() time.Time
}

// Agent is one agent's configuration to check: the section's title and
// the agent's own checks (health.Agent.Diagnose).
type Agent struct {
	Title    string
	Diagnose func() []health.Finding
}

// Section is one area of the report with its findings.
type Section struct {
	Title    string
	Findings []health.Finding
	// Details are lines quoted under the last finding.
	Details []string
}

// Check runs every check, in report order.
func Check(o Options) []Section {
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	sections := []Section{checkRoot(o.Root)}
	for _, agent := range o.Agents {
		sections = append(sections, Section{Title: agent.Title, Findings: agent.Diagnose()})
	}
	return append(sections, checkHookErrors(o.Root, now()))
}

func checkRoot(root string) Section {
	section := Section{Title: "Board root " + root}
	add := func(level health.Level, text string) {
		section.Findings = append(section.Findings, health.Finding{Level: level, Text: text})
	}
	info, err := os.Stat(root)
	switch {
	case os.IsNotExist(err):
		add(health.Problem, "does not exist; create it, or pass --root with the board you use")
		return section
	case err != nil:
		add(health.Problem, fmt.Sprintf("cannot be read: %v", err))
		return section
	case !info.IsDir():
		add(health.Problem, "is not a folder")
		return section
	}
	if err := writable(root); err != nil {
		add(health.Problem, fmt.Sprintf("is not writable: %v", err))
	} else {
		add(health.OK, "exists and is writable")
	}
	// Agents read ticket text from the root, so other users must not
	// write it (POSIX permission bits only).
	if mode := info.Mode().Perm(); runtime.GOOS != "windows" && mode&0o022 != 0 {
		add(health.Warning, fmt.Sprintf("other users can write to it (mode %04o); chmod go-w %s", mode, root))
	}
	if _, err := config.Load(root); err != nil {
		add(health.Problem, fmt.Sprintf("settings: %v", err))
	}
	return section
}

func checkHookErrors(root string, now time.Time) Section {
	section := Section{Title: "Hook errors"}
	path := filepath.Join(root, ".flashheart", "hook-errors.log")
	entries, err := logfile.Recent(path, now.Add(-RecentErrors))
	switch {
	case err != nil:
		section.Findings = []health.Finding{{Level: health.Warning, Text: fmt.Sprintf("cannot read %s: %v", path, err)}}
	case len(entries) == 0:
		section.Findings = []health.Finding{{Level: health.OK, Text: "none in the last 24 hours"}}
	default:
		section.Findings = []health.Finding{{Level: health.Warning, Text: fmt.Sprintf("%d in the last 24 hours, in %s", len(entries), path)}}
		for _, entry := range entries[max(0, len(entries)-maxShownErrors):] {
			text, _, _ := strings.Cut(entry.Text, "\n")
			section.Details = append(section.Details, entry.Time.UTC().Format(time.RFC3339)+" "+text)
		}
	}
	return section
}

var labels = map[health.Level]string{health.OK: "ok", health.Warning: "warning", health.Problem: "problem"}

// Render writes the report and returns the number of problems.
func Render(w io.Writer, sections []Section) int {
	problems := 0
	for index, section := range sections {
		if index > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, section.Title)
		for _, finding := range section.Findings {
			if finding.Level == health.Problem {
				problems++
			}
			fmt.Fprintf(w, "  %-8s %s\n", labels[finding.Level], finding.Text)
		}
		for _, line := range section.Details {
			fmt.Fprintf(w, "  %-8s %s\n", "", line)
		}
	}
	return problems
}
