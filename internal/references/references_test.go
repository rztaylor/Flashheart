package references

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

type fixture struct {
	t       *testing.T
	root    string
	outside string
	files   *store.Store
	board   *index.Index
	copier  *Copier
	log     bytes.Buffer
	touched time.Time
}

// newFixture serves the sample board and has seen it once, so only later
// edits count.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root, outside: filepath.Join(base, "Desktop"), touched: time.Now().Add(time.Hour)}
	if err := os.MkdirAll(f.outside, 0o755); err != nil {
		t.Fatal(err)
	}
	f.files = store.New(root)
	t.Cleanup(func() { f.files.Close() })
	f.board = index.New(f.files, index.Options{})
	f.copier = New(f.files, root, 1<<20, &f.log)
	f.pass()
	return f
}

func (f *fixture) pass() {
	f.t.Helper()
	snapshot, err := f.board.Rebuild()
	if err != nil {
		f.t.Fatal(err)
	}
	f.copier.Pass(snapshot)
}

// edit appends text to a board file as a text editor would, a little later.
func (f *fixture) edit(name, text string) {
	f.t.Helper()
	path := filepath.Join(f.root, name)
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, text...), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.touched = f.touched.Add(time.Second)
	if err := os.Chtimes(path, f.touched, f.touched); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) read(name string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *fixture) save(name, data string) string {
	f.t.Helper()
	path := filepath.Join(f.outside, name)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

const ticket = "alpha/tickets/AL-2-board-columns/AL-2-board-columns.md"

func TestADirectlyEditedTicketGetsCopiesOfTheFilesItLinks(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shot := f.save("my shot.png", "\x89PNG synthetic")
	notes := f.save("notes.txt", "synthetic log")
	f.edit(ticket, "\n![Board](<"+shot+">) and [the log](file://"+notes+" \"log\").\n"+
		"```\n![example](/tmp/example.png)\n```\n"+
		"[source](/Users/example/src/demo/main.go) [missing](/Users/example/gone.png) [inside]("+filepath.Join(f.root, "alpha", "project.yaml")+")\n")
	f.pass()

	text := f.read(ticket)
	images := regexp.MustCompile(`!\[Board\]\(files/[^)]*-my-shot\.png\)`)
	logs := regexp.MustCompile(`\[the log\]\(files/[^)]*-notes\.txt "log"\)`)
	if !images.MatchString(text) || !logs.MatchString(text) {
		t.Fatalf("links not rewritten:\n%s", text)
	}
	for _, kept := range []string{"![example](/tmp/example.png)", "[source](/Users/example/src/demo/main.go)", "[missing](/Users/example/gone.png)", "[inside](" + f.root} {
		if !strings.Contains(text, kept) {
			t.Fatalf("%q changed:\n%s", kept, text)
		}
	}
	index := f.read("alpha/tickets/AL-2-board-columns/files/index.yaml")
	if !strings.Contains(index, "my-shot.png\n  caption: Board\n  kind: screenshot") || !strings.Contains(index, "kind: log") {
		t.Fatalf("index.yaml:\n%s", index)
	}
	if got := f.log.String(); strings.Count(got, "links to /Users/example/gone.png") != 1 || strings.Count(got, "\n") != 1 || strings.Contains(got, "main.go") {
		t.Fatalf("log:\n%s", got)
	}

	// Its own rewrite, and a pass with nothing new, change nothing more.
	f.pass()
	f.pass()
	if f.read(ticket) != text || strings.Count(f.log.String(), "\n") != 1 {
		t.Fatalf("a later pass changed the ticket or logged again:\n%s", f.log.String())
	}
}

func TestADirectlyEditedReviewGetsCopies(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shot := f.save("after.png", "\x89PNG synthetic")
	f.edit("alpha/tickets/AL-2-board-columns/review.md", "\n![After]("+shot+")\n")
	f.pass()
	if text := f.read("alpha/tickets/AL-2-board-columns/review.md"); !regexp.MustCompile(`!\[After\]\(files/[^)]*-after\.png\)`).MatchString(text) {
		t.Fatalf("review:\n%s", text)
	}
}

func TestEditsMadeBeforeServeStartedAreLeftAlone(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	shot := f.save("before.png", "\x89PNG synthetic")
	f.edit(ticket, "\n![Before]("+shot+")\n")
	// A new serve process sees the board for the first time.
	f.copier = New(f.files, f.root, 1<<20, &f.log)
	f.pass()
	if !strings.Contains(f.read(ticket), "![Before]("+shot+")") {
		t.Fatal("the first pass rewrote a ticket")
	}
}
