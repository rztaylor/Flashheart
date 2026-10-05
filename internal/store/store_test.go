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

func TestNewWaitsForTheRootToExist(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "later")
	store := New(root)
	defer store.Close()
	if _, _, err := store.ReadBoard(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadBoard before the root exists = %v, want ErrNotExist", err)
	}
	write(t, filepath.Join(root, "late", "tickets", "LA-1-x", "ticket.md"), "---\nid: LA-1\nstatus: backlog\n---\n# X\n")
	b, _, err := store.ReadBoard()
	if err != nil || len(b.Projects) != 1 || b.Projects[0].Name != "late" {
		t.Errorf("ReadBoard after creation = %+v, %v", b.Projects, err)
	}
}

func TestProjectDiscovery(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "_ignored", "tickets", "IG-1-x", "ticket.md"), "# X\n")
	write(t, filepath.Join(root, ".hidden", "tickets", "HI-1-x", "ticket.md"), "# X\n")
	write(t, filepath.Join(root, "_scratch", "tickets", "SC-1-s", "ticket.md"), "# S\n")
	write(t, filepath.Join(root, "only-config", "project.yaml"), "name: Only\n")
	write(t, filepath.Join(root, "legacy", "todo", "feat--x.md"), "# X\n")
	write(t, filepath.Join(root, "notes.md"), "not a project\n")
	outside := t.TempDir()
	write(t, filepath.Join(outside, "tickets", "OU-1-x", "ticket.md"), "# X\n")
	if err := os.Symlink(outside, filepath.Join(root, "escapes")); err != nil {
		t.Fatal(err)
	}

	store := open(t, root)
	names, err := store.Projects()
	if err != nil {
		t.Fatalf("Projects(): %v", err)
	}
	if want := []string{"_scratch", "alpha", "beta", "only-config"}; !reflect.DeepEqual(names, want) {
		t.Errorf("Projects() = %q, want %q", names, want)
	}
	if v1, err := store.V1Projects(); err != nil || !reflect.DeepEqual(v1, []string{"legacy"}) {
		t.Errorf("V1Projects() = %q, %v", v1, err)
	}
}

func TestReadSampleProject(t *testing.T) {
	t.Parallel()

	store := open(t, sampleCopy(t))
	project, err := store.ReadProject("alpha")
	if err != nil {
		t.Fatalf("ReadProject: %v", err)
	}
	if project.DisplayName != "Alpha" || project.Key != "AL" || project.KeyDerived || project.NextID != 9 || !reflect.DeepEqual(project.Repos, []string{"/Users/example/src/alpha"}) {
		t.Errorf("project = %s %q key=%s derived=%v next=%d", project.Name, project.DisplayName, project.Key, project.KeyDerived, project.NextID)
	}
	var placed []string
	for _, ticket := range project.Tickets {
		placed = append(placed, string(ticket.Column)+"/"+ticket.ID)
	}
	want := []string{"backlog/AL-5", "backlog/AL-6", "backlog/AL-7", "up-next/AL-4", "in-progress/AL-3", "review/AL-2", "done/AL-1"}
	if !reflect.DeepEqual(placed, want) {
		t.Errorf("tickets =\n  %q\nwant\n  %q", placed, want)
	}
	for _, ticket := range project.Tickets {
		if ticket.Modified.IsZero() {
			t.Errorf("%s has no modification time", ticket.ID)
		}
		if (ticket.ID == "AL-7") != ticket.NeedsRepair() {
			t.Errorf("%s NeedsRepair = %v: %q", ticket.ID, ticket.NeedsRepair(), ticket.Repair)
		}
	}
	if len(project.Workstreams) != 1 || !reflect.DeepEqual(project.Workstreams[0].Tickets, []string{"AL-2", "AL-3", "AL-4"}) {
		t.Errorf("workstreams = %+v", project.Workstreams)
	}
	if !project.Reviews["AL-2"] || len(project.Reviews) != 1 {
		t.Errorf("reviews = %v", project.Reviews)
	}
	if files := project.Attachments["AL-2"]; len(files) != 1 || files[0].File != "20261003T1000-board-desktop.png" || files[0].Kind != "screenshot" {
		t.Errorf("files = %+v", project.Attachments)
	}
	if !reflect.DeepEqual(project.Archived, []string{"AL-8"}) {
		t.Errorf("archived = %q", project.Archived)
	}

	beta, err := store.ReadProject("beta")
	if err != nil {
		t.Fatal(err)
	}
	if beta.DisplayName != "beta" || beta.Key != "BE" || len(beta.Tickets) != 1 || len(beta.Warnings) != 0 {
		t.Errorf("beta = %q %s %d tickets, warnings %q", beta.DisplayName, beta.Key, len(beta.Tickets), beta.Warnings)
	}
}

