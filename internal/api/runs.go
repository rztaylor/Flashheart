package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
)

// endedVisible is how long an Ended run stays in the Agents view by default
// (VIEW-3).
const endedVisible = 24 * time.Hour

// maxTicketRuns bounds the runs sent with a ticket's Runs tab.
const maxTicketRuns = 10

// RunCountsJSON counts runs by state; Live is every run not Ended.
type RunCountsJSON struct {
	Working  int `json:"working"`
	NeedsYou int `json:"needsYou"`
	Waiting  int `json:"waiting"`
	Quiet    int `json:"quiet"`
	Ended    int `json:"ended"`
	Live     int `json:"live"`
}

func countsJSON(c index.RunCounts) RunCountsJSON {
	return RunCountsJSON{Working: c.Working, NeedsYou: c.NeedsYou, Waiting: c.Waiting, Quiet: c.Quiet, Ended: c.Ended, Live: c.Live}
}

// LiveJSON is a card's live badge (VIEW-8): the linked run that most needs
// attention.
type LiveJSON struct {
	Run        string `json:"run"`
	Short      string `json:"short"`
	Agent      string `json:"agent"`
	State      string `json:"state"`
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	Step       string `json:"step"`
	Permission string `json:"permission"`
	// WaitingOn names the subagent (by type) whose permission prompt holds
	// the run; empty when the prompt is the run's own.
	WaitingOn string `json:"waitingOn"`
	// Question is the kind of the open question the run (or a subagent)
	// is waiting on when that, not a permission prompt, is why it needs you.
	Question     string `json:"question"`
	LastActivity string `json:"lastActivity"`
}

// RunJSON is one run (RUN-2). Timeline is sent only for a single run or a
// ticket's runs.
type RunJSON struct {
	ID           string            `json:"id"`
	Short        string            `json:"short"`
	Agent        string            `json:"agent"`
	Kind         string            `json:"kind"`
	Parent       string            `json:"parent"`
	Children     []string          `json:"children"`
	Project      string            `json:"project"`
	Cwd          string            `json:"cwd"`
	Branch       string            `json:"branch"`
	Worktree     string            `json:"worktree"`
	Source       string            `json:"source"`
	AgentType    string            `json:"agentType"`
	State        string            `json:"state"`
	Ticket       string            `json:"ticket"`
	TicketTitle  string            `json:"ticketTitle"`
	LinkedBy     string            `json:"linkedBy"`
	Dirty        bool              `json:"dirty"`
	NoHandoff    bool              `json:"noHandoff"`
	Permission   string            `json:"permission"`
	Started      string            `json:"started"`
	LastActivity string            `json:"lastActivity"`
	Ended        string            `json:"ended"`
	EndReason    string            `json:"endReason"`
	Tools        int               `json:"tools"`
	Edits        int               `json:"edits"`
	Files        []string          `json:"files"`
	Plan         []events.PlanItem `json:"plan"`
	Progress     runs.Progress     `json:"progress"`
	Questions    []runs.Question   `json:"questions"`
	Timeline     []runs.Entry      `json:"timeline,omitempty"`
}

// RunsResponse is GET /api/runs[?project=][&ended=all].
type RunsResponse struct {
	Revision uint64        `json:"revision"`
	Runs     []RunJSON     `json:"runs"`
	Counts   RunCountsJSON `json:"counts"`
	// HiddenEnded counts Ended runs older than a day left out (VIEW-3).
	HiddenEnded int `json:"hiddenEnded"`
}

// RunResponse is GET /api/run?id=.
type RunResponse struct {
	Revision uint64  `json:"revision"`
	Run      RunJSON `json:"run"`
}

func (b boardAPI) registerRuns(mux *http.ServeMux) {
	mux.Handle("/api/runs", getOnly(b.runs))
	mux.Handle("/api/run", getOnly(b.run))
}

