package references

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

type fixture struct {
	t    *testing.T
	base string
	root string
	// outside is a checkout of the alpha project's repository, outside the
	// board root.
	outside string
	files   *store.Store
	board   *index.Index
	copier  *Copier
	log     bytes.Buffer
	touched time.Time
}

// newFixture serves the sample board, whose alpha project records a git
// checkout named alpha, and has seen it once, so only later edits count.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "board")
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "boards", "sample"))); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, base: base, root: root, outside: filepath.Join(base, "src", "alpha"), touched: time.Now().Add(time.Hour)}
	f.repo(f.outside)
	f.recordRepos(f.outside)
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
	return f.write(filepath.Join(f.outside, name), data)
}

func (f *fixture) write(path, data string) string {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		f.t.Fatal(err)
	}
	return path
}

// repo makes dir a git checkout that ignores secret*.
func (f *fixture) repo(dir string) {
	f.t.Helper()
	f.write(filepath.Join(dir, ".gitignore"), "secret*\n")
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("git init: %v\n%s", err, out)
	}
}

// recordRepos sets the repositories alpha's project.yaml records, as a
// synced edit could.
func (f *fixture) recordRepos(repos ...string) {
	f.t.Helper()
	name := filepath.Join(f.root, "alpha", "project.yaml")
	data, err := os.ReadFile(name)
	if err != nil {
		f.t.Fatal(err)
	}
	list := "repos:\n"
	for _, repo := range repos {
		list += "  - " + repo + "\n"
	}
	next := regexp.MustCompile(`repos:\n(  - .*\n)+`).ReplaceAllLiteralString(string(data), list)
	if next == string(data) {
		f.t.Fatalf("alpha's project.yaml is unchanged:\n%s", data)
	}
	if err := os.WriteFile(name, []byte(next), 0o644); err != nil {
		f.t.Fatal(err)
	}
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

// Writes Flashheart makes itself, such as a raw edit sent from the browser,
// are not acted on: the browser must not be able to pull local files in.
func TestEditsThroughFlashheartAreLeftAlone(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	secret := f.save("statement.pdf", "%PDF synthetic")
	_, hash, err := f.files.ReadTicket("alpha", "AL-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.files.UpdateTicket("alpha", "AL-2", hash, func(data []byte) ([]byte, error) {
		return append(data, "\n[statement]("+secret+")\n"...), nil
	}); err != nil {
		t.Fatal(err)
	}
	f.pass()
	if !strings.Contains(f.read(ticket), "[statement]("+secret+")") {
		t.Fatal("a link written through Flashheart was copied")
	}
}

// A board shared through git or a sync service gets edits from other
// people, which serve cannot tell from the user's own. Only files in a
// checkout of the ticket's project's repository that git does not ignore
// are copied, so a synced link cannot pull in anything else on this
// machine.
func TestASyncedEditCopiesOnlyFilesInTheProjectsRepository(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	desktop := f.write(filepath.Join(f.base, "Desktop", "statement.pdf"), "%PDF synthetic")
	beta := filepath.Join(f.base, "src", "beta")
	f.repo(beta)
	otherRepo := f.write(filepath.Join(beta, "plan.pdf"), "%PDF synthetic")
	unlisted := filepath.Join(f.base, "elsewhere", "alpha")
	f.repo(unlisted)
	sameName := f.write(filepath.Join(unlisted, "notes.txt"), "synthetic")
	ignored := f.save("secret-keys.json", "{}")
	gitInternals := f.save(".git/info/exclude.txt", "synthetic")
	gitInternalsCased := strings.Replace(gitInternals, ".git", ".GIT", 1)
	link := filepath.Join(f.outside, "statement-link.pdf")
	if err := os.Symlink(desktop, link); err != nil {
		t.Fatal(err)
	}
	shot := f.save("shot.png", "\x89PNG synthetic")
	// The synced edit also lists the other checkouts as alpha's.
	f.recordRepos(f.outside, filepath.Dir(desktop), beta)

	refused := []string{desktop, otherRepo, sameName, ignored, gitInternals, link}
	if _, err := os.Stat(gitInternalsCased); err == nil {
		refused = append(refused, gitInternalsCased)
	}
	text := "\n![Shot](" + shot + ")\n"
	for i, path := range refused {
		text += "[r" + string(rune('a'+i)) + "](" + path + ")\n"
	}
	f.edit(ticket, text)
	f.pass()

	got := f.read(ticket)
	if !regexp.MustCompile(`!\[Shot\]\(files/[^)]*-shot\.png\)`).MatchString(got) {
		t.Fatalf("the repository's file was not copied:\n%s", got)
	}
	for _, path := range refused {
		if !strings.Contains(got, "("+path+")") {
			t.Errorf("%s was copied:\n%s", path, got)
		}
		if !strings.Contains(f.log.String(), "links to "+path+", which was not copied") {
			t.Errorf("%s not logged:\n%s", path, f.log.String())
		}
	}
	index := f.read("alpha/tickets/AL-2-board-columns/files/index.yaml")
	if strings.Count(index, "run: serve") != 1 {
		t.Fatalf("index.yaml:\n%s", index)
	}
}
