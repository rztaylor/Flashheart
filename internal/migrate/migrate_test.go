package migrate

import (
	"bytes"
	"errors"
	"io/fs"
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

func TestConvertTicket(t *testing.T) {
	t.Parallel()

	ids := func(value string) string {
		if id, ok := map[string]string{"feat--a": "FH-7"}[value]; ok {
			return id
		}
		return value
	}
	tests := map[string]string{
		"---\ntype: bug\ndepends-on: [feat--a]\n---\n# T\n": "---\nid: FH-1\nstatus: backlog\ntype: bug\ndepends-on: [FH-7]\n---\n# T\n",
		"# No frontmatter\n": "---\nid: FH-1\nstatus: backlog\n---\n# No frontmatter\n",
		"BOM---\r\ncreated: 2026-10-03\r\ndepends-on:\r\n  - feat--a\r\n---\r\n# T\r\n": "BOM---\r\nid: FH-1\r\nstatus: backlog\r\ncreated: 2026-10-03\r\ndepends-on:\r\n  - FH-7\r\n---\r\n# T\r\n",
	}
	for input, want := range tests {
		input, want = strings.ReplaceAll(input, "BOM", "\ufeff"), strings.ReplaceAll(want, "BOM", "\ufeff")
		got, err := convertTicket([]byte(input), Ticket{ID: "FH-1", Status: board.Backlog}, ids)
		if err != nil || string(got) != want {
			t.Errorf("convertTicket(%q) =\n  %q, %v\nwant\n  %q", input, got, err, want)
		}
	}
}

func TestRewriteList(t *testing.T) {
	t.Parallel()

	ids := func(value string) string {
		if id, ok := map[string]string{"feat--a": "FH-1", "feat--b": "FH-2"}[value]; ok {
			return id
		}
		return value
	}
	tests := map[string]string{
		"---\ndepends-on: [feat--a, feat--b]\nx: 1\n---\nbody depends-on: [feat--a]\n": "---\ndepends-on: [FH-1, FH-2]\nx: 1\n---\nbody depends-on: [feat--a]\n",
		"---\ndepends-on: feat--a\n---\n":                                              "---\ndepends-on: [FH-1]\n---\n",
		"---\ndepends-on:\n  - feat--a\n  - feat--zzz\nx: 1\n---\n":                    "---\ndepends-on:\n  - FH-1\n  - feat--zzz\nx: 1\n---\n",
		"---\r\ndepends-on: [feat--a, feat--b]\r\n---\r\n":                             "---\r\ndepends-on: [FH-1, FH-2]\r\n---\r\n",
		"---\ndepends-on: []\n---\n":                                                   "---\ndepends-on: []\n---\n",
		"---\nx: 1\n---\n":                                                             "---\nx: 1\n---\n",
		"---\ndepends-on: [feat--a\n---\n":                                             "---\ndepends-on: [feat--a\n---\n",
	}
	for input, want := range tests {
		if got, err := rewriteList([]byte(input), "depends-on", ids); err != nil || string(got) != want {
			t.Errorf("rewriteList(%q) =\n  %q, %v\nwant\n  %q", input, got, err, want)
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

// TestApplyKeepsWindowsLineEndingsAndByteOrderMarks migrates the v1 fixture
// with every text file re-encoded (FH-79): each ticket keeps one frontmatter
// with its created date and its dependencies and workstream lists rewritten
// to ids, and each file keeps its line endings and byte-order mark.
func TestApplyKeepsWindowsLineEndingsAndByteOrderMarks(t *testing.T) {
	t.Parallel()

	encodings := map[string]func(string) string{
		"crlf":     func(text string) string { return strings.ReplaceAll(text, "\n", "\r\n") },
		"bom":      func(text string) string { return "\ufeff" + text },
		"bom+crlf": func(text string) string { return "\ufeff" + strings.ReplaceAll(text, "\n", "\r\n") },
	}
	for name, encode := range encodings {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root, s := v1Copy(t)
			err := filepath.WalkDir(root, func(file string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || (filepath.Ext(file) != ".md" && filepath.Ext(file) != ".yaml") {
					return err
				}
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				return os.WriteFile(file, []byte(encode(string(data))), 0o644)
			})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := Prepare(s, options())
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(s, plan); err != nil {
				t.Fatalf("Apply: %v", err)
			}

			b, _, err := s.ReadBoard()
			if err != nil {
				t.Fatal(err)
			}
			var alpha board.Project
			for _, project := range b.Projects {
				if project.Name == "alpha" {
					alpha = project
				}
			}
			if alpha.Key != "AL" || alpha.KeyDerived || alpha.NextID != 8 || alpha.DisplayName != "Alpha" {
				t.Errorf("alpha project = key %q derived %v next %d name %q", alpha.Key, alpha.KeyDerived, alpha.NextID, alpha.DisplayName)
			}
			tickets := map[string]board.Ticket{}
			for _, ticket := range alpha.Tickets {
				tickets[ticket.ID] = ticket
			}
			overflow := tickets["AL-5"]
			if overflow.Created != "2026-10-03" || !slices.Equal(overflow.DependsOn, []string{"AL-4"}) ||
				overflow.Column != board.Backlog || overflow.Type != "bug" || len(overflow.Repair) > 0 {
				t.Errorf("AL-5 = created %q depends-on %q column %q type %q repair %q", overflow.Created, overflow.DependsOn, overflow.Column, overflow.Type, overflow.Repair)
			}
			if drag := tickets["AL-4"]; drag.Created != "2026-10-02" || drag.Workstream != "board-ui" || len(drag.DependsOn) != 0 {
				t.Errorf("AL-4 = created %q workstream %q depends-on %q", drag.Created, drag.Workstream, drag.DependsOn)
			}
			if len(alpha.Workstreams) != 1 || !slices.Equal(alpha.Workstreams[0].Tickets, []string{"AL-2", "AL-3", "AL-4"}) {
				t.Errorf("workstreams = %+v", alpha.Workstreams)
			}

			for _, file := range []string{
				"alpha/tickets/AL-5-column-overflow/AL-5-column-overflow.md",
				"alpha/workstreams/board-ui.md",
				"alpha/project.yaml",
				".flashheart/config.yaml",
			} {
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
				if err != nil {
					t.Fatal(err)
				}
				text := string(data)
				if strings.HasPrefix(name, "bom") != strings.HasPrefix(text, "\ufeff") || strings.Count(text, "\ufeff") > 1 {
					t.Errorf("%s byte-order mark not kept:\n%q", file, text)
				}
				if lone := strings.Count(text, "\n") - strings.Count(text, "\r\n"); strings.HasSuffix(name, "crlf") && lone != 0 {
					t.Errorf("%s has %d lone line feeds:\n%q", file, lone, text)
				}
				if strings.HasSuffix(file, ".md") && strings.Count(strings.ReplaceAll(text, "\r\n", "\n"), "\n---\n") != 1 {
					t.Errorf("%s does not have exactly one frontmatter:\n%q", file, text)
				}
			}
			if config, _ := s.ReadConfig(); config.Version != 2 {
				t.Errorf("config version = %d", config.Version)
			}
		})
	}
}

// TestApplyWritesUnderTheRootLock: while another writer holds the root lock,
// Apply gives up busy and writes nothing (FH-79). Not parallel: it shortens
// the package-wide LockWait, and parallel tests run after it.
func TestApplyWritesUnderTheRootLock(t *testing.T) {
	old := store.LockWait
	store.LockWait = 200 * time.Millisecond
	t.Cleanup(func() { store.LockWait = old })

	root, s := v1Copy(t)
	plan, err := Prepare(s, options())
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(root, ".flashheart", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	other := store.New(root)
	t.Cleanup(func() { other.Close() })
	err = other.WithRootLock(nil, func() error { return Apply(s, plan) })
	if !errors.Is(err, store.ErrBusy) {
		t.Errorf("Apply under another's root lock = %v, want ErrBusy", err)
	}
	for _, absent := range []string{"alpha/tickets", "beta/tickets", "beta/project.yaml", ".flashheart/backup"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(absent))); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Apply wrote %s while the root was locked (%v)", absent, err)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(root, ".flashheart", "config.yaml")); !bytes.Equal(after, config) {
		t.Errorf("config.yaml changed while the root was locked: %q", after)
	}
}

// TestBrokenYAMLIsRefusedOrKept: a project.yaml that does not parse stops
// the plan before anything is written, and a workstream that does not parse
// is warned about and kept as written.
func TestBrokenYAMLIsRefusedOrKept(t *testing.T) {
	t.Parallel()

	root, s := v1Copy(t)
	projectFile := filepath.Join(root, "alpha", "project.yaml")
	if err := os.WriteFile(projectFile, []byte("name: [Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(s, options()); err == nil || !strings.Contains(err.Error(), "alpha/project.yaml does not parse") {
		t.Errorf("Prepare with a broken project.yaml = %v", err)
	}

	root, s = v1Copy(t)
	broken := "---\ntickets: [feat--card-panel\n---\n# Board UI\n"
	if err := os.WriteFile(filepath.Join(root, "alpha", "workstreams", "board-ui.md"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := Prepare(s, options())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(plan.Warnings, func(w string) bool { return strings.Contains(w, "board-ui.md does not parse") }) {
		t.Errorf("warnings = %q, want the broken workstream named", plan.Warnings)
	}
	if err := Apply(s, plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "board-ui.md")); string(data) != broken {
		t.Errorf("broken workstream rewritten: %q", data)
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
