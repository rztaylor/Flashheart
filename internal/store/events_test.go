package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestEventFilesAppendListReadAndRemove(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.AppendEventLines("alpha", "2026-10-05.jsonl", []byte("{\"a\":1}\n")); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEventLines("alpha", "2026-10-05.jsonl", []byte("{\"a\":2}\n")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "alpha", ".flashheart", "events", "2026-10-05.jsonl"))
	if string(data) != "{\"a\":1}\n{\"a\":2}\n" {
		t.Fatalf("event file = %q", data)
	}

	files, err := s.EventFiles("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, []string{"2026-10-04.jsonl", "2026-10-05.jsonl"}) {
		t.Fatalf("EventFiles = %v", files)
	}

	file, err := s.OpenEventFile("alpha", "2026-10-05.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	read, _ := io.ReadAll(file)
	file.Close()
	if string(read) != string(data) {
		t.Fatalf("OpenEventFile read %q", read)
	}

	if err := s.RemoveEventFile("alpha", "2026-10-04.jsonl"); err != nil {
		t.Fatal(err)
	}
	if files, _ := s.EventFiles("alpha"); !slices.Equal(files, []string{"2026-10-05.jsonl"}) {
		t.Fatalf("after remove EventFiles = %v", files)
	}
	// A project without an event log lists nothing.
	if files, err := s.EventFiles("beta"); err != nil || len(files) != 0 {
		t.Fatalf("beta EventFiles = %v, %v", files, err)
	}
}

func TestEventFilesRefuseOtherNames(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	for _, name := range []string{"../x.jsonl", "2026-10-05.json", "notes.jsonl", "2026-10-05.jsonl/../../x", ""} {
		if err := s.AppendEventLines("alpha", name, []byte("{}\n")); !errors.Is(err, ErrInvalidName) {
			t.Errorf("AppendEventLines(%q) = %v, want ErrInvalidName", name, err)
		}
		if err := s.RemoveEventFile("alpha", name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("RemoveEventFile(%q) = %v, want ErrInvalidName", name, err)
		}
	}
	if err := s.AppendEventLines("../alpha", "2026-10-05.jsonl", []byte("{}\n")); !errors.Is(err, ErrInvalidName) {
		t.Errorf("unsafe project = %v", err)
	}
	if err := s.AppendEventLines("missing", "2026-10-05.jsonl", []byte("{}\n")); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing project = %v", err)
	}
}

func TestProjectForCreatesAdoptsAndSeparatesRepositories(t *testing.T) {
	t.Parallel()

	s, root := writable(t)

	// A new repository gets a project with its name and repository, no key.
	name, err := s.ProjectFor("gamma", "/src/gamma", true)
	if err != nil || name != "gamma" {
		t.Fatalf("ProjectFor(gamma) = %q, %v", name, err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "gamma", "project.yaml"))
	if string(data) != "name: gamma\nrepos:\n  - /src/gamma\n" {
		t.Fatalf("project.yaml = %q", data)
	}
	if again, err := s.ProjectFor("gamma", "/src/gamma", true); err != nil || again != "gamma" {
		t.Fatalf("second ProjectFor = %q, %v", again, err)
	}

	// An existing project with no repositories recorded adopts the first one.
	name, err = s.ProjectFor("beta", "/src/beta", true)
	if err != nil || name != "beta" {
		t.Fatalf("ProjectFor(beta) = %q, %v", name, err)
	}
	if repos, _ := s.projectRepos("beta"); !slices.Equal(repos, []string{"/src/beta"}) {
		t.Fatalf("beta repos = %v", repos)
	}
	if data, _ = os.ReadFile(filepath.Join(root, "beta", "project.yaml")); !strings.HasPrefix(string(data), "key: BE\nnext_id: 2\n") {
		t.Fatalf("beta project.yaml lost its fields: %q", data)
	}

	// A different repository with the same name gets a suffixed project (PRJ-3).
	other, err := s.ProjectFor("gamma", "/elsewhere/gamma", true)
	if err != nil || other != "gamma-"+repoSuffix("/elsewhere/gamma") || len(other) != len("gamma-")+6 {
		t.Fatalf("colliding ProjectFor = %q, %v", other, err)
	}
	if again, _ := s.ProjectFor("gamma", "/elsewhere/gamma", true); again != other {
		t.Fatalf("colliding ProjectFor again = %q", again)
	}

	// Without auto-create, a missing project is reported, not created.
	if _, err := s.ProjectFor("delta", "/src/delta", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ProjectFor without create = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "delta")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("delta was created")
	}

	// Activity outside git goes to _scratch, and names are made safe.
	if name, err := s.ProjectFor("", "", true); err != nil || name != ScratchProject {
		t.Fatalf("scratch ProjectFor = %q, %v", name, err)
	}
	if name, err := s.ProjectFor("My Repo!", "/src/My Repo!", true); err != nil || name != "My-Repo" {
		t.Fatalf("unsafe name ProjectFor = %q, %v", name, err)
	}
	projects, _ := s.Projects()
	for _, want := range []string{"gamma", other, ScratchProject, "My-Repo"} {
		if !slices.Contains(projects, want) {
			t.Errorf("Projects() = %v, missing %s", projects, want)
		}
	}
}

