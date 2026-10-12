package events

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

// ReadSelected gives exactly the events Read gives whose header keep
// accepts, in order, whether a line is in the envelope's own field order
// (judged on its header) or not (decoded first), and skips every line Read
// skips.
func TestReadSelectedGivesWhatReadGivesThatKeepAccepts(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	if err := log.Append(
		Event{Time: t0, Run: "claude:a", Agent: "claude", Kind: RunStart, Project: "alpha", Data: RunStartData{Kind: KindSession, Worktree: "/src/alpha"}},
		Event{Time: t0.Add(time.Second), Run: "claude:a/x1", Agent: "claude", Kind: Activity, Project: "alpha", Data: ActivityData{Tools: 2, Paths: []string{`src/"quoted"\a.go`}}},
		Event{Time: t0.Add(2 * time.Second), Run: "claude:b", Agent: "claude", Kind: Claim, Project: "alpha", Data: TicketData{Ticket: "AL-1"}},
		Event{Time: t0.Add(3 * time.Second), Run: Human, Agent: Human, Kind: TicketMoved, Project: "alpha", Data: TicketData{Ticket: "AL-1", To: "review"}},
	); err != nil {
		t.Fatal(err)
	}
	day := FileName(t0)
	appendFile(t, filepath.Join(root, "alpha", ".flashheart", "events", day), strings.Join([]string{
		// Not in the envelope's order, or with escapes: decoded in full.
		`{"ts":"2026-10-05T14:13:00.000Z","v":1,"run":"claude:a","agent":"claude","kind":"turn.start","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-05T14:13:01.000Z","run":"claude:b","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-05T16:13:02+02:00","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":2,"ts":"2026-10-05T14:13:03.5Z","run":"claude:a","agent":"cl\"aude","kind":"compact","project":"alpha","data":{"phase":"pre"}}`,
		// Skipped by both.
		`{"v":0,"ts":"2026-10-05T14:13:04.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":01,"ts":"2026-10-05T14:13:04.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-02-31T14:13:05.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-05T24:13:05.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-05T14:13:06.000Z","run":"","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-05T14:13:07.000Z","run":"claude:a","agent":"claude","kind":"future.kind","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-05T14:13:08.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{"blocked`,
		`not an event`,
		``,
	}, "\n"))

	var all []Event
	if err := log.Read("alpha", t0, func(e Event) { all = append(all, e) }); err != nil {
		t.Fatal(err)
	}
	if len(all) != 8 {
		t.Fatalf("Read gave %d events", len(all))
	}
	for name, keep := range map[string]func(Header) bool{
		"every event":      func(Header) bool { return true },
		"one session":      func(h Header) bool { return SessionOf(h.Run) == "claude:a" },
		"claims and later": func(h Header) bool { return h.Kind == Claim || h.Time.After(t0.Add(time.Minute)) },
		"nothing":          func(Header) bool { return false },
	} {
		var want []Event
		for _, e := range all {
			if keep(HeaderOf(e)) {
				want = append(want, e)
			}
		}
		var got []Event
		if err := log.ReadSelected("alpha", t0, func(h Header) bool {
			if h.Run == "" || !h.Kind.Known() || h.Time.IsZero() {
				t.Errorf("%s: keep saw %+v", name, h)
			}
			return keep(h)
		}, func(e Event) { got = append(got, e) }); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: ReadSelected =\n%+v\nwant\n%+v", name, got, want)
		}
		headers, err := log.ReadHeaders("alpha", t0)
		if err != nil {
			t.Fatal(err)
		}
		got = nil
		headers.Select(keep, func(e Event) { got = append(got, e) })
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: ReadHeaders and Select =\n%+v\nwant\n%+v", name, got, want)
		}
		got = nil
		headers.SelectBackward(keep, func(e Event) { got = append(got, e) })
		slices.Reverse(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: SelectBackward reversed =\n%+v\nwant\n%+v", name, got, want)
		}
		var seen []Header
		headers.Each(func(h Header) { seen = append(seen, h) })
		if len(seen) != len(all) {
			t.Errorf("%s: Each saw %d headers, want %d", name, len(seen), len(all))
		}
	}
}

// ReadHeaders reads a file as it is now: a line still being written is
// read once it is whole, and a line too long to be an event is skipped, as
// by ReadFrom.
func TestReadHeadersReadsCompleteLines(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	if err := log.Append(Event{Time: t0, Run: "claude:a", Agent: "claude", Kind: TurnStart, Project: "alpha", Data: TurnStartData{}}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "alpha", ".flashheart", "events", FileName(t0))
	long := `{"v":1,"ts":"2026-10-05T14:12:10.000Z","run":"claude:a","agent":"claude","kind":"compact","project":"alpha","data":{"phase":"` + strings.Repeat("x", MaxLineBytes) + `"}}`
	appendFile(t, file, long+"\n"+`{"v":1,"ts":"2026-10-05T14:12:11.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}`)
	count := func() int {
		headers, err := log.ReadHeaders("alpha", t0)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		headers.Select(func(Header) bool { return true }, func(Event) { n++ })
		return n
	}
	if n := count(); n != 1 {
		t.Fatalf("events before the line is finished = %d, want 1", n)
	}
	appendFile(t, file, "}\n")
	if n := count(); n != 2 {
		t.Fatalf("events once it is finished = %d, want 2", n)
	}
}

func TestReadSelectedFailsAsReadDoes(t *testing.T) {
	t.Parallel()

	log, _ := newRoot(t, "alpha")
	for _, project := range []string{"alpha", "missing", "../escape"} {
		want := log.Read(project, t0, func(Event) {})
		got := log.ReadSelected(project, t0, func(Header) bool { return true }, func(Event) {})
		if (got == nil) != (want == nil) || errors.Is(got, fs.ErrNotExist) != errors.Is(want, fs.ErrNotExist) {
			t.Errorf("%s: ReadSelected = %v, Read = %v", project, got, want)
		}
	}
}

// The header of a line Append writes is read without decoding it.
func TestPeekReadsAppendsHeader(t *testing.T) {
	t.Parallel()

	e := Event{Time: t0, Run: "claude:a/x1", Agent: "claude", Kind: Activity, Project: "alpha", Data: ActivityData{Tools: 1}}
	line, err := e.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	h, ok := peek(line[:len(line)-1], map[string]string{})
	if !ok || h != HeaderOf(e) {
		t.Fatalf("peek = %+v, %v", h, ok)
	}
}

// BenchmarkReadSelected reads two days of a busy log (50,000 events a day)
// keeping nothing, as a hook's partial fold reads the lines it skips.
func BenchmarkReadSelected(b *testing.B) {
	root := b.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha"), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alpha", "project.yaml"), []byte("name: alpha\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	log := New(s)
	list := make([]Event, 0, 100_000)
	for i := range 100_000 {
		run := fmt.Sprintf("claude:seed-%d-%d", i%8, i/4800)
		list = append(list, Event{Time: t0.Add(time.Duration(i) * 1728 * time.Millisecond), Run: run, Agent: "claude", Kind: Activity, Project: "alpha", Data: ActivityData{Tools: 3, Paths: []string{"internal/seed/file.go"}, Shell: []Span{{From: t0, To: t0.Add(time.Second)}}}})
	}
	if err := log.Append(list...); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if err := log.ReadSelected("alpha", time.Time{}, func(h Header) bool { return h.Run == "claude:none" }, func(Event) {}); err != nil {
			b.Fatal(err)
		}
	}
}
