package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

func TestArchiveAndRestoreProject(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.ArchiveProject("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("alpha is still live: %v", err)
	}
	if names, _ := s.Projects(); slices.Contains(names, "alpha") {
		t.Errorf("projects = %v", names)
	}
	archived, err := s.ArchivedProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 || archived[0].Name != "alpha" || archived[0].Key != "AL" || archived[0].DisplayName != "Alpha" ||
		len(archived[0].IDs) != 8 || !slices.Contains(archived[0].Repos, "/Users/example/src/alpha") || archived[0].Archived.IsZero() {
		t.Fatalf("archived = %+v", archived)
	}

	// Its tickets count as done for other projects: BE-1 waited on AL-3.
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.ArchivedProjects) != 1 || !slices.Contains(b.ArchivedProjects[0].IDs, "AL-3") {
		t.Errorf("board archived projects = %+v", b.ArchivedProjects)
	}
	if reasons := board.Analyze(b).Blocked[board.Ref{Project: "beta", ID: "BE-1"}]; len(reasons) != 0 {
		t.Errorf("BE-1 blocked by %v", reasons)
	}

	// An archived project of the same name blocks a second archive.
	write(t, filepath.Join(root, "alpha", "tickets", ".keep"), "")
	if err := s.ArchiveProject("alpha"); !errors.Is(err, ErrExists) {
		t.Errorf("second archive: %v", err)
	}
	// A live project of the same name blocks a restore.
	if err := s.RestoreProject("alpha"); !errors.Is(err, ErrExists) {
		t.Errorf("restore over a live project: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "alpha")); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreProject("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "project.yaml")); err != nil {
		t.Errorf("restored project: %v", err)
	}
	if err := s.RestoreProject("alpha"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second restore: %v", err)
	}
	if err := s.ArchiveProject("nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("archive unknown: %v", err)
	}
	if err := s.ArchiveProject("../x"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("archive a path: %v", err)
	}
}

func TestArchivedAndDeletedProjectKeysStayTaken(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.ArchiveProject("alpha"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "alpine", "project.yaml"), "name: Alpine\n")
	var taken *KeyTakenError
	if err := s.SetProjectKey("alpine", "AL"); !errors.As(err, &taken) {
		t.Errorf("key of an archived project: %v", err)
	}
	plan, err := s.PlanProjectDelete("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteArchivedProject("alpha", plan.Token); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProjectKey("alpine", "AL"); !errors.As(err, &taken) {
		t.Errorf("key of a deleted project: %v", err)
	}
	// A project derived from the same name gets a fresh key.
	created, err := s.CreateTicket("alpine", NewTicket{Title: "First", Type: "feature", Priority: "low"})
	if err != nil || strings.HasPrefix(created.ID, "AL-") {
		t.Errorf("created %q, %v", created.ID, err)
	}
}

func TestDeleteArchivedProject(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if _, err := s.PlanProjectDelete("alpha"); !errors.Is(err, ErrNotFound) {
		t.Errorf("plan for a live project: %v", err)
	}
	if err := s.DeleteArchivedProject("alpha", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete a live project: %v", err)
	}
	if err := s.ArchiveProject("alpha"); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PlanProjectDelete("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Key != "AL" || plan.Tickets != 8 || len(plan.References) != 1 ||
		plan.References[0].Project != "beta" || plan.References[0].ID != "BE-1" || !slices.Equal(plan.References[0].DependsOn, []string{"AL-3"}) {
		t.Fatalf("plan = %+v", plan)
	}

	// A referencing ticket changed since the plan: nothing happens.
	if _, err := s.UpdateTicket("beta", "BE-1", "", setStatus("up-next")); err != nil {
		t.Fatal(err)
	}
	var conflict *ConflictError
	if err := s.DeleteArchivedProject("alpha", plan.Token); !errors.As(err, &conflict) {
		t.Fatalf("stale token: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".archive", "alpha")); err != nil {
		t.Fatalf("deleted despite the conflict: %v", err)
	}

	plan, _ = s.PlanProjectDelete("alpha")
	if err := s.DeleteArchivedProject("alpha", plan.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".archive", "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("archived directory still there: %v", err)
	}
	be1, _ := os.ReadFile(filepath.Join(root, "beta", "tickets", "BE-1-hello", "BE-1-hello.md"))
	if list := mdfile.Parse(be1).List("depends-on"); len(list) != 0 {
		t.Errorf("BE-1 depends-on = %v", list)
	}
	retired, _ := os.ReadFile(filepath.Join(root, ".flashheart", "retired.yaml"))
	if !strings.Contains(string(retired), "keys: [AL]") {
		t.Errorf("retired.yaml =\n%s", retired)
	}

	// Links are refused, and names are names.
	if err := os.Symlink(filepath.Join(root, "beta"), filepath.Join(root, ".archive", "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlanProjectDelete("linked"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("plan a symlink: %v", err)
	}
	if err := s.DeleteArchivedProject("linked", "x"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("delete a symlink: %v", err)
	}
	if err := s.RestoreProject("linked"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("restore a symlink: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "beta", "tickets", "BE-1-hello")); err != nil {
		t.Errorf("symlink target damaged: %v", err)
	}
	if err := s.DeleteArchivedProject("../beta", "x"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("delete a path: %v", err)
	}
}

func TestArchivedProjectsRepositoryIsNotRecreated(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.ArchiveProject("alpha"); err != nil {
		t.Fatal(err)
	}
	_, err := s.ProjectFor("alpha", "/Users/example/src/alpha", true)
	if !errors.Is(err, ErrProjectArchived) {
		t.Fatalf("ProjectFor = %v, want ErrProjectArchived", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("project recreated: %v", err)
	}
	// Another repository with the same name still gets its own project.
	if name, err := s.ProjectFor("alpha", "/Users/example/src/other/alpha", true); err != nil || name == "alpha" {
		t.Errorf("other repository: %q, %v", name, err)
	}
}
