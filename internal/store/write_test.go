package store

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

var fixedNow = time.Date(2026, 10, 5, 14, 12, 9, 0, time.UTC)

func writable(t *testing.T) (*Store, string) {
	t.Helper()
	root := sampleCopy(t)
	s := open(t, root)
	s.SetClock(func() time.Time { return fixedNow })
	return s, root
}

func setStatus(status string) Edit {
	return func(data []byte) ([]byte, error) { return mdfile.SetScalar(data, "status", status) }
}

func TestUpdateTicketWritesStampsAndChecksTheHash(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	data, hash, err := s.ReadTicket("alpha", "AL-4")
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.UpdateTicket("alpha", "AL-4", hash, setStatus("in-progress"))
	if err != nil {
		t.Fatal(err)
	}
	written, _ := os.ReadFile(filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "AL-4-drag-and-drop.md"))
	if Hash(written) != next {
		t.Errorf("returned hash does not match the file")
	}
	doc := mdfile.Parse(written)
	if status, _ := doc.String("status"); status != "in-progress" {
		t.Errorf("status = %q", status)
	}
	if updated, _ := doc.String("updated"); updated != "2026-10-05T14:12:09Z" {
		t.Errorf("updated = %q", updated)
	}

	// The old hash is now stale: the write is refused with the current file.
	_, err = s.UpdateTicket("alpha", "AL-4", hash, setStatus("done"))
	var conflict *ConflictError
	if !errors.As(err, &conflict) || !errors.Is(err, ErrConflict) || conflict.Hash != next || string(conflict.Current) != string(written) {
		t.Fatalf("stale write error = %v", err)
	}
	if string(data) == string(written) {
		t.Fatal("ticket did not change")
	}
}

func TestUnchangedEditWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	name := filepath.Join(root, "alpha", "tickets", "AL-3-card-panel", "AL-3-card-panel.md")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(name, old, old); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(name)
	hash, err := s.UpdateTicket("alpha", "AL-3", "", setStatus("in-progress"))
	if err != nil || hash != Hash(before) {
		t.Fatalf("UpdateTicket = %s, %v", hash, err)
	}
	info, _ := os.Stat(name)
	after, _ := os.ReadFile(name)
	if !info.ModTime().Equal(old) || string(after) != string(before) {
		t.Error("a no-op edit rewrote the ticket")
	}
}

func TestWritesRefuseUnsafeNames(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	for _, call := range []func() error{
		func() error { _, err := s.UpdateTicket("../alpha", "AL-1", "", setStatus("done")); return err },
		func() error { _, err := s.UpdateTicket("alpha", "../AL-1", "", setStatus("done")); return err },
		func() error { _, err := s.UpdateWorkstream("alpha", "../x", "", setStatus("done")); return err },
		func() error { return s.Archive("alpha", "AL-1/..") },
	} {
		if err := call(); !errors.Is(err, ErrInvalidName) {
			t.Errorf("error = %v, want ErrInvalidName", err)
		}
	}
	if _, err := s.UpdateTicket("alpha", "AL-99", "", setStatus("done")); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing ticket error = %v", err)
	}
}

