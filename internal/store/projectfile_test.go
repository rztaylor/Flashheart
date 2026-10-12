package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/mdfile"
)

// One reader: a field with a problem warns and is left out; the others are
// kept (FH-77).
func TestParseProjectFileWarnsPerField(t *testing.T) {
	t.Parallel()

	f := parseProjectFile("app", []byte("\ufeffname: My App\r\nkey: bad\r\nnext_id: x\r\nrepos: /src/app\r\nretired: [AP-1]\r\nsettings:\r\n  enforce_handoff: maybe\r\n"))
	if f.Name != "My App" || f.Key != "" || f.NextID != 0 || !slices.Equal(f.Repos, []string{"/src/app"}) ||
		!slices.Equal(f.Retired, []string{"AP-1"}) || f.Settings.EnforceHandoff != nil {
		t.Errorf("fields = %+v", f)
	}
	if len(f.Warnings) != 3 || !strings.HasPrefix(f.Warnings[0], "project.yaml next_id: line 3: ") ||
		!strings.HasPrefix(f.Warnings[1], "project.yaml settings: line 7: ") || !strings.HasPrefix(f.Warnings[2], `key "bad" in project.yaml`) {
		t.Errorf("warnings = %q", f.Warnings)
	}
	if !f.readable("repos") || f.readable("settings") {
		t.Errorf("readable: repos %v, settings %v", f.readable("repos"), f.readable("settings"))
	}

	broken := parseProjectFile("app", []byte("name: [oops\n"))
	if len(broken.Warnings) != 1 || !strings.HasPrefix(broken.Warnings[0], "project.yaml does not parse: line 1: ") || broken.readable("repos") {
		t.Errorf("broken = %+v", broken)
	}
}

func TestProjectFileListsThatCannotBeReadAreNeverReplaced(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	name := filepath.Join(root, "app", "project.yaml")
	write(t, name, "name: App\nrepos:\n  first: /src/app\n")
	if _, err := s.ProjectFor("app", "/src/app", true); !errors.Is(err, mdfile.ErrBrokenFrontmatter) {
		t.Errorf("ProjectFor = %v, want ErrBrokenFrontmatter", err)
	}
	if data, _ := os.ReadFile(name); string(data) != "name: App\nrepos:\n  first: /src/app\n" {
		t.Errorf("project.yaml = %q", data)
	}
	project, _ := s.ReadProject("app")
	if len(project.Warnings) == 0 || !strings.HasPrefix(project.Warnings[0], "project.yaml repos: line 3: ") {
		t.Errorf("warnings = %q", project.Warnings)
	}
}

func TestDeleteArchivedKeepsAnEncodedProjectFile(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	name := filepath.Join(root, "alpha", "project.yaml")
	data, _ := os.ReadFile(name)
	encoded := "\ufeff" + strings.ReplaceAll(string(data), "\n", "\r\n")
	write(t, name, encoded)
	plan, err := s.PlanDelete("alpha", "AL-8")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteArchived("alpha", "AL-8", plan.Token); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(name)
	if want := encoded + "retired: [AL-8]\r\n"; string(after) != want {
		t.Errorf("project.yaml = %q, want %q", after, want)
	}
	project, _ := s.ReadProject("alpha")
	if !slices.Equal(project.Retired, []string{"AL-8"}) || project.Key != "AL" || len(project.Warnings) != 0 {
		t.Errorf("project = %v %q %q", project.Retired, project.Key, project.Warnings)
	}
	if key := s.ProjectFileKey("alpha"); key != "AL" {
		t.Errorf("ProjectFileKey = %q", key)
	}
}
