package store

import (
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
)

// sampleCopy copies testdata/boards/sample into a temporary root.
func sampleCopy(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatalf("copy sample: %v", err)
	}
	return root
}

func open(t *testing.T, root string) *Store {
	t.Helper()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open(%s): %v", root, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMissingRoot(t *testing.T) {
	t.Parallel()

	_, err := Open(filepath.Join(t.TempDir(), "absent"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open(absent) = %v, want ErrNotExist", err)
	}
}

func TestProjectDiscovery(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "_ignored", "todo", "feat--x.md"), "# X\n")
	write(t, filepath.Join(root, ".hidden", "todo", "feat--x.md"), "# X\n")
	write(t, filepath.Join(root, "_scratch", "todo", "feat--s.md"), "# S\n")
	write(t, filepath.Join(root, "only-config", "project.yaml"), "name: Only\n")
	write(t, filepath.Join(root, "notes.md"), "not a project\n")
	if err := os.MkdirAll(filepath.Join(root, "empty-dir", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	write(t, filepath.Join(outside, "todo", "feat--x.md"), "# X\n")
	if err := os.Symlink(outside, filepath.Join(root, "escapes")); err != nil {
		t.Fatal(err)
	}

	names, err := open(t, root).Projects()
	if err != nil {
		t.Fatalf("Projects(): %v", err)
	}
	if want := []string{"_scratch", "alpha", "beta", "only-config"}; !reflect.DeepEqual(names, want) {
		t.Errorf("Projects() = %q, want %q", names, want)
	}
}

func TestReadSampleProject(t *testing.T) {
	t.Parallel()

	store := open(t, sampleCopy(t))
	project, err := store.ReadProject("alpha")
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if project.Name != "alpha" || project.DisplayName != "Alpha" || !reflect.DeepEqual(project.Repos, []string{"/Users/example/src/alpha"}) {
		t.Errorf("project = %s %q %q", project.Name, project.DisplayName, project.Repos)
	}
	var placed []string
	for _, ticket := range project.Tickets {
		placed = append(placed, string(ticket.Column)+"/"+ticket.Slug)
	}
	want := []string{
		"todo/bug--column-overflow", "todo/docs--broken-frontmatter", "todo/feat--drag-and-drop", "todo/spike--offline-mode",
		"in-progress/feat--card-panel", "ready-to-review/feat--board-columns", "done/infra--project-skeleton",
	}
	if !reflect.DeepEqual(placed, want) {
		t.Errorf("tickets =\n  %q\nwant\n  %q", placed, want)
	}
	for _, ticket := range project.Tickets {
		if ticket.Modified.IsZero() {
			t.Errorf("%s has no modification time", ticket.Slug)
		}
		if (ticket.Slug == "docs--broken-frontmatter") != ticket.NeedsRepair() {
			t.Errorf("%s NeedsRepair = %v", ticket.Slug, ticket.NeedsRepair())
		}
	}
	if len(project.Workstreams) != 1 || project.Workstreams[0].Slug != "board-ui" {
		t.Errorf("workstreams = %+v", project.Workstreams)
	}
	if !project.Reviews["feat--board-columns"] || len(project.Reviews) != 1 {
		t.Errorf("reviews = %v", project.Reviews)
	}
	attachments := project.Attachments["feat--board-columns"]
	if len(attachments) != 1 || attachments[0].File != "20261003T1000-board-desktop.png" || attachments[0].Kind != "screenshot" || attachments[0].Caption != "Board at 1440x900" {
		t.Errorf("attachments = %+v", project.Attachments)
	}
	if project.LastModified.IsZero() {
		t.Error("project has no last modification time")
	}

	beta, err := store.ReadProject("beta")
	if err != nil {
		t.Fatal(err)
	}
	if beta.DisplayName != "beta" || len(beta.Tickets) != 1 {
		t.Errorf("beta = %q with %d tickets", beta.DisplayName, len(beta.Tickets))
	}
}

func TestReadProjectRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	store := open(t, sampleCopy(t))
	for _, name := range []string{"", ".", "..", "../alpha", "alpha/todo", ".flashheart", "_ignored", `a\b`} {
		if _, err := store.ReadProject(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("ReadProject(%q) = %v, want ErrInvalidName", name, err)
		}
	}
	if _, err := store.ReadProject("gamma"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ReadProject(gamma) = %v, want ErrNotFound", err)
	}
}

func TestFilesThatEscapeOrOverflowNeedRepair(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	outside := filepath.Join(t.TempDir(), "secret.md")
	write(t, outside, "---\ntype: feature\n---\n# Secret\n")
	if err := os.Symlink(outside, filepath.Join(root, "beta", "todo", "feat--escape.md")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "beta", "todo", "feat--huge.md"), "# Huge\n"+strings.Repeat("x", MaxFileBytes))
	write(t, filepath.Join(root, "beta", "todo", "README.txt"), "ignored\n")
	write(t, filepath.Join(root, "beta", "todo", ".feat--hidden.md"), "# hidden editor file\n")
	if err := os.MkdirAll(filepath.Join(root, "beta", "todo", "subdir.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	project, err := open(t, root).ReadProject("beta")
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]board.Ticket{}
	for _, ticket := range project.Tickets {
		bySlug[ticket.Slug] = ticket
	}
	if len(bySlug) != 3 {
		t.Errorf("tickets = %v, want feat--hello, feat--escape and feat--huge", slices.Collect(maps.Keys(bySlug)))
	}
	escape := bySlug["feat--escape"]
	if !escape.NeedsRepair() || !strings.Contains(escape.Repair[0], "outside the board root") || strings.Contains(escape.Body, "Secret") {
		t.Errorf("escaping symlink = %+v", escape)
	}
	if huge := bySlug["feat--huge"]; !huge.NeedsRepair() || !strings.Contains(huge.Repair[0], "larger than") {
		t.Errorf("huge = %+v", huge.Repair)
	}
}

