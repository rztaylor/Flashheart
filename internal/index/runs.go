package index

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// tracker folds each project's recent event files, reading only lines
// appended since the last rebuild. It is used under the index's lock.
type tracker struct {
	source   EventSource
	day      string
	projects map[string]*projectRuns
}

type projectRuns struct {
	set     *runs.Set
	offsets map[string]int64
}

// update folds new events for the given projects and returns a fingerprint
// of what has been read.
func (t *tracker) update(projects []string, now time.Time) string {
	if day := events.FileName(now); day != t.day {
		// A new day: start again from the window, so old runs fall away.
		t.day, t.projects = day, map[string]*projectRuns{}
	}
	first := events.FileName(now.AddDate(0, 0, -runWindowDays))
	var parts []string
	for name := range t.projects {
		if !slices.Contains(projects, name) {
			delete(t.projects, name)
		}
	}
	for _, project := range projects {
		files, err := t.source.Files(project)
		if err != nil || len(files) == 0 {
			continue
		}
		state := t.projects[project]
		if state == nil {
			state = &projectRuns{set: runs.NewSet(), offsets: map[string]int64{}}
			t.projects[project] = state
		}
		for _, file := range files {
			if file < first {
				continue
			}
			offset, _ := t.source.ReadFrom(project, file, state.offsets[file], state.set.Apply)
			state.offsets[file] = offset
			parts = append(parts, fmt.Sprintf("%s/%s:%d", project, file, offset))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// views derives every tracked run at now, linking runs by branch to the
// board's in-progress tickets (RUN-5).
func (t *tracker) views(b board.Board, now time.Time, settings runs.Settings) []runs.View {
	branches := map[string]map[string][]string{}
	for _, project := range b.Projects {
		byBranch := map[string][]string{}
		for _, ticket := range project.Tickets {
			if ticket.Column == board.InProgress && ticket.Branch != "" && !ticket.NeedsRepair() {
				byBranch[ticket.Branch] = append(byBranch[ticket.Branch], ticket.ID)
			}
		}
		branches[project.Name] = byBranch
	}
	inProgress := func(project, branch string) []string { return branches[project][branch] }
	names := make([]string, 0, len(t.projects))
	for name := range t.projects {
		names = append(names, name)
	}
	slices.Sort(names)
	var all []runs.View
	for _, name := range names {
		all = append(all, t.projects[name].set.Views(now, settings, inProgress)...)
	}
	return all
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
type RunCounts struct {
	Working, NeedsYou, Waiting, Quiet, Ended, Live int
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
		if best < 0 || statePriority[run.State] < statePriority[s.Runs[best].State] ||
			(run.State == s.Runs[best].State && run.LastActivity.After(s.Runs[best].LastActivity)) {
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