func runJSON(snapshot *index.Snapshot, v runs.View, timeline bool) RunJSON {
	result := RunJSON{
		ID: v.ID, Short: protocol.ShortRun(v.ID), Agent: v.Agent, Kind: v.Kind, Parent: v.Parent,
		Children: nonNil(v.Children), Project: v.Project, Cwd: v.Cwd, Branch: v.Branch, Worktree: v.Worktree,
		Source: v.Source, AgentType: v.AgentType, State: string(v.State),
		Ticket: v.Link.Ticket, LinkedBy: v.Link.By, Dirty: v.Dirty, NoHandoff: v.NoHandoff, Permission: v.Permission,
		Started: timestamp(v.Started), LastActivity: timestamp(v.LastActivity), Ended: timestamp(v.EndedAt), EndReason: v.EndReason,
		Tools: v.Tools, Edits: v.Edits, Files: nonNil(v.Files), Plan: nonNil(v.Plan), Progress: v.Progress,
		Questions: nonNil(v.Questions),
	}
	if v.Link.Ticket != "" {
		if _, ticket, ok := snapshot.FindTicket(v.Link.Ticket); ok {
			result.TicketTitle = ticket.Title
		}
	}
	if timeline {
		result.Timeline = nonNil(v.Timeline)
	}
	return result
}

func (b boardAPI) runs(w http.ResponseWriter, r *http.Request) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	project := r.URL.Query().Get("project")
	if _, ok := snapshot.Project(project); project != "" && !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No project %s", project))
		return
	}
	all := r.URL.Query().Get("ended") == "all"
	response := RunsResponse{Revision: snapshot.Revision, Runs: []RunJSON{}, Counts: countsJSON(snapshot.RunCounts(project))}
	for _, v := range snapshot.Runs {
		if project != "" && v.Project != project {
			continue
		}
		if !all && v.State == runs.Ended && snapshot.BuiltAt.Sub(endedAt(v)) > endedVisible {
			response.HiddenEnded++
			continue
		}
		response.Runs = append(response.Runs, runJSON(snapshot, v, false))
	}
	writeJSON(w, http.StatusOK, response)
}

// endedAt is when a run ended, or its last activity when it went stale.
func endedAt(v runs.View) time.Time {
	if !v.EndedAt.IsZero() {
		return v.EndedAt
	}
	return v.LastActivity
}

func (b boardAPI) run(w http.ResponseWriter, r *http.Request) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	id := r.URL.Query().Get("id")
	for _, v := range snapshot.Runs {
		if v.ID == id {
			writeJSON(w, http.StatusOK, RunResponse{Revision: snapshot.Revision, Run: runJSON(snapshot, v, true)})
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "No such run in the last two days")
}

// liveBadge fills a card's live run and virtual-column flags (VIEW-2, VIEW-8).
func liveBadge(snapshot *index.Snapshot, card *Card) {
	// An open question about the ticket needs you whoever asked it.
	card.OpenQuestions = len(snapshot.Questions(card.ID))
	card.NeedsYou = card.OpenQuestions > 0
	v, ok := snapshot.TicketRun(card.ID)
	if !ok {
		return
	}
	card.Live = &LiveJSON{
		Run: v.ID, Short: protocol.ShortRun(v.ID), Agent: v.Agent, State: string(v.State),
		Done: v.Progress.Done, Total: v.Progress.Total, Step: v.Progress.Current,
		Permission: v.Permission, LastActivity: timestamp(v.LastActivity),
	}
	if v.State == runs.NeedsYou && v.Permission == "" {
		for _, child := range snapshot.Runs {
			if child.Parent == v.ID && child.State == runs.NeedsYou && child.Permission != "" {
				card.Live.Permission, card.Live.WaitingOn = child.Permission, child.AgentType
				if card.Live.WaitingOn == "" {
					card.Live.WaitingOn = "Subagent"
				}
				break
			}
		}
	}
	if v.State == runs.NeedsYou && card.Live.Permission == "" {
		card.Live.Question = openQuestion(snapshot, v)
	}
	card.NeedsYou = card.NeedsYou || v.State == runs.NeedsYou
	card.AgentWorking = v.State == runs.Working || v.State == runs.Quiet
}

// openQuestion is the kind of the first open question of a run or its
// subagents, or "".
func openQuestion(snapshot *index.Snapshot, v runs.View) string {
	for _, run := range snapshot.Runs {
		if run.ID != v.ID && run.Parent != v.ID {
			continue
		}
		for _, q := range run.Questions {
			if q.Open() {
				return q.Kind
			}
		}
	}
	return ""
}

// ticketRuns lists a ticket's runs with timelines for its Runs tab.
func ticketRuns(snapshot *index.Snapshot, id string) []RunJSON {
	list := []RunJSON{}
	for _, v := range snapshot.TicketRuns(id) {
		if len(list) == maxTicketRuns {
			break
		}
		list = append(list, runJSON(snapshot, v, true))
	}
	return list
}