// A v1 project is left for flashheart migrate: a hook never writes into it.
func TestProjectForLeavesV1ProjectsAlone(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := os.MkdirAll(filepath.Join(root, "legacy", "todo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProjectFor("legacy", "/src/legacy", true); !errors.Is(err, ErrNeedsMigration) {
		t.Fatalf("ProjectFor(v1) = %v, want ErrNeedsMigration", err)
	}
	if _, err := os.Stat(filepath.Join(root, "legacy", "project.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("project.yaml written into a v1 project")
	}
}

// Repository names that are not usable as directories get a derived name,
// never the reserved _scratch project.
func TestProjectForUnusableNames(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	if name, err := s.ProjectFor("_hidden", "/src/_hidden", true); err != nil || name != "hidden" {
		t.Fatalf("_hidden = %q, %v", name, err)
	}
	first, err := s.ProjectFor("日本", "/src/日本", true)
	if err != nil || first != "repo-"+repoSuffix("/src/日本") {
		t.Fatalf("non-ASCII = %q, %v", first, err)
	}
	second, err := s.ProjectFor("中文", "/src/中文", true)
	if err != nil || second == first || second == ScratchProject {
		t.Fatalf("second non-ASCII = %q, %v", second, err)
	}
	if repos, _ := s.projectRepos(ScratchProject); len(repos) != 0 {
		t.Fatalf("_scratch adopted a repository: %v", repos)
	}
}

// TestProjectForHelperProcess creates the same project from a child process.
func TestProjectForHelperProcess(t *testing.T) {
	root := os.Getenv("FLASHHEART_PROJECTFOR_ROOT")
	if root == "" {
		t.Skip("helper process only")
	}
	s, err := Open(root)
	if err != nil {
		os.Exit(2)
	}
	name, err := s.ProjectFor("shared", os.Getenv("FLASHHEART_PROJECTFOR_REPO"), true)
	if err != nil {
		os.Exit(3)
	}
	fmt.Print(name)
	os.Exit(0)
}

// Processes racing to create projects for two repositories with the same
// name end with one project each, never both in one (PRJ-3).
func TestProjectForAcrossProcesses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	names := make([]string, 4)
	var wg sync.WaitGroup
	for i := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo := "/src/a/shared"
			if i%2 == 1 {
				repo = "/src/b/shared"
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestProjectForHelperProcess$")
			cmd.Env = append(os.Environ(), "FLASHHEART_PROJECTFOR_ROOT="+root, "FLASHHEART_PROJECTFOR_REPO="+repo)
			out, err := cmd.Output()
			if err != nil {
				t.Errorf("helper %d: %v", i, err)
			}
			names[i] = string(out)
		}()
	}
	wg.Wait()
	if names[0] != names[2] || names[1] != names[3] || names[0] == names[1] {
		t.Fatalf("projects = %v; want one per repository", names)
	}
}