func TestKeysAndNextIDs(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "beta", "project.yaml"), "name: Beta\n")
	write(t, filepath.Join(root, "gamma", "tickets", "GAM-4-x", "ticket.md"), "---\nid: GAM-4\nstatus: backlog\n---\n# X\n")
	write(t, filepath.Join(root, "delta", "project.yaml"), "key: lower\nnext_id: 2\n")
	write(t, filepath.Join(root, "delta", "tickets", "DEL-7-x", "ticket.md"), "---\nid: DEL-7\nstatus: backlog\n---\n# X\n")
	store := open(t, root)

	beta, _ := store.ReadProject("beta")
	if beta.Key != "BET" || !beta.KeyDerived || !strings.Contains(beta.Warnings[0], "no key in project.yaml; using BET") {
		t.Errorf("beta key=%s derived=%v warnings=%q", beta.Key, beta.KeyDerived, beta.Warnings)
	}
	gamma, _ := store.ReadProject("gamma")
	if gamma.Key != "GAM" || gamma.NextID != 5 {
		t.Errorf("gamma key=%s next=%d", gamma.Key, gamma.NextID)
	}
	delta, _ := store.ReadProject("delta")
	if delta.Key != "DEL" || delta.NextID != 8 || !strings.Contains(strings.Join(delta.Warnings, "\n"), `key "lower"`) {
		t.Errorf("delta key=%s next=%d warnings=%q", delta.Key, delta.NextID, delta.Warnings)
	}
}

func TestReadProjectRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	store := open(t, sampleCopy(t))
	for _, name := range []string{"", ".", "..", "../alpha", "alpha/tickets", ".flashheart", "_ignored", `a\b`} {
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
	write(t, outside, "---\nid: BE-9\n---\n# Secret\n")
	if err := os.MkdirAll(filepath.Join(root, "beta", "tickets", "BE-2-escape"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "beta", "tickets", "BE-2-escape", "ticket.md")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "beta", "tickets", "BE-3-huge", "ticket.md"), "# Huge\n"+strings.Repeat("x", MaxFileBytes))
	write(t, filepath.Join(root, "beta", "tickets", "README.txt"), "ignored\n")
	write(t, filepath.Join(root, "beta", "tickets", ".BE-4-hidden", "ticket.md"), "# hidden\n")
	if err := os.MkdirAll(filepath.Join(root, "beta", "tickets", "BE-5-empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	project, err := open(t, root).ReadProject("beta")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]board.Ticket{}
	for _, ticket := range project.Tickets {
		byID[ticket.ID] = ticket
	}
	if keys := slices.Sorted(maps.Keys(byID)); !reflect.DeepEqual(keys, []string{"BE-1", "BE-2", "BE-3"}) {
		t.Errorf("tickets = %q", keys)
	}
	escape := byID["BE-2"]
	if !escape.NeedsRepair() || !strings.Contains(escape.Repair[0], "outside the board root") || strings.Contains(escape.Body, "Secret") {
		t.Errorf("escaping symlink = %+v", escape)
	}
	if huge := byID["BE-3"]; !huge.NeedsRepair() || !strings.Contains(huge.Repair[0], "larger than") {
		t.Errorf("huge = %+v", huge.Repair)
	}
}

func TestDuplicateKeysAreFlagged(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "beta", "project.yaml"), "key: AL\n")
	b, _, err := open(t, root).ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range b.Projects {
		if !slices.ContainsFunc(project.Warnings, func(w string) bool { return strings.Contains(w, "key AL is also used by") }) {
			t.Errorf("%s warnings = %q", project.Name, project.Warnings)
		}
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

	path := filepath.Join(root, "beta", "tickets", "BE-1-hello", "ticket.md")
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

	write(t, filepath.Join(root, "beta", "tickets", "BE-1-hello", "review.md"), "# Review\n")
	_, added, _ := store.ReadBoard()
	if added == changed {
		t.Error("adding a review did not change the fingerprint")
	}
}

