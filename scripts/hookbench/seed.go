package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
	"github.com/rztaylor/flashheart/internal/store"
)

// seedSpan is how far back the log is seeded: the run window.
const seedSpan = runs.Lookback

// The seeded board: tickets, and sessions of seedLength events each,
// seedSlots of them at a time, the last two slots in worktrees of their
// own.
const (
	seedTickets = 40
	seedSlots   = 8
	seedLength  = 600
)

// setUp creates the bench's project with its tickets, turns on handoff
// enforcement and gives the enforced session (when there is one) a claim
// and an edit, and seeds the log with perDay events a day; it returns how
// many events it wrote.
func setUp(root, repo string, o options, session string, now time.Time) (int, error) {
	s := store.New(root)
	defer s.Close()
	if err := s.EnsureRoot(); err != nil {
		return 0, err
	}
	project, err := s.ProjectFor(filepath.Base(repo), repo, true)
	if err != nil {
		return 0, err
	}
	tickets := make([]string, 0, seedTickets)
	columns := []board.Column{board.Backlog, board.UpNext, board.InProgress, board.Review, board.Done}
	for i := range seedTickets {
		created, err := s.CreateTicket(project, store.NewTicket{Title: fmt.Sprintf("Bench ticket %d", i+1), Status: columns[i%len(columns)], Description: "Seeded by hookbench."})
		if err != nil {
			return 0, err
		}
		tickets = append(tickets, created.ID)
	}
	var list []events.Event
	if o.seedEvents > 0 {
		list = seedEvents(o.agent, project, repo, tickets, o.seedEvents, now)
	}
	if session != "" {
		if err := s.UpdateConfig(func([]byte) ([]byte, error) { return []byte("enforce_handoff: true\n"), nil }); err != nil {
			return 0, err
		}
		list = append(list, enforced(o.agent+":"+session, project, repo, tickets[2], now)...)
	}
	slices.SortStableFunc(list, func(a, b events.Event) int { return a.Time.Compare(b.Time) })
	log := events.New(s)
	for start := 0; start < len(list); start += 5000 {
		if err := log.Append(list[start:min(start+5000, len(list))]...); err != nil {
			return 0, err
		}
	}
	return len(list), nil
}

// enforced is the stop scenario's session: started an hour ago in the
// bench's worktree, holding an in-progress ticket, with an edit since.
func enforced(run, project, repo, ticket string, now time.Time) []events.Event {
	at := func(minutes int) time.Time { return now.Add(-time.Duration(minutes) * time.Minute) }
	e := func(t time.Time, kind events.Kind, data any) events.Event {
		return events.Event{Time: t, Run: run, Agent: events.RunOf(run).Agent(), Kind: kind, Project: project, Data: data}
	}
	return []events.Event{
		e(at(60), events.RunStart, events.RunStartData{Kind: events.KindSession, Cwd: repo, Branch: "feature/demo", Worktree: repo, Source: "startup"}),
		e(at(59), events.TurnStart, events.TurnStartData{Cwd: repo, Branch: "feature/demo", Worktree: repo}),
		e(at(58), events.Claim, events.TicketData{Ticket: ticket, Home: project}),
		e(at(50), events.Activity, events.ActivityData{Tools: 2, Paths: []string{"README.md"}}),
	}
}

