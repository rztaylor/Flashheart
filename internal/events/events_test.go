package events

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/store"
)

var t0 = time.Date(2026, 10, 5, 14, 12, 9, 123_000_000, time.UTC)

func newRoot(t *testing.T, projects ...string) (*Log, string) {
	t.Helper()
	root := t.TempDir()
	for _, project := range projects {
		if err := os.MkdirAll(filepath.Join(root, project), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, project, "project.yaml"), []byte("name: "+project+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return New(s), root
}

func TestAppendWritesTheEnvelopeToTheDailyFile(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	event := Event{Time: t0, Run: "claude:abc", Agent: "claude", Kind: ToolUsed, Project: "alpha", Data: ToolData{Tool: "Edit", OK: true, Path: "src/a.ts"}}
	if err := log.Append(event); err != nil {
		t.Fatal(err)
	}
	// An event just after midnight UTC goes to the next day's file.
	late := event
	late.Time = time.Date(2026, 10, 6, 0, 0, 1, 0, time.UTC)
	late.Data = TurnEndData{}
	late.Kind = TurnEnd
	if err := log.Append(late); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "alpha", ".flashheart", "events", "2026-10-05.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"v":1,"ts":"2026-10-05T14:12:09.123Z","run":"claude:abc","agent":"claude","kind":"tool.used","project":"alpha","data":{"tool":"Edit","ok":true,"path":"src/a.ts"}}` + "\n"
	if string(data) != want {
		t.Fatalf("line =\n%s\nwant\n%s", data, want)
	}
	next, _ := os.ReadFile(filepath.Join(root, "alpha", ".flashheart", "events", "2026-10-06.jsonl"))
	if !strings.Contains(string(next), `"kind":"turn.end","project":"alpha","data":{"blocked_for_handoff":false}`) {
		t.Fatalf("next day = %s", next)
	}
}

func TestReadIsTolerant(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	dir := filepath.Join(root, "alpha", ".flashheart", "events")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join([]string{
		`{"v":1,"ts":"2026-10-04T13:00:00.000Z","run":"claude:a","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"session","cwd":"/src/alpha","branch":"main"}}`,
		`this line is malformed`,
		`{"v":1,"ts":"2026-10-04T13:00:01.000Z","run":"claude:a","agent":"claude","kind":"future.kind","project":"alpha","data":{}}`,
		`{"v":1,"ts":"not a time","run":"claude:a","agent":"claude","kind":"turn.start","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-04T13:00:02.000Z","run":"","agent":"claude","kind":"turn.start","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-04T13:00:03.000Z","run":"claude:a","agent":"claude","kind":"turn.start","project":"alpha","data":{}}`,
		`{"v":1,"ts":"2026-10-04T13:00:04.000Z","run":"claude:a","agent":"claude","kind":"tool.used","project":"alpha","data":{"tool":"Ed`,
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "2026-10-04.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	var got []Event
	if err := log.Read("alpha", time.Time{}, func(e Event) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != RunStart || got[1].Kind != TurnStart {
		t.Fatalf("Read = %+v", got)
	}
	var start RunStartData
	if err := got[0].Decode(&start); err != nil || start.Branch != "main" || start.Kind != "session" {
		t.Fatalf("Decode = %+v, %v", start, err)
	}
	if !got[1].Time.Equal(time.Date(2026, 10, 4, 13, 0, 3, 0, time.UTC)) {
		t.Fatalf("time = %v", got[1].Time)
	}

	// The unfinished last line is left for the writer to complete: an
	// incremental read stops before it, and finishes it once it is whole.
	file := "2026-10-04.jsonl"
	var tail []Event
	offset, err := log.ReadFrom("alpha", file, 0, func(e Event) { tail = append(tail, e) })
	if err != nil || len(tail) != 2 {
		t.Fatalf("ReadFrom = %d events, %v", len(tail), err)
	}
	if offset != int64(strings.LastIndex(lines, "\n")+1) {
		t.Fatalf("offset = %d", offset)
	}
	appendFile(t, filepath.Join(dir, file), `it","ok":true}}`+"\n")
	tail = nil
	if _, err := log.ReadFrom("alpha", file, offset, func(e Event) { tail = append(tail, e) }); err != nil || len(tail) != 1 || tail[0].Kind != ToolUsed {
		t.Fatalf("second ReadFrom = %+v, %v", tail, err)
	}
}

func appendFile(t *testing.T, name, text string) {
	t.Helper()
	f, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func TestReadSinceSkipsOlderFiles(t *testing.T) {
	t.Parallel()

	log, _ := newRoot(t, "alpha")
	for day := 1; day <= 3; day++ {
		e := Event{Time: time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC), Run: "claude:a", Agent: "claude", Kind: TurnStart, Project: "alpha", Data: TurnStartData{}}
		if err := log.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	var days []int
	since := time.Date(2026, 10, 2, 18, 0, 0, 0, time.UTC)
	if err := log.Read("alpha", since, func(e Event) { days = append(days, e.Time.Day()) }); err != nil {
		t.Fatal(err)
	}
	// Whole files from since's date are read; the caller filters by time.
	if !slices.Equal(days, []int{2, 3}) {
		t.Fatalf("days = %v", days)
	}
}

func TestPruneRemovesOnlyExpiredEventFiles(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	dir := filepath.Join(root, "alpha", ".flashheart", "events")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"2026-07-06.jsonl", "2026-07-07.jsonl", "2026-10-05.jsonl", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	removed, err := log.Prune("alpha", now, 90)
	if err != nil || removed != 1 {
		t.Fatalf("Prune = %d, %v", removed, err)
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"2026-07-07.jsonl", "2026-10-05.jsonl", "notes.txt"}) {
		t.Fatalf("left %v", names)
	}
}

// A session inbox goes with the event file of the day it was last written
// (FH-82): by then no reader of the log can deliver its answers, and no
// prompt of its session has come since, or it would have taken them.
func TestPruneRemovesInboxesWithTheirDaysEvents(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	// The inbox and the old answers/ location alike (D54).
	dirs := []string{filepath.Join(root, "alpha", ".flashheart", "inbox"), filepath.Join(root, "alpha", ".flashheart", "answers")}
	inboxes := map[string]time.Time{
		"claude--expired.jsonl": time.Date(2026, 7, 6, 23, 59, 0, 0, time.UTC),
		"claude--kept.jsonl":    time.Date(2026, 7, 7, 0, 1, 0, 0, time.UTC),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, at := range inboxes {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(filepath.Join(dir, name), at, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	if removed, err := log.Prune("alpha", now, 90); err != nil || removed != 2 {
		t.Fatalf("Prune = %d, %v", removed, err)
	}
	for _, dir := range dirs {
		entries, _ := os.ReadDir(dir)
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		if !slices.Equal(names, []string{"claude--kept.jsonl"}) {
			t.Fatalf("%s: left %v", dir, names)
		}
	}
}

func TestEncodeBoundsLines(t *testing.T) {
	t.Parallel()

	// The largest line is an activity record with every path and window.
	paths := make([]string, 0, MaxToolPaths)
	for i := range MaxToolPaths {
		paths = append(paths, strconv.Itoa(i)+strings.Repeat("x", maxPaths))
	}
	spans := make([]Span, MaxActivitySpans)
	for i := range spans {
		spans[i] = Span{From: t0, To: t0.Add(time.Second)}
	}
	line, err := Event{Time: t0, Run: "claude:a", Agent: "claude", Kind: Activity, Project: "alpha", Data: ActivityData{Tools: 1 << 20, Failed: 1 << 20, Paths: paths, Shell: spans}}.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	if len(line) > MaxLineBytes {
		t.Fatalf("largest activity line is %d bytes; MaxLineBytes is %d", len(line), MaxLineBytes)
	}
	var check map[string]any
	if err := json.Unmarshal(line, &check); err != nil {
		t.Fatal(err)
	}
}

// Logs from before D51 hold plan.updated, which hooks no longer write
// (FH-133): readers skip it like any unknown kind, and every other line of
// the log still reads; a writer is refused it.
func TestOldPlanEventsReadAsSkipped(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	dir := filepath.Join(root, "alpha", ".flashheart", "events")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join([]string{
		`{"v":1,"ts":"2026-10-04T13:00:00.000Z","run":"claude:a","agent":"claude","kind":"run.start","project":"alpha","data":{"kind":"session","cwd":"/src/alpha","branch":"main"}}`,
		`{"v":1,"ts":"2026-10-04T13:00:01.000Z","run":"claude:a","agent":"claude","kind":"plan.updated","project":"alpha","data":{"items":[{"text":"Read the code","status":"completed"},{"text":"Fix it","status":"in_progress"}]}}`,
		`{"v":1,"ts":"2026-10-04T13:00:02.000Z","run":"claude:a","agent":"claude","kind":"plan.updated","project":"alpha","data":{"merge":true,"items":[{"id":"1","status":"deleted"}]}}`,
		`{"v":1,"ts":"2026-10-04T13:00:03.000Z","run":"claude:a","agent":"claude","kind":"turn.end","project":"alpha","data":{}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-10-04.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	var kinds []Kind
	if err := log.Read("alpha", time.Time{}, func(e Event) { kinds = append(kinds, e.Kind) }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kinds, []Kind{RunStart, TurnEnd}) {
		t.Fatalf("Read = %v, want run.start and turn.end with the plans skipped", kinds)
	}
	if Kind("plan.updated").Known() {
		t.Fatal("plan.updated is still a kind the log records")
	}
}

// TestHelperProcess appends events from a child process for the concurrency
// test.
func TestHelperProcess(t *testing.T) {
	root := os.Getenv("FLASHHEART_EVENTS_HELPER_ROOT")
	if root == "" {
		t.Skip("helper process only")
	}
	s, err := store.Open(root)
	if err != nil {
		os.Exit(2)
	}
	log := New(s)
	for i := range 100 {
		e := Event{Time: t0, Run: "claude:" + os.Getenv("FLASHHEART_EVENTS_HELPER_RUN"), Agent: "claude", Kind: ToolUsed, Project: "alpha", Data: ToolData{Tool: "Edit" + strconv.Itoa(i), OK: true, Path: strings.Repeat("p", 2000)}}
		if err := log.Append(e); err != nil {
			os.Exit(3)
		}
	}
	os.Exit(0)
}

func TestConcurrentAppendsFromProcessesKeepWholeLines(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	var wg sync.WaitGroup
	for writer := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
			cmd.Env = append(os.Environ(), "FLASHHEART_EVENTS_HELPER_ROOT="+root, "FLASHHEART_EVENTS_HELPER_RUN="+strconv.Itoa(writer))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("writer %d: %v\n%s", writer, err, out)
			}
		}()
	}
	wg.Wait()
	count := 0
	if err := log.Read("alpha", time.Time{}, func(Event) { count++ }); err != nil {
		t.Fatal(err)
	}
	if count != 400 {
		t.Fatalf("read %d whole events, want 400", count)
	}
}

func TestAnInboxDeliversEachAnswerOnce(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "alpha", "tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	log := New(s)
	answer := Entry{Type: EntryAnswer, ID: "q-1", Run: "claude:s/a1", Question: "Q?", Text: "A", By: "human"}
	for range 2 {
		if err := log.Queue("alpha", answer); err != nil {
			t.Fatal(err)
		}
	}
	got, err := log.Take("alpha", Mailbox{Session: "claude:s"})
	if err != nil || len(got) != 1 || got[0] != answer {
		t.Fatalf("answers = %+v, %v", got, err)
	}
}

// Deliver hands over what it took even when an inbox cannot be read or
// delivery cannot be recorded (FH-78): an answer is never lost.
func TestDeliverReturnsWhatItTookWhenRecordingFails(t *testing.T) {
	t.Parallel()

	log, root := newRoot(t, "alpha")
	mine := Entry{Type: EntryAnswer, ID: "q-1", Run: "claude:s/a1", Question: "Q1?", Text: "A1", By: "human"}
	theirs := Entry{Type: EntryComment, ID: "c-2", Run: "claude:p", Text: "C2", By: "human"}
	for _, d := range []Entry{mine, theirs} {
		if err := log.Queue("alpha", d); err != nil {
			t.Fatal(err)
		}
	}
	// An unreadable inbox (a directory) and an event file that cannot be
	// appended to (a directory too).
	if err := os.MkdirAll(filepath.Join(root, "alpha", ".flashheart", "inbox", "claude--broken.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "alpha", ".flashheart", "events", FileName(t0)), 0o755); err != nil {
		t.Fatal(err)
	}
	pending := []Entry{mine, {Type: EntryAnswer, ID: "q-3", Run: "claude:s", Question: "Q3?", Text: "A3", By: "human"}}
	boxes := []Mailbox{{Session: "claude:s"}, {Session: "claude:broken"}, {Session: "claude:p"}, {}}
	got, err := Deliver(log, "alpha", boxes, pending, t0)
	if err == nil {
		t.Fatal("Deliver reported no error")
	}
	var ids []string
	for _, d := range got {
		ids = append(ids, d.ID)
	}
	if !slices.Equal(ids, []string{"q-1", "c-2", "q-3"}) {
		t.Fatalf("delivered %v", ids)
	}
	// The inboxes it read are emptied: the answers are the caller's now.
	if again, err := log.Take("alpha", Mailbox{Session: "claude:s"}); err != nil || len(again) != 0 {
		t.Fatalf("inbox after delivery = %+v, %v", again, err)
	}
}

func TestDeliverRecordsEachAnswerDelivered(t *testing.T) {
	t.Parallel()

	log, _ := newRoot(t, "alpha")
	d := Entry{Type: EntryAnswer, ID: "q-1", Run: "claude:s/a1", Question: "Q?", Text: "A", By: "human"}
	if err := log.Queue("alpha", d); err != nil {
		t.Fatal(err)
	}
	if got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, []Entry{d}, t0); err != nil || len(got) != 1 {
		t.Fatalf("Deliver = %+v, %v", got, err)
	}
	var delivered []string
	if err := log.Read("alpha", t0, func(e Event) {
		if e.Kind == QuestionDelivered {
			delivered = append(delivered, e.Run)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(delivered, []string{"claude:s/a1"}) {
		t.Fatalf("delivered on %v", delivered)
	}
	if got, err := Deliver(log, "alpha", []Mailbox{{Session: "claude:s"}}, nil, t0); err != nil || len(got) != 0 {
		t.Fatalf("second Deliver = %+v, %v", got, err)
	}
}

// Only the user's prompt is from the user; a turn without an origin, from
// a log older than the field, is the user's unless marked background
// (FH-57, agent-protocol §3).
func TestTurnStartFromUser(t *testing.T) {
	t.Parallel()

	cases := []struct {
		data TurnStartData
		want bool
	}{
		{TurnStartData{Origin: OriginUser}, true},
		{TurnStartData{}, true},
		{TurnStartData{Background: true}, false},
		{TurnStartData{Origin: OriginTask, Background: true}, false},
		{TurnStartData{Origin: OriginAgent}, false},
		{TurnStartData{Origin: "scheduler"}, false},
	}
	for _, tc := range cases {
		if got := tc.data.FromUser(); got != tc.want {
			t.Errorf("%+v.FromUser() = %v, want %v", tc.data, got, tc.want)
		}
	}
	long := TurnStartData{Origin: strings.Repeat("o", 100)}.bound().(TurnStartData)
	if len([]rune(long.Origin)) > maxWord {
		t.Fatalf("origin of %d runes kept", len([]rune(long.Origin)))
	}
}
