package api

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/protocol"
	"github.com/rztaylor/flashheart/internal/runs"
)

// endedVisible is how long an Ended run stays in the runs list by default
// (VIEW-3).
const endedVisible = 24 * time.Hour

// maxTicketRuns bounds the runs sent with a ticket's Runs tab, and
// maxSubagents the subagents sent for each, most recent first.
const (
	maxTicketRuns = 10
	maxSubagents  = 20
)

// RunCountsJSON counts runs by state; Live is every run not Ended.
type RunCountsJSON struct {
	Working  int `json:"working"`
	NeedsYou int `json:"needsYou"`
	Waiting  int `json:"waiting"`
	Quiet    int `json:"quiet"`
	Ended    int `json:"ended"`
	Live     int `json:"live"`
	// NeedsYouUnticketed are the runs in Needs you with no ticket on the
	// board (FH-44).
	NeedsYouUnticketed int `json:"needsYouUnticketed"`
}

func countsJSON(c index.RunCounts) RunCountsJSON {
	return RunCountsJSON{Working: c.Working, NeedsYou: c.NeedsYou, Waiting: c.Waiting, Quiet: c.Quiet, Ended: c.Ended, Live: c.Live, NeedsYouUnticketed: c.NeedsYouUnticketed}
}

// LiveJSON is a card's live badge (VIEW-8): the linked run that most needs
// attention, with why it needs you (the reason its Runs tab words too).
type LiveJSON struct {
	Run          string      `json:"run"`
	Short        string      `json:"short"`
	Agent        string      `json:"agent"`
	State        string      `json:"state"`
	Done         int         `json:"done"`
	Total        int         `json:"total"`
	Step         string      `json:"step"`
	Reason       runs.Reason `json:"reason"`
	LastActivity string      `json:"lastActivity"`
}

// SessionsJSON summarises the sessions working for a ticket (RUN-9): the
// run that speaks for it (the live one that most needs attention, else the
// most recently active; empty with no run), its state ("ended" when none is
// live, "" with no run), the latest activity of any of them, its handoff
// flags, and the ticket's subagents by state.
type SessionsJSON struct {
	Run          string `json:"run"`
	Short        string `json:"short"`
	Agent        string `json:"agent"`
	State        string `json:"state"`
	LastActivity string `json:"lastActivity"`
	// NoLiveSession marks an in-progress ticket no live run works on.
	NoLiveSession bool               `json:"noLiveSession"`
	Dirty         bool               `json:"dirty"`
	NoHandoff     bool               `json:"noHandoff"`
	Subagents     SubagentCountsJSON `json:"subagents"`
}

// SubagentCountsJSON counts a ticket's subagents: Done (ended), Running
// and NeedsYou.
type SubagentCountsJSON struct {
	Done     int `json:"done"`
	Running  int `json:"running"`
	NeedsYou int `json:"needsYou"`
}

// sessionsJSON is a card's session summary, or nil when no run is linked
// to the ticket and it is not in progress.
func sessionsJSON(snapshot *index.Snapshot, ticket board.Ticket) *SessionsJSON {
	summary, ok := snapshot.Sessions(ticket)
	if !ok {
		return nil
	}
	return &SessionsJSON{
		Run: summary.Run, Short: protocol.ShortRun(summary.Run), Agent: summary.Agent, State: string(summary.State),
		LastActivity:  timestamp(summary.LastActivity),
		NoLiveSession: summary.NoLiveSession, Dirty: summary.Dirty, NoHandoff: summary.NoHandoff,
		Subagents: SubagentCountsJSON{
			Done: summary.Subagents.Done, Running: summary.Subagents.Running, NeedsYou: summary.Subagents.NeedsYou,
		},
	}
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
	Reason       runs.Reason       `json:"reason"`
	Started      string            `json:"started"`
	LastActivity string            `json:"lastActivity"`
	Ended        string            `json:"ended"`
	EndReason    string            `json:"endReason"`
	Tools        int               `json:"tools"`
	Edits        int               `json:"edits"`
	Files        []string          `json:"files"`
	Plan         []events.PlanItem `json:"plan"`
	Progress     runs.Progress     `json:"progress"`
	Questions    []QuestionJSON    `json:"questions"`
	Timeline     []runs.Entry      `json:"timeline,omitempty"`
}

