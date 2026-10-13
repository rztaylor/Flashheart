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
	// humans is each project's latest human board activity (agent-protocol
	// §3): from the files before the window, read once when the project is
	// first seen (seeded), and from every event read since. Unlike the
	// window, it outlives a new day.
	humans map[string]events.HumanActivity
	seeded map[string]bool
}

type projectEvents struct {
	events  []events.Event
	offsets map[string]int64
}

// boardKinds are the events recorded with a write to a ticket, review,
// attachment or activity file. A runs-only refold that reads one has the
// board read too: the file written may be in a folder the watcher does not
// watch (FH-96).
var boardKinds = map[events.Kind]bool{
	events.Claim: true, events.Release: true, events.Checkpoint: true,
	events.TicketMoved: true, events.TicketUpdated: true, events.TicketCreated: true,
	events.ReviewWritten: true, events.AttachmentAdded: true, events.QuestionAnswered: true,
	events.CommentAdded: true, events.CommentEdited: true, events.CommentDeleted: true,
	// A delivered comment sent from the board is marked delivered in its
	// ticket's activity file (FH-127).
	events.CommentDelivered: true,
}

// update reads new events for the given projects, folds them, and returns
// a fingerprint of what has been read and whether a new event is one of
// boardKinds.
func (t *tracker) update(projects []string, now time.Time) (string, bool) {
	refold := t.set == nil
	if day := events.FileName(now); day != t.day {
		// A new day: start again from the window, so old runs fall away.
		t.day, t.projects, refold = day, map[string]*projectEvents{}, true
	}
	// The event files of the run window, as every reader folds them.
	first := events.FileName(now.Add(-runs.Lookback))
	var parts []string
	for name := range t.projects {
		if !slices.Contains(projects, name) {
			delete(t.projects, name)
			refold = true
		}
	}
	if t.humans == nil {
		t.humans, t.seeded = map[string]events.HumanActivity{}, map[string]bool{}
	}
	for name := range t.seeded {
		if !slices.Contains(projects, name) {
			delete(t.seeded, name)
			delete(t.humans, name)
		}
	}
	var fresh []events.Event
	writes := false
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
		if !t.seeded[project] {
			t.seeded[project] = true
			t.seedHuman(project, files, first)
		}
		for _, file := range files {
			if file < first {
				continue
			}
			offset, _ := t.source.ReadFrom(project, file, state.offsets[file], func(e events.Event) {
				state.events = append(state.events, e)
				t.noteHuman(e)
				fresh = append(fresh, e)
				writes = writes || boardKinds[e.Kind]
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
	return strings.Join(parts, ","), writes
}

// noteHuman keeps e if it is the human's latest board activity in its
// project so far.
func (t *tracker) noteHuman(e events.Event) {
	if activity, ok := events.HumanActivityOf(e); ok && activity.Time.After(t.humans[e.Project].Time) {
		t.humans[e.Project] = activity
	}
}

// seedHuman reads a project's files dated before the window, newest first,
// up to the first with any human activity: the human may not have touched
// the board for longer than runs are kept.
func (t *tracker) seedHuman(project string, files []string, first string) {
	for index := len(files) - 1; index >= 0; index-- {
		if _, found := t.humans[project]; found {
			return
		}
		if files[index] < first {
			_, _ = t.source.ReadFrom(project, files[index], 0, t.noteHuman)
		}
	}
}

// human returns a copy of each project's latest human activity.
func (t *tracker) human() map[string]events.HumanActivity { return maps.Clone(t.humans) }

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

// runIndex finds a snapshot's runs without scanning them all: built once
// per snapshot, so a board of cards looks each one's runs up (FH-96).
// Positions are into Runs and keep its order.
type runIndex struct {
	byID map[string]int
	// byTicket lists the runs linked to each ticket.
	byTicket map[string][]int
	// family lists each run with its subagents.
	family map[string][]int
	// questions holds each ticket's open questions, oldest first, marked
	// SessionEnded as their run's view says.
	questions map[string][]runs.Question
	// question finds a question by id: run and question position.
	question map[string][2]int
}

func indexRuns(views []runs.View) runIndex {
	index := runIndex{
		byID: make(map[string]int, len(views)), byTicket: map[string][]int{}, family: make(map[string][]int, len(views)),
		questions: map[string][]runs.Question{}, question: map[string][2]int{},
	}
	for position, run := range views {
		if _, seen := index.byID[run.ID]; !seen {
			index.byID[run.ID] = position
		}
		if run.Link.Ticket != "" {
			index.byTicket[run.Link.Ticket] = append(index.byTicket[run.Link.Ticket], position)
		}
		index.family[run.ID] = append(index.family[run.ID], position)
		if run.Parent != "" {
			index.family[run.Parent] = append(index.family[run.Parent], position)
		}
		for n, q := range run.Questions {
			if _, seen := index.question[q.ID]; !seen {
				index.question[q.ID] = [2]int{position, n}
			}
			if q.Open() && q.Ticket != "" {
				index.questions[q.Ticket] = append(index.questions[q.Ticket], q)
			}
		}
	}
	for _, list := range index.questions {
		sort.SliceStable(list, func(a, b int) bool { return list[a].Asked.Before(list[b].Asked) })
	}
	return index
}

// linked returns the runs linked to a ticket, in Runs order.
func (s *Snapshot) linked(id string) []runs.View {
	positions := s.runIndex.byTicket[id]
	if id == "" || len(positions) == 0 {
		return nil
	}
	list := make([]runs.View, len(positions))
	for n, position := range positions {
		list[n] = s.Runs[position]
	}
	return list
}

// Run finds a run by id.
func (s *Snapshot) Run(id string) (runs.View, bool) {
	position, ok := s.runIndex.byID[id]
	if !ok {
		return runs.View{}, false
	}
	return s.Runs[position], true
}

// Family returns a run and its subagents, in the snapshot's order.
func (s *Snapshot) Family(id string) []runs.View {
	positions := s.runIndex.family[id]
	list := make([]runs.View, 0, len(positions))
	for _, position := range positions {
		list = append(list, s.Runs[position])
	}
	return list
}

// Subagents returns a run's subagents, in the snapshot's order.
func (s *Snapshot) Subagents(id string) []runs.View {
	var list []runs.View
	for _, run := range s.Family(id) {
		if run.Parent == id {
			list = append(list, run)
		}
	}
	return list
}

// TicketRun returns the live session run linked to a ticket that most needs
// attention (VIEW-8), preferring the most recently active.
func (s *Snapshot) TicketRun(id string) (runs.View, bool) {
	best, found := runs.View{}, false
	for _, run := range s.linked(id) {
		if run.State == runs.Ended || run.Parent != "" {
			continue
		}
		if !found || leads(run, best) {
			best, found = run, true
		}
	}
	return best, found
}

// TicketRuns returns every run linked to a ticket, most recent first.
func (s *Snapshot) TicketRuns(id string) []runs.View {
	list := s.linked(id)
	sort.SliceStable(list, func(a, b int) bool { return list[a].LastActivity.After(list[b].LastActivity) })
	return list
}

// Questions returns the open questions about a ticket (answered or not,
// until delivered), oldest first (CARD-6). Questions whose session has
// ended are included and marked SessionEnded (runs.View): they can still be
// answered, and the answer waits for the session to resume, but they do not
// need you (VIEW-2, RUN-8). A finished subagent's question waits on its
// session, so it needs you while the session is live (FH-112).
func (s *Snapshot) Questions(ticket string) []runs.Question {
	return slices.Clone(s.runIndex.questions[ticket])
}

// Question finds a question by id among the snapshot's runs.
func (s *Snapshot) Question(id string) (runs.Question, runs.View, bool) {
	position, ok := s.runIndex.question[id]
	if !ok {
		return runs.Question{}, runs.View{}, false
	}
	run := s.Runs[position[0]]
	return run.Questions[position[1]], run, true
}

// Holder returns the live run that holds a ticket's claim (agent-protocol
// §6): linked to it by a claim, not Ended, and active within the lease
// at the snapshot's time. A session holds a ticket its subagents work on
// under its claim; a subagent holds one it claimed itself. It is what a
// comment or change request is sent to now (FH-126, FH-127); a run linked
// by branch alone holds nothing.
func (s *Snapshot) Holder(ticket string) (runs.View, bool) {
	list := s.Claimants(ticket)
	if len(list) == 0 {
		return runs.View{}, false
	}
	return list[0], true
}

// Claimants returns the live runs linked to a ticket by a claim, each
// active within the lease, as Holder judges them: each session before its
// subagents, which work under its claim, the holder first. They are the
// runs a person may send a comment to now (FH-127).
func (s *Snapshot) Claimants(ticket string) []runs.View {
	var sessions, subagents []runs.View
	for _, run := range s.linked(ticket) {
		switch {
		case run.Link.By != runs.LinkClaim || run.State == runs.Ended || s.BuiltAt.Sub(run.LastActivity) >= s.lease:
		case run.Parent == "":
			sessions = append(sessions, run)
		default:
			subagents = append(subagents, run)
		}
	}
	list := make([]runs.View, 0, len(sessions)+len(subagents))
	placed := map[string]bool{}
	for _, session := range sessions {
		list = append(list, session)
		for _, run := range subagents {
			if run.Parent == session.ID {
				list, placed[run.ID] = append(list, run), true
			}
		}
	}
	// Subagents whose session does not hold the ticket claimed it
	// themselves.
	for _, run := range subagents {
		if !placed[run.ID] {
			list = append(list, run)
		}
	}
	return list
}
