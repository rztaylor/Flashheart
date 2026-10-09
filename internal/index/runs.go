package index

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/runs"
)

// EventSource reads projects' event logs incrementally (events.Log).
type EventSource interface {
	Files(project string) ([]string, error)
	ReadFrom(project, file string, offset int64, fn func(events.Event)) (int64, error)
}

// runWindowDays is how many days of event files the index folds: enough for
// any live run (stale after 12 hours) and a day of ended runs (VIEW-3).
const runWindowDays = 2

// tracker reads each project's recent event files, only lines appended
// since the last rebuild, and folds all of them into one set in time order:
// a run's events can sit in several projects' logs (a claim on another
// project's ticket), and it is one run. New events no older than the last
// one applied are applied as they arrive; an older one, a removed project or
// a new day refolds the window from the events kept. It is used under the
// index's lock.
type tracker struct {
	source   EventSource
	day      string
	projects map[string]*projectEvents
	set      *runs.Set
	// last is the time of the latest event applied to set.
	last time.Time
	// refolds counts full refolds, for tests.
	refolds int
}

type projectEvents struct {
	events  []events.Event
	offsets map[string]int64
}

// update reads new events for the given projects, folds them, and returns
// a fingerprint of what has been read.
func (t *tracker) update(projects []string, now time.Time) string {
	refold := t.set == nil
	if day := events.FileName(now); day != t.day {
		// A new day: start again from the window, so old runs fall away.
		t.day, t.projects, refold = day, map[string]*projectEvents{}, true
	}
	first := events.FileName(now.AddDate(0, 0, -runWindowDays))
	var parts []string
	for name := range t.projects {
		if !slices.Contains(projects, name) {
			delete(t.projects, name)
			refold = true
		}
	}
	var fresh []events.Event
	for _, project := range slices.Sorted(slices.Values(projects)) {
		files, err := t.source.Files(project)
		if err != nil || len(files) == 0 {
			continue
		}
		state := t.projects[project]
		if state == nil {
			state = &projectEvents{offsets: map[string]int64{}}
			t.projects[project] = state
		}
		for _, file := range files {
			if file < first {
				continue
			}
			offset, _ := t.source.ReadFrom(project, file, state.offsets[file], func(e events.Event) {
				state.events = append(state.events, e)
				fresh = append(fresh, e)
			})
			state.offsets[file] = offset
			parts = append(parts, fmt.Sprintf("%s/%s:%d", project, file, offset))
		}
	}
	slices.SortStableFunc(fresh, func(a, b events.Event) int { return a.Time.Compare(b.Time) })
	if len(fresh) > 0 && fresh[0].Time.Before(t.last) {
		refold = true
	}
	if refold {
		t.refolds++
		var all []events.Event
		for _, name := range slices.Sorted(maps.Keys(t.projects)) {
			all = append(all, t.projects[name].events...)
		}
		slices.SortStableFunc(all, func(a, b events.Event) int { return a.Time.Compare(b.Time) })
		t.set, t.last = runs.NewSet(), time.Time{}
		fresh = all
	}
	for _, e := range fresh {
		t.set.Apply(e)
		if e.Time.After(t.last) {
			t.last = e.Time
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// views derives every tracked run at now, linking runs by branch to the
// board's in-progress tickets (RUN-5).
func (t *tracker) views(b board.Board, now time.Time, settings runs.Settings) []runs.View {
	projects := make(map[string]*board.Project, len(b.Projects))
	for i := range b.Projects {
		projects[b.Projects[i].Name] = &b.Projects[i]
	}
	inProgress := func(project, branch string) []string {
		if p := projects[project]; p != nil {
			return p.InProgressOnBranch(branch)
		}
		return nil
	}
	if t.set == nil {
		return nil
	}
	return t.set.Views(now, settings, inProgress)
}

// signature summarises what the clock can change about runs, so a run
// turning Quiet or Ended moves the revision without any file changing.
func signature(views []runs.View) string {
	hash := sha256.New()
	for _, v := range views {
		fmt.Fprintf(hash, "%s|%s|%s|%t\n", v.ID, v.State, v.Link.Ticket, v.NoHandoff)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// RunCounts counts runs by state; Live is every run not Ended.
// NeedsYouUnticketed counts the runs in Needs you that no card on the
// board shows: neither their linked ticket nor a ticket one of their open
// questions is about is on it (none, or archived or gone). The board's
// Needs you filter cannot show them (FH-44).
type RunCounts struct {
	Working, NeedsYou, Waiting, Quiet, Ended, Live int
	NeedsYouUnticketed                             int
}

// RunCounts counts a project's runs, or every run when project is "".
func (s *Snapshot) RunCounts(project string) RunCounts {
	var counts RunCounts
	for _, run := range s.Runs {
		if project != "" && run.Project != project {
			continue
		}
		switch run.State {
		case runs.Working:
			counts.Working++
		case runs.NeedsYou:
			counts.NeedsYou++
			if !s.shownOnBoard(run) {
				counts.NeedsYouUnticketed++
			}
		case runs.Waiting:
			counts.Waiting++
		case runs.Quiet:
			counts.Quiet++
		case runs.Ended:
			counts.Ended++
		}
		if run.State != runs.Ended {
			counts.Live++
		}
	}
	return counts
}

// shownOnBoard reports whether a card on the board shows the run: its
// linked ticket's, or that of a ticket one of its open questions is about.
func (s *Snapshot) shownOnBoard(run runs.View) bool {
	if s.onBoard(run.Link.Ticket) {
		return true
	}
	for _, question := range run.Questions {
		if question.Open() && s.onBoard(question.Ticket) {
			return true
		}
	}
	return false
}

// onBoard reports whether id names a ticket the board shows.
func (s *Snapshot) onBoard(id string) bool {
	if id == "" || s.Archived(id) {
		return false
	}
	_, _, ok := s.FindTicket(id)
	return ok
}

// statePriority orders live states for a ticket's badge: the one that most
// needs attention first.
var statePriority = map[runs.State]int{runs.NeedsYou: 0, runs.Working: 1, runs.Quiet: 2, runs.Waiting: 3}

// TicketRun returns the live session run linked to a ticket that most needs
// attention (VIEW-8), preferring the most recently active.
func (s *Snapshot) TicketRun(id string) (runs.View, bool) {
	best := -1
	for i, run := range s.Runs {
		if run.Link.Ticket != id || run.State == runs.Ended || run.Parent != "" {
			continue
		}
		if best < 0 || leads(run, s.Runs[best]) {
			best = i
		}
	}
	if best < 0 {
		return runs.View{}, false
	}
	return s.Runs[best], true
}

// TicketRuns returns every run linked to a ticket, most recent first.
func (s *Snapshot) TicketRuns(id string) []runs.View {
	var list []runs.View
	for _, run := range s.Runs {
		if run.Link.Ticket == id {
			list = append(list, run)
		}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].LastActivity.After(list[b].LastActivity) })
	return list
}

// Questions returns the open questions about a ticket (answered or not,
// until delivered), oldest first (CARD-6). Questions of a run that has
// ended are included and marked SessionEnded: they can still be answered,
// and the answer waits for the session to resume, but they do not need you
// (VIEW-2, RUN-8).
func (s *Snapshot) Questions(ticket string) []runs.Question {
	var list []runs.Question
	for _, run := range s.Runs {
		for _, q := range run.Questions {
			if q.Ticket == ticket && q.Open() {
				q.SessionEnded = run.State == runs.Ended
				list = append(list, q)
			}
		}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].Asked.Before(list[b].Asked) })
	return list
}

// Question finds a question by id among the snapshot's runs.
func (s *Snapshot) Question(id string) (runs.Question, runs.View, bool) {
	for _, run := range s.Runs {
		for _, q := range run.Questions {
			if q.ID == id {
				return q, run, true
			}
		}
	}
	return runs.Question{}, runs.View{}, false
}