func TestCreateTicketUsesTheNextIDAndTemplate(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	created, err := s.CreateTicket("alpha", NewTicket{
		Title:       "Keyboard   moves: between columns!",
		Type:        "bug",
		Priority:    "high",
		Status:      board.UpNext,
		Workstream:  "board-ui",
		Description: "Arrow keys should move cards.",
		Criteria:    []string{"Move left", " ", "Move right"},
		DependsOn:   []string{"AL-4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "AL-9" || created.Folder != "AL-9-keyboard-moves-between-columns" || created.Key != "" {
		t.Errorf("created = %+v", created)
	}
	data, err := os.ReadFile(filepath.Join(root, "alpha", "tickets", created.Folder, created.Folder+".md"))
	if err != nil {
		t.Fatal(err)
	}
	ticket := board.ParseTicket(created.Folder, data)
	if ticket.NeedsRepair() || len(ticket.Warnings) > 0 || ticket.ID != "AL-9" || ticket.Column != board.UpNext ||
		ticket.Title != "Keyboard moves: between columns!" || ticket.Type != "bug" || ticket.Workstream != "board-ui" ||
		!slices.Equal(ticket.DependsOn, []string{"AL-4"}) || ticket.Created != "2026-10-05" {
		t.Errorf("ticket = %+v\n%s", ticket, data)
	}
	for _, want := range []string{"- [ ] Move left\n- [ ] Move right\n", "## Reproduction\n", "## Notes\n\nNone.\n"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("ticket lacks %q:\n%s", want, data)
		}
	}
	project, _ := os.ReadFile(filepath.Join(root, "alpha", "project.yaml"))
	if !strings.Contains(string(project), "next_id: 10\n") || !strings.Contains(string(project), "  enforce_handoff: false\n") {
		t.Errorf("project.yaml =\n%s", project)
	}
	second, err := s.CreateTicket("alpha", NewTicket{Title: "Another"})
	if err != nil || second.ID != "AL-10" {
		t.Errorf("second = %+v, %v", second, err)
	}
	if _, err := s.CreateTicket("alpha", NewTicket{Title: " "}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty title error = %v", err)
	}
}

func TestNewProjectKeys(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	write(t, filepath.Join(root, "gamma-ray", "project.yaml"), "name: Gamma ray\n")
	write(t, filepath.Join(root, "alpine", "project.yaml"), "name: Alpine\n")

	// A key someone else holds is refused with the keys in use.
	var taken *KeyTakenError
	if err := s.SetProjectKey("gamma-ray", "AL"); !errors.As(err, &taken) || !slices.Equal(taken.InUse, []string{"AL", "BE"}) {
		t.Errorf("taken key error = %v", err)
	}
	if err := s.SetProjectKey("gamma-ray", "g"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("invalid key error = %v", err)
	}
	if err := s.SetProjectKey("gamma-ray", "GR"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProjectKey("alpha", "AA"); !errors.Is(err, ErrKeyFixed) {
		t.Errorf("fixed key error = %v", err)
	}
	created, err := s.CreateTicket("gamma-ray", NewTicket{Title: "First"})
	if err != nil || created.ID != "GR-1" || created.Key != "" {
		t.Errorf("created = %+v, %v", created, err)
	}
	if err := s.SetProjectKey("gamma-ray", "GX"); !errors.Is(err, ErrKeyFixed) {
		t.Errorf("key change after a ticket error = %v", err)
	}

	// With no key chosen, the first ticket records the derived key, adding
	// a digit when it is taken (alpine derives ALP; make that taken).
	if err := s.SetProjectKey("beta", "BE"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "delta", "project.yaml"), "key: ALP\n")
	created, err = s.CreateTicket("alpine", NewTicket{Title: "First"})
	if err != nil || created.ID != "ALP2-1" || created.Key != "ALP2" {
		t.Errorf("derived key = %+v, %v", created, err)
	}
	project, _ := os.ReadFile(filepath.Join(root, "alpine", "project.yaml"))
	if string(project) != "name: Alpine\nkey: ALP2\nnext_id: 2\n" {
		t.Errorf("alpine project.yaml = %q", project)
	}
}

func TestArchiveAndUnarchive(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.Archive("alpha", "AL-6"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ".archive", "tickets", "AL-6-offline-mode", "AL-6-offline-mode.md")); err != nil {
		t.Fatal(err)
	}
	project, _ := s.ReadProject("alpha")
	if !slices.Contains(project.Archived, "AL-6") {
		t.Errorf("archived = %v", project.Archived)
	}
	if err := s.Unarchive("alpha", "AL-6"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "tickets", "AL-6-offline-mode")); err != nil {
		t.Fatal(err)
	}
	if err := s.Unarchive("alpha", "AL-6"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second unarchive error = %v", err)
	}
}

func TestWorkstreamOrderKeepsBlockStyle(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	_, err := s.UpdateWorkstream("alpha", "board-ui", "", func(data []byte) ([]byte, error) {
		return mdfile.SetList(data, "tickets", []string{"AL-3", "AL-2", "AL-4"})
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "alpha", "workstreams", "board-ui.md"))
	if !strings.Contains(string(data), "tickets:\n  - AL-3\n  - AL-2\n  - AL-4\n") || strings.Contains(string(data), "updated:") {
		t.Errorf("workstream =\n%s", data)
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	for title, want := range map[string]string{
		"Card panel":                  "card-panel",
		"Fix: the  Café's menu (v2)!": "fix-the-caf-s-menu-v2",
		"!!!":                         "ticket",
		strings.Repeat("word ", 20):   "word-word-word-word-word-word-word-word-word",
		strings.Repeat("x", 60):       strings.Repeat("x", 48),
	} {
		if got := Slugify(title); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", title, got, want)
		}
	}
}