func TestReadReview(t *testing.T) {
	t.Parallel()

	store := open(t, sampleCopy(t))
	review, found, err := store.ReadReview("alpha", "AL-2-board-columns")
	if err != nil || !found || !strings.HasPrefix(review, "# Review: Board columns") {
		t.Errorf("ReadReview = %.40q, %v, %v", review, found, err)
	}
	if _, found, err := store.ReadReview("alpha", "AL-3-card-panel"); found || err != nil {
		t.Errorf("missing review = %v, %v", found, err)
	}
	if _, _, err := store.ReadReview("alpha", "../tickets"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("unsafe folder = %v", err)
	}
}

func TestOpenAttachment(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	files := filepath.Join(root, "alpha", "tickets", "AL-2-board-columns", "files")
	write(t, filepath.Join(files, "page.svg"), "<svg/>")
	write(t, filepath.Join(files, "notes.txt"), "notes")
	store := open(t, root)

	file, contentType, err := store.OpenAttachment("alpha", "AL-2-board-columns", "20261003T1000-board-desktop.png")
	if err != nil {
		t.Fatalf("OpenAttachment: %v", err)
	}
	data, _ := io.ReadAll(file)
	file.Close()
	if contentType != "image/png" || len(data) == 0 {
		t.Errorf("png = %q, %d bytes", contentType, len(data))
	}
	if _, contentType, err := store.OpenAttachment("alpha", "AL-2-board-columns", "notes.txt"); err != nil || contentType != "text/plain; charset=utf-8" {
		t.Errorf("txt = %q, %v", contentType, err)
	}
	tests := []struct {
		project, folder, file string
		want                  error
	}{
		{"alpha", "AL-2-board-columns", "page.svg", ErrTypeNotAllowed},
		{"alpha", "AL-2-board-columns", "index.yaml", ErrTypeNotAllowed},
		{"alpha", "AL-2-board-columns", "absent.png", ErrNotFound},
		{"alpha", "AL-2-board-columns", "../../x.png", ErrInvalidName},
		{"alpha", "..", "x.png", ErrInvalidName},
	}
	for _, test := range tests {
		if _, _, err := store.OpenAttachment(test.project, test.folder, test.file); !errors.Is(err, test.want) {
			t.Errorf("OpenAttachment(%s, %s, %s) = %v, want %v", test.project, test.folder, test.file, err, test.want)
		}
	}
}

func TestWritePrimitivesStayInsideTheRoot(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	store := open(t, root)
	if err := store.WriteFileAtomic("alpha/new/dir/file.md", []byte("hello")); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "alpha", "new", "dir", "file.md")); string(data) != "hello" {
		t.Errorf("written = %q", data)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "alpha", "new", "dir"))
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
	if err := store.WriteFileAtomic("../outside.md", []byte("x")); err == nil {
		t.Error("WriteFileAtomic escaped the root")
	}
	if err := store.Move("alpha/new", "alpha/moved/new"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if err := store.Move("alpha/moved/new", "alpha/project.yaml"); err == nil {
		t.Error("Move replaced an existing file")
	}
	if err := store.Move("alpha/moved", "../escaped"); err == nil {
		t.Error("Move escaped the root")
	}
}