// QuestionJSON is a question with its asking run's short id, as people see
// it (agent-protocol §2).
type QuestionJSON struct {
	runs.Question
	RunShort string `json:"runShort"`
}

// questionsJSON adds each question's run short id. Whether a question's
// session has ended is the run view's SessionEnded (RUN-8), so the card,
// the ticket and the runs the Overview reads agree (FH-112).
func questionsJSON(list []runs.Question) []QuestionJSON {
	out := make([]QuestionJSON, len(list))
	for index, q := range list {
		out[index] = QuestionJSON{Question: q, RunShort: protocol.ShortRun(q.Run)}
	}
	return out
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
		Ticket: v.Link.Ticket, LinkedBy: v.Link.By, Dirty: v.Dirty, NoHandoff: v.NoHandoff, Permission: v.Permission, Reason: v.Reason,
		Started: timestamp(v.Started), LastActivity: timestamp(v.LastActivity), Ended: timestamp(v.EndedAt), EndReason: v.EndReason,
		Tools: v.Tools, Edits: v.Edits, Files: nonNil(v.Files), Plan: nonNil(v.Plan), Progress: v.Progress,
		Questions: questionsJSON(v.Questions),
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
	v, ok := snapshot.Run(r.URL.Query().Get("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "No such run in the last two days")
		return
	}
	writeJSON(w, http.StatusOK, RunResponse{Revision: snapshot.Revision, Run: runJSON(snapshot, v, true)})
}

// liveBadge fills a card's live run and virtual-column flags (VIEW-2,
// VIEW-8), looking the ticket's runs up in the snapshot's run index.
func liveBadge(snapshot *index.Snapshot, card *Card) {
	// An open question about the ticket needs you whoever asked it, while
	// the asking session is live (VIEW-2).
	for _, question := range snapshot.Questions(card.ID) {
		if !question.SessionEnded {
			card.OpenQuestions++
		}
	}
	card.NeedsYou = card.OpenQuestions > 0
	v, ok := snapshot.TicketRun(card.ID)
	if !ok {
		return
	}
	card.Live = &LiveJSON{
		Run: v.ID, Short: protocol.ShortRun(v.ID), Agent: v.Agent, State: string(v.State),
		Done: v.Progress.Done, Total: v.Progress.Total, Step: v.Progress.Current,
		Reason: v.Reason, LastActivity: timestamp(v.LastActivity),
	}
	card.NeedsYou = card.NeedsYou || v.State == runs.NeedsYou
	card.AgentWorking = v.State == runs.Working || v.State == runs.Quiet
}

// ticketRuns lists a ticket's runs with timelines for its Runs tab, with
// every subagent of a listed session, so the tab shows the whole tree
// (agent-protocol §10) even where a subagent claimed another ticket.
// maxTicketRuns bounds the runs linked to the ticket, maxSubagents each
// session's subagents.
func ticketRuns(snapshot *index.Snapshot, id string) []RunJSON {
	linked := snapshot.TicketRuns(id)
	isLinked := map[string]bool{}
	for _, v := range linked {
		isLinked[v.ID] = true
	}
	// Roots are the sessions, and subagents whose session is not linked to
	// the ticket; subagents follow their root.
	list := []RunJSON{}
	listed := map[string]bool{}
	for _, v := range linked {
		if v.Parent != "" && isLinked[v.Parent] {
			continue
		}
		if len(listed) == maxTicketRuns {
			break
		}
		listed[v.ID] = true
		list = append(list, runJSON(snapshot, v, true))
	}
	for _, root := range list {
		subagents := snapshot.Subagents(root.ID)
		sort.SliceStable(subagents, func(a, b int) bool { return subagents[a].LastActivity.After(subagents[b].LastActivity) })
		for _, child := range subagents[:min(len(subagents), maxSubagents)] {
			list = append(list, runJSON(snapshot, child, true))
		}
	}
	return list
}