// Multi-process tests run this test binary as separate writers (STO-3).

const helperEnv = "FLASHHEART_STORE_HELPER"

func TestHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		t.Skip("helper process only")
	}
	args := strings.Split(os.Getenv("FLASHHEART_HELPER_ARGS"), "|")
	s, err := Open(args[0])
	if err != nil {
		fmt.Println("error", err)
		os.Exit(0)
	}
	// Start together so the writes overlap.
	for time.Now().Before(parseTime(args[1])) {
		time.Sleep(time.Millisecond)
	}
	switch mode {
	case "update":
		_, err = s.UpdateTicket("alpha", "AL-4", args[2], func(data []byte) ([]byte, error) {
			time.Sleep(20 * time.Millisecond) // hold the lock while the other waits
			return mdfile.SetScalar(data, "branch", args[3])
		})
	case "key":
		err = s.SetProjectKey(args[2], args[3])
	}
	switch {
	case err == nil:
		fmt.Println("ok")
	case errors.Is(err, ErrConflict):
		fmt.Println("conflict")
	case errors.Is(err, ErrKeyTaken):
		fmt.Println("taken")
	default:
		fmt.Println("error", err)
	}
	os.Exit(0)
}

func parseTime(value string) time.Time {
	at, _ := time.Parse(time.RFC3339Nano, value)
	return at
}

func runWriters(t *testing.T, mode string, argSets ...[]string) []string {
	t.Helper()
	start := time.Now().Add(300 * time.Millisecond).Format(time.RFC3339Nano)
	outputs := make([]string, len(argSets))
	var wg sync.WaitGroup
	for index, args := range argSets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(os.Environ(), helperEnv+"="+mode,
				"FLASHHEART_HELPER_ARGS="+strings.Join(append([]string{args[0], start}, args[1:]...), "|"))
			out, err := cmd.Output()
			if err != nil {
				t.Errorf("writer %d: %v", index, err)
			}
			outputs[index] = strings.TrimSpace(string(out))
		}()
	}
	wg.Wait()
	slices.Sort(outputs)
	return outputs
}

func TestConcurrentWritersOneWinsTheOtherConflicts(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	name := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "AL-4-drag-and-drop.md")
	original, _ := os.ReadFile(name)
	base := Hash(original)

	// Watch the file the whole time: every read must be a whole version.
	stop := make(chan struct{})
	var torn []string
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(name)
			if err != nil {
				torn = append(torn, err.Error())
				continue
			}
			if doc := mdfile.Parse(data); doc.FrontmatterError != nil || !strings.HasSuffix(string(data), string(original[len(original)-40:])) {
				torn = append(torn, string(data))
			}
		}
	}()

	outputs := runWriters(t, "update", []string{root, base, "feature/one"}, []string{root, base, "feature/two"})
	close(stop)
	watcher.Wait()
	if !slices.Equal(outputs, []string{"conflict", "ok"}) {
		t.Errorf("writers = %v, want one ok and one conflict", outputs)
	}
	if len(torn) > 0 {
		t.Errorf("observed %d partial files, first: %.200s", len(torn), torn[0])
	}
	data, _ := os.ReadFile(name)
	branch, _ := mdfile.Parse(data).String("branch")
	if branch != "feature/one" && branch != "feature/two" {
		t.Errorf("branch = %q", branch)
	}
}

func TestConcurrentKeyChoicesOneWins(t *testing.T) {
	t.Parallel()

	root := sampleCopy(t)
	write(t, filepath.Join(root, "first", "project.yaml"), "name: First\n")
	write(t, filepath.Join(root, "second", "project.yaml"), "name: Second\n")
	outputs := runWriters(t, "key", []string{root, "first", "XY"}, []string{root, "second", "XY"})
	if !slices.Equal(outputs, []string{"ok", "taken"}) {
		t.Errorf("key writers = %v, want one ok and one taken", outputs)
	}
}
