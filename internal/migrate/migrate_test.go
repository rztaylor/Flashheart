package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/store"
)

var fixed = func() time.Time { return time.Date(2026, 10, 5, 10, 15, 0, 0, time.UTC) }

func v1Copy(t *testing.T) (string, *store.Store) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample-v1"))); err != nil {
		t.Fatal(err)
	}
	s := store.New(root)
	t.Cleanup(func() { s.Close() })
	return root, s
}

func options() Options {
	return Options{Keys: map[string]string{"alpha": "AL", "beta": "BE"}, Now: fixed}
}

func TestInsertFields(t *testing.T) {
	t.Parallel()

	got := string(insertFields([]byte("---\ntype: bug\n---\n# T\n"), "FH-1", board.Backlog))
	if got != "---\nid: FH-1\nstatus: backlog\ntype: bug\n---\n# T\n" {
		t.Errorf("insertFields = %q", got)
	}
	got = string(insertFields([]byte("# No frontmatter\n"), "FH-2", board.Done))
	if got != "---\nid: FH-2\nstatus: done\n---\n# No frontmatter\n" {
		t.Errorf("insertFields without frontmatter = %q", got)
	}
}

func TestRewriteList(t *testing.T) {
	t.Parallel()

	ids := func(value string) string {
		return map[string]string{"feat--a": "FH-1", "feat--b": "FH-2"}[value]
	}
	tests := map[string]string{
		"---\ndepends-on: [feat--a, feat--b]\nx: 1\n---\nbody depends-on: [feat--a]\n": "---\ndepends-on: [FH-1, FH-2]\nx: 1\n---\nbody depends-on: [feat--a]\n",
		"---\ndepends-on: feat--a\n---\n":                                              "---\ndepends-on: FH-1\n---\n",
		"---\ndepends-on:\n  - feat--a\n  - feat--zzz # keep\nx: 1\n---\n":             "---\ndepends-on:\n  - FH-1\n  - feat--zzz # keep\nx: 1\n---\n",
		"---\ndepends-on: []\n---\n":                                                   "---\ndepends-on: []\n---\n",
	}
	for input, want := range tests {
		if got := string(rewriteList([]byte(input), "depends-on", ids)); got != want {
			t.Errorf("rewriteList(%q) =\n  %q\nwant\n  %q", input, got, want)
		}
	}
}

func TestRewriteLinks(t *testing.T) {
	t.Parallel()

	folders := map[string]string{"feat--board-columns": "AL-2-board-columns", "feat--x": "AL-9-x"}
	ids := map[string]string{"feat--board-columns": "AL-2", "feat--x": "AL-9"}
	body := "**Work Item:** [feat--board-columns](../ready-to-review/feat--board-columns.md)\n" +
		"See [the other one](../todo/feat--x.md#notes) and [web](https://example.com/a.md).\n" +
		"![Board](../attachments/feat--board-columns/shot.png) ![Other](../attachments/feat--x/b.png)\n"
	got := rewriteLinks(body, "AL-2-board-columns", folders, ids)
	want := "**Work Item:** [AL-2](AL-2-board-columns.md)\n" +
		"See [the other one](../AL-9-x/AL-9-x.md#notes) and [web](https://example.com/a.md).\n" +
		"![Board](files/shot.png) ![Other](../AL-9-x/files/b.png)\n"
	if got != want {
		t.Errorf("rewriteLinks =\n%s\nwant\n%s", got, want)
	}
}

func TestPrepareNumbersTicketsInCreationOrder(t *testing.T) {
	t.Parallel()

	_, s := v1Copy(t)
	plan, err := Prepare(s, options())
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(plan.Projects) != 2 || plan.Backup != ".flashheart/backup/v1-20261005T101500Z" {
		t.Fatalf("plan = %+v", plan)
	}
	alpha := plan.Projects[0]
	var got []string
	for _, ticket := range alpha.Tickets {
		got = append(got, ticket.ID+" "+ticket.Folder+" "+string(ticket.Status))
	}
	want := []string{
		"AL-1 AL-1-project-skeleton done",
		"AL-2 AL-2-board-columns review",
		"AL-3 AL-3-card-panel in-progress",
		"AL-4 AL-4-drag-and-drop backlog",
		"AL-5 AL-5-column-overflow backlog",
		"AL-6 AL-6-offline-mode backlog",
		"AL-7 AL-7-broken-frontmatter backlog",
	}
	if !slices.Equal(got, want) {
		t.Errorf("tickets =\n  %q\nwant\n  %q", got, want)
	}
	if alpha.Key != "AL" || alpha.NextID != 8 || alpha.KeySource != "--key" {
		t.Errorf("alpha = %s %d %s", alpha.Key, alpha.NextID, alpha.KeySource)
	}
	if !slices.ContainsFunc(plan.Warnings, func(w string) bool { return strings.Contains(w, "feat--does-not-exist") }) {
		t.Errorf("warnings = %q, want the unknown dependency named", plan.Warnings)
	}
	var out bytes.Buffer
	plan.Write(&out)
	for _, line := range []string{"alpha → key AL (--key)", "AL-3  in-progress/feat--card-panel.md → tickets/AL-3-card-panel (in-progress)", "Nothing is deleted.", "Run again with --write"} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("plan output lacks %q:\n%s", line, out.String())
		}
	}
}

