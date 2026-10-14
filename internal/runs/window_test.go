package runs

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
)

var (
	randomSessions  = []string{"claude:s0", "claude:s1", "claude:s2", "codex:s3", "claude:s4"}
	randomWorktrees = []string{"/src/demo", "/src/demo-wt", ""}
	randomTickets   = []string{"DM-1", "DM-2", "DM-3"}
)

// randomLog is a mixed log of n events: sessions and their subagents in
// two worktrees doing everything the protocol records, claims taken from
// one another, subagents claiming under their session, questions asked,
// answered and delivered, and times now and then out of order.
func randomLog(rng *rand.Rand, n int) []events.Event {
	pick := func(list []string) string { return list[rng.IntN(len(list))] }
	var asked []string
	list := make([]events.Event, 0, n)
	at := t0
	for i := range n {
		at = at.Add(time.Duration(rng.IntN(90)) * time.Second)
		when := at
		if rng.IntN(10) == 0 {
			when = at.Add(-time.Duration(rng.IntN(300)) * time.Second)
		}
		session := pick(randomSessions)
		run := session
		switch rng.IntN(10) {
		case 0, 1, 2:
			run = fmt.Sprintf("%s/a%d", session, rng.IntN(3))
		case 3:
			if rng.IntN(5) == 0 {
				run = events.Human
			}
		}
		subagent := events.RunOf(run).IsSubagent()
		worktree, ticket := pick(randomWorktrees), pick(randomTickets)
		var kind events.Kind
		var data any
		switch rng.IntN(20) {
		case 0:
			kind, data = events.RunStart, events.RunStartData{Kind: events.KindSession, Worktree: worktree, Cwd: worktree, Branch: pick([]string{"b1", "b2"}), Source: pick([]string{"startup", "resume", events.SourceMCP})}
			if subagent {
				data = events.RunStartData{Kind: events.KindSubagent, Parent: session, AgentType: "Explore"}
			}
		case 1:
			kind, data = events.RunEnd, events.RunEndData{Reason: "other", WorktreeChanged: rng.IntN(3) == 0}
		case 2:
			kind, data = events.TurnStart, events.TurnStartData{Worktree: worktree, Branch: pick([]string{"b1", "b2", ""}), Background: rng.IntN(4) == 0}
		case 3:
			kind, data = events.TurnEnd, events.TurnEndData{BlockedForHandoff: rng.IntN(4) == 0, WorktreeChanged: rng.IntN(4) == 0}
		case 4:
			kind, data = events.ToolUsed, events.ToolData{Tool: "Bash", OK: rng.IntN(5) > 0, Shell: rng.IntN(2) == 0, Paths: []string{pick([]string{"a.go", "b.go"})}}
		case 5, 6, 7:
			activity := events.ActivityData{Tools: rng.IntN(4), Failed: rng.IntN(2)}
			if rng.IntN(2) == 0 {
				activity.Paths = []string{pick([]string{"a.go", "b.go", "c.go"})}
			}
			if rng.IntN(2) == 0 {
				activity.Shell = []events.Span{{From: when.Add(-30 * time.Second), To: when}}
			}
			kind, data = events.Activity, activity
		case 8:
			kind, data = events.Compact, events.CompactData{Phase: pick([]string{"pre", "post"})}
		case 9:
			kind, data = events.PermissionRequested, events.PermissionData{Tool: pick([]string{"Bash", ""})}
		case 10:
			kind, data = events.PermissionResolved, events.ResolvedData{Outcome: "allow"}
		case 11:
			kind, data = events.Notification, events.NotificationData{Type: "idle_prompt"}
		case 12, 13:
			kind, data = events.Claim, events.TicketData{Ticket: ticket, Home: "demo"}
		case 14:
			kind, data = events.Release, events.TicketData{Ticket: ticket}
		case 15:
			kind, data = events.Checkpoint, events.CheckpointData{Ticket: ticket}
		case 16:
			kind = events.Kind(pick([]string{string(events.TicketMoved), string(events.TicketUpdated), string(events.ReviewWritten)}))
			data = events.TicketData{Ticket: ticket, To: "review"}
		case 17:
			id := fmt.Sprintf("q-%d", i)
			asked = append(asked, id)
			kind, data = events.QuestionAsked, events.QuestionData{ID: id, Ticket: ticket, Kind: events.QuestionKindDecision, Text: "Which?"}
		case 18, 19:
			if len(asked) == 0 {
				kind, data = events.Compact, events.CompactData{Phase: "pre"}
				break
			}
			// Mostly on the asking run, as the board records it.
			kind, data = events.QuestionAnswered, events.AnswerData{ID: pick(asked), Answer: "Yes", By: "Robert"}
			if rng.IntN(2) == 0 {
				kind, data = events.QuestionDelivered, events.DeliveredData{ID: pick(asked)}
			}
		}
		list = append(list, events.Event{Time: when, Run: run, Agent: events.RunOf(run).Agent(), Kind: kind, Project: "demo", Data: data})
	}
	return list
}