// seedEvents is perDay events a day over seedSpan, ending a minute ago: a
// realistic mix of sessions (activity records, turns, plans, permissions,
// subagents, questions), a third of them claiming a ticket.
func seedEvents(agent, project, repo string, tickets []string, perDay int, now time.Time) []events.Event {
	total := int(int64(perDay) * int64(seedSpan) / int64(24*time.Hour))
	start := now.Add(-seedSpan + time.Minute)
	step := (seedSpan - 2*time.Minute) / time.Duration(total)
	list := make([]events.Event, 0, total)
	for i := range total {
		at := start.Add(time.Duration(i) * step)
		slot, n := i%seedSlots, i/seedSlots
		seq, k := n/seedLength, n%seedLength
		session := fmt.Sprintf("%s:seed-%d-%d", agent, slot, seq)
		worktree := repo
		if slot >= seedSlots-2 {
			worktree = fmt.Sprintf("%s-wt%d", repo, slot)
		}
		branch := fmt.Sprintf("feature/seed-%d-%d", slot, seq)
		ticket, claimed := tickets[(slot*7+seq)%len(tickets)], seq%3 == 0
		question := fmt.Sprintf("q-%d-%d-%d", slot, seq, k/200)
		run, kind, data := session, events.Activity, any(nil)
		switch {
		case k == 0:
			kind, data = events.RunStart, events.RunStartData{Kind: events.KindSession, Cwd: worktree, Branch: branch, Worktree: worktree, Source: "startup"}
		case k == seedLength-1:
			kind, data = events.RunEnd, events.RunEndData{Reason: "other"}
		case k == 2 && claimed:
			kind, data = events.Claim, events.TicketData{Ticket: ticket, Home: project}
		case k == seedLength-3 && claimed:
			kind, data = events.Release, events.TicketData{Ticket: ticket}
		case k%100 == 50 && claimed:
			kind, data = events.Checkpoint, events.CheckpointData{Ticket: ticket, Done: 2, Next: 1, Files: 3}
		case k%120 == 60 && claimed:
			kind, data = events.TicketUpdated, events.TicketData{Ticket: ticket, Fields: []string{"notes"}}
		case k%25 == 1:
			kind, data = events.TurnStart, events.TurnStartData{Cwd: worktree, Branch: branch, Worktree: worktree}
		case k%25 == 24:
			kind, data = events.TurnEnd, events.TurnEndData{}
		case k%30 == 5:
			kind, data = events.PlanUpdated, events.PlanData{Items: []events.PlanItem{{Text: "Read the code", Status: events.PlanCompleted}, {Text: "Write the test", Status: events.PlanInProgress}, {Text: "Make it pass", Status: events.PlanPending}}}
		case k%60 == 7:
			kind, data = events.PermissionRequested, events.PermissionData{Tool: "Bash"}
		case k%60 == 8:
			kind, data = events.PermissionResolved, events.ResolvedData{Outcome: "allow", Tool: "Bash"}
		case k%90 == 40:
			kind, data = events.Notification, events.NotificationData{Type: "idle_prompt"}
		case k%200 == 99:
			kind, data = events.QuestionAsked, events.QuestionData{ID: question, Kind: events.QuestionKindQuestion, Text: "Which way?"}
		case k%200 == 120:
			kind, data = events.QuestionAnswered, events.AnswerData{ID: question, Answer: "This way.", By: "bench"}
		case k%200 == 130:
			kind, data = events.QuestionDelivered, events.DeliveredData{ID: question}
		case k%150 >= 10 && k%150 <= 20:
			// A subagent: started, nine activity records, ended.
			run = fmt.Sprintf("%s/agent-%d", session, k/150)
			switch k % 150 {
			case 10:
				kind, data = events.RunStart, events.RunStartData{Kind: events.KindSubagent, Parent: session, AgentType: "Explore"}
			case 20:
				kind, data = events.RunEnd, events.RunEndData{Reason: "completed"}
			default:
				data = activity(k, at)
			}
		default:
			data = activity(k, at)
		}
		list = append(list, events.Event{Time: at, Run: run, Agent: agent, Kind: kind, Project: project, Data: data})
	}
	return list
}

// activity is an activity record: a few tools, now and then a failure, an
// edit or a shell command.
func activity(k int, at time.Time) events.ActivityData {
	data := events.ActivityData{Tools: 1 + k%5}
	if k%7 == 0 {
		data.Failed = 1
	}
	if k%3 == 0 {
		data.Paths = []string{fmt.Sprintf("internal/seed/file%d.go", k%40)}
	}
	if k%4 == 0 {
		data.Shell = []events.Span{{From: at.Add(-20 * time.Second), To: at.Add(-time.Second)}}
	}
	return data
}