func TestPrepareWritesNothing(t *testing.T) {
	t.Parallel()

	root, s := v1Copy(t)
	before := snapshot(t, root)
	if _, err := Prepare(s, options()); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, root); !slices.Equal(before, after) {
		t.Errorf("dry run changed the tree:\nbefore %q\nafter  %q", before, after)
	}
}

func TestApplyProducesTheV2Tree(t *testing.T) {
	t.Parallel()

	root, s := v1Copy(t)
	plan, err := Prepare(s, options())
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(s, plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}

	overflow := read("alpha/tickets/AL-5-column-overflow/AL-5-column-overflow.md")
	if !strings.HasPrefix(overflow, "---\nid: AL-5\nstatus: backlog\ntype: bug\n") || !strings.Contains(overflow, "depends-on: [AL-4]") {
		t.Errorf("column overflow ticket =\n%s", overflow)
	}
	if offline := read("alpha/tickets/AL-6-offline-mode/AL-6-offline-mode.md"); !strings.Contains(offline, "depends-on: [feat--does-not-exist]") {
		t.Errorf("unknown dependency was not kept:\n%s", offline)
	}
	broken := read("alpha/tickets/AL-7-broken-frontmatter/AL-7-broken-frontmatter.md")
	original, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "boards", "sample-v1", "alpha", "todo", "docs--broken-frontmatter.md"))
	if broken != string(original) {
		t.Errorf("broken ticket was rewritten:\n%s", broken)
	}
	if ws := read("alpha/workstreams/board-ui.md"); !strings.Contains(ws, "  - AL-2\n  - AL-3\n  - AL-4\n") {
		t.Errorf("workstream =\n%s", ws)
	}
	review := read("alpha/tickets/AL-2-board-columns/review.md")
	if !strings.Contains(review, "[AL-2](AL-2-board-columns.md)") || !strings.Contains(review, "(files/20261003T1000-board-desktop.png)") {
		t.Errorf("review =\n%s", review)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-2-board-columns", "files", "index.yaml")); err != nil {
		t.Errorf("files index not moved: %v", err)
	}
	if project := read("alpha/project.yaml"); !strings.Contains(project, "key: AL\n") || !strings.Contains(project, "next_id: 8\n") || !strings.Contains(project, "name: Alpha") {
		t.Errorf("alpha project.yaml =\n%s", project)
	}
	if project := read("beta/project.yaml"); project != "key: BE\nnext_id: 2\n" {
		t.Errorf("beta project.yaml = %q", project)
	}
	if config := read(".flashheart/config.yaml"); !strings.Contains(config, "version: 2\n") {
		t.Errorf("config.yaml =\n%s", config)
	}
	for _, gone := range []string{"alpha/todo", "alpha/reviews", "alpha/attachments", "beta/todo"} {
		if _, err := os.Stat(filepath.Join(root, gone)); !os.IsNotExist(err) {
			t.Errorf("%s still exists", gone)
		}
	}
	backup := filepath.Join(root, ".flashheart", "backup", "v1-20261005T101500Z")
	for _, kept := range []string{"alpha/todo/docs--broken-frontmatter.md", "alpha/reviews/feat--board-columns.md", "alpha/workstreams/board-ui.md", "alpha/project.yaml", ".flashheart/config.yaml"} {
		if _, err := os.Stat(filepath.Join(backup, filepath.FromSlash(kept))); err != nil {
			t.Errorf("backup lacks %s: %v", kept, err)
		}
	}

	// The migrated board reads cleanly, and a second run has nothing to do.
	b, _, err := s.ReadBoard()
	if err != nil {
		t.Fatal(err)
	}
	analysis := board.Analyze(b)
	if reasons := analysis.Blocked[board.Ref{Project: "alpha", ID: "AL-5"}]; len(reasons) != 1 || reasons[0].Ticket.ID != "AL-4" {
		t.Errorf("AL-5 reasons = %+v", reasons)
	}
	again, err := Prepare(s, options())
	if err != nil || len(again.Projects) != 0 {
		t.Errorf("second Prepare = %+v, %v", again, err)
	}
}

func TestPrepareRejectsBadOrDuplicateKeys(t *testing.T) {
	t.Parallel()

	_, s := v1Copy(t)
	if _, err := Prepare(s, Options{Keys: map[string]string{"alpha": "al"}, Now: fixed}); err == nil || !strings.Contains(err.Error(), `"al"`) {
		t.Errorf("lowercase key err = %v", err)
	}
	if _, err := Prepare(s, Options{Keys: map[string]string{"alpha": "SAME", "beta": "SAME"}, Now: fixed}); err == nil || !strings.Contains(err.Error(), "SAME") {
		t.Errorf("duplicate key err = %v", err)
	}
	if _, err := Prepare(s, Options{Keys: map[string]string{"gamma": "GA"}, Now: fixed}); err == nil || !strings.Contains(err.Error(), "gamma") {
		t.Errorf("unknown project err = %v", err)
	}
}

func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		entries = append(entries, rel+" "+info.ModTime().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