func fold(list []events.Event, keep func(events.Header) bool) *Set {
	set := NewSet()
	for _, e := range list {
		if keep == nil || keep(events.HeaderOf(e)) {
			set.Apply(e)
		}
	}
	return set
}

// What a hook asks of a fold that folded only Keep's events comes out as a
// full fold's for the kept sessions and their subagents: everything a
// view shows (state, link, edits, files, questions, timeline), shell
// windows, undelivered answers, the turn's handoff block and the claim.
func TestKeepFoldsTheSessionsAsTheFullFoldDoes(t *testing.T) {
	t.Parallel()

	byBranch := func(_, branch string) []string {
		if branch == "b1" {
			return []string{"DM-1"}
		}
		return nil
	}
	settings := DefaultSettings()
	for seed := range 200 {
		list := randomLog(rand.New(rand.NewPCG(uint64(seed), 7)), 400)
		now := list[len(list)-1].Time.Add(time.Duration(seed%40) * time.Minute)
		full := fold(list, nil)
		fullViews := full.Views(now, settings, byBranch)
		for _, sessions := range [][]string{{randomSessions[seed%5]}, {randomSessions[seed%5], randomSessions[(seed+2)%5]}} {
			partial := fold(list, Keep(sessions...))
			views := partial.Views(now, settings, byBranch)
			for _, want := range fullViews {
				if !slices.Contains(sessions, events.SessionOf(want.ID)) {
					continue
				}
				i := slices.IndexFunc(views, func(v View) bool { return v.ID == want.ID })
				if i < 0 {
					t.Fatalf("seed %d, %v: run %s missing from the partial fold", seed, sessions, want.ID)
				}
				if !reflect.DeepEqual(views[i], want) {
					t.Fatalf("seed %d, %v: run %s\n got %+v\nwant %+v", seed, sessions, want.ID, views[i], want)
				}
				got, run := partial.Get(want.ID), full.Get(want.ID)
				if got.Claim != run.Claim || got.Home != run.Home || got.BlockedForHandoff != run.BlockedForHandoff || !got.LastCheckpoint.Equal(run.LastCheckpoint) {
					t.Fatalf("seed %d, %v: run %s claim %q/%q home %q/%q blocked %v/%v", seed, sessions, want.ID, got.Claim, run.Claim, got.Home, run.Home, got.BlockedForHandoff, run.BlockedForHandoff)
				}
				if !reflect.DeepEqual(partial.ShellWindows(want.ID), full.ShellWindows(want.ID)) || !reflect.DeepEqual(partial.PendingAnswers(want.ID), full.PendingAnswers(want.ID)) {
					t.Fatalf("seed %d, %v: run %s shell windows or pending answers differ", seed, sessions, want.ID)
				}
			}
		}
	}
}

// previousIn is the recovery note's previous run as a full fold finds it:
// the session other than except in worktree that was active last.
func previousIn(set *Set, worktree, except string) string {
	var previous *Run
	for _, r := range set.Runs() {
		if r.ID == except || r.Kind != events.KindSession || r.Worktree == "" || r.Worktree != worktree {
			continue
		}
		if previous == nil || r.LastActivity.After(previous.LastActivity) {
			previous = r
		}
	}
	if previous == nil {
		return ""
	}
	return previous.ID
}

func TestLatestFindsThePreviousSessionAsTheFullFoldDoes(t *testing.T) {
	t.Parallel()

	for seed := range 300 {
		list := randomLog(rand.New(rand.NewPCG(uint64(seed), 11)), 1+seed%250)
		full := fold(list, nil)
		latest := NewLatest()
		for _, e := range list {
			latest.Note(events.HeaderOf(e))
		}
		for _, e := range slices.Backward(list) {
			if latest.Wants(events.HeaderOf(e)) {
				latest.Apply(e)
			}
		}
		for _, worktree := range randomWorktrees {
			for _, except := range append([]string{""}, randomSessions...) {
				if got, want := latest.Session(worktree, except), previousIn(full, worktree, except); got != want {
					t.Fatalf("seed %d: previous in %q but %q = %q, want %q", seed, worktree, except, got, want)
				}
			}
		}
	}
}

// Every reader of runs folds the one run window (FH-95).
func TestEveryReaderFoldsTheRunWindow(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"hooks", "index", "mcpserver", "await"} {
		files, err := filepath.Glob(filepath.Join("..", dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		uses := false
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			uses = uses || strings.Contains(string(data), "-runs.Lookback)")
		}
		if !uses {
			t.Errorf("internal/%s does not fold runs.Lookback", dir)
		}
	}
}