func TestArchivedTicketsAreListed(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "alpha", ".archive", "done", "feat--old.md"), "# Old\n")
	write(t, filepath.Join(root, "alpha", ".archive", "todo", "feat--dropped.md"), "# Dropped\n")
	project, err := open(t, root).ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"feat--dropped", "feat--old"}; !reflect.DeepEqual(project.Archived, want) {
		t.Errorf("Archived = %q, want %q", project.Archived, want)
	}
}

func TestBrokenProjectFilesBecomeWarnings(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "beta", "project.yaml"), "name: [broken\n")
	write(t, filepath.Join(root, "beta", "attachments", "feat--hello", "index.yaml"), "not: a list\n")
	project, err := open(t, root).ReadProject("beta")
	if err != nil {
		t.Fatal(err)
	}
	if project.DisplayName != "beta" || len(project.Warnings) != 2 {
		t.Errorf("display=%q warnings=%q", project.DisplayName, project.Warnings)
	}
}

func TestReadBoardAndFingerprint(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	store := open(t, root)
	first, fingerprint, err := store.ReadBoard()
	if err != nil {
		t.Fatalf("ReadBoard: %v", err)
	}
	if len(first.Projects) != 2 || first.Projects[0].Name != "alpha" || first.Projects[1].Name != "beta" {
		t.Fatalf("projects = %+v", first.Projects)
	}
	_, again, err := store.ReadBoard()
	if err != nil || again != fingerprint {
		t.Fatalf("unchanged board fingerprint changed: %s -> %s (%v)", fingerprint, again, err)
	}

	path := filepath.Join(root, "beta", "todo", "feat--hello.md")
	data, _ := os.ReadFile(path)
	write(t, path, string(data)+"\nMore.\n")
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	_, changed, err := store.ReadBoard()
	if err != nil || changed == fingerprint {
		t.Errorf("edited board fingerprint did not change (%v)", err)
	}

	write(t, filepath.Join(root, "beta", "reviews", "feat--hello.md"), "# Review\n")
	_, added, _ := store.ReadBoard()
	if added == changed {
		t.Error("adding a review did not change the fingerprint")
	}
}

func TestReadReview(t *testing.T) {
	t.Parallel()

	store := open(t, sampleCopy(t))
	review, found, err := store.ReadReview("alpha", "feat--board-columns")
	if err != nil || !found || !strings.HasPrefix(review, "# Review: Board columns") {
		t.Errorf("ReadReview = %.40q, %v, %v", review, found, err)
	}
	if _, found, err := store.ReadReview("alpha", "feat--card-panel"); found || err != nil {
		t.Errorf("missing review = %v, %v", found, err)
	}
	if _, _, err := store.ReadReview("alpha", "../reviews/x"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("unsafe slug = %v", err)
	}
}

func TestOpenAttachment(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "alpha", "attachments", "feat--board-columns", "page.svg"), "<svg/>")
	write(t, filepath.Join(root, "alpha", "attachments", "feat--board-columns", "notes.txt"), "notes")
	store := open(t, root)

	file, contentType, err := store.OpenAttachment("alpha", "feat--board-columns", "20261003T1000-board-desktop.png")
	if err != nil {
		t.Fatalf("OpenAttachment: %v", err)
	}
	data, _ := io.ReadAll(file)
	file.Close()
	if contentType != "image/png" || len(data) == 0 {
		t.Errorf("png = %q, %d bytes", contentType, len(data))
	}
	if _, contentType, err := store.OpenAttachment("alpha", "feat--board-columns", "notes.txt"); err != nil || contentType != "text/plain; charset=utf-8" {
		t.Errorf("txt = %q, %v", contentType, err)
	}
	tests := []struct {
		project, ticket, file string
		want                  error
	}{
		{"alpha", "feat--board-columns", "page.svg", ErrTypeNotAllowed},
		{"alpha", "feat--board-columns", "index.yaml", ErrTypeNotAllowed},
		{"alpha", "feat--board-columns", "absent.png", ErrNotFound},
		{"alpha", "feat--board-columns", "../../todo/x.png", ErrInvalidName},
		{"alpha", "..", "x.png", ErrInvalidName},
	}
	for _, test := range tests {
		if _, _, err := store.OpenAttachment(test.project, test.ticket, test.file); !errors.Is(err, test.want) {
			t.Errorf("OpenAttachment(%s, %s, %s) = %v, want %v", test.project, test.ticket, test.file, err, test.want)
		}
	}
}
