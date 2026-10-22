package api

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// searchTextRunes bounds the body text sent with project board cards for
// client-side search (VIEW-7).
const searchTextRunes = 1000

// BoardSource supplies the current board snapshot and waits for changes.
// Rebuild reads the board after a write; RefreshRuns refolds only the runs.
type BoardSource interface {
	Current() (*index.Snapshot, error)
	Rebuild() (*index.Snapshot, error)
	RefreshRuns() (*index.Snapshot, error)
	Wait(ctx context.Context, since uint64) uint64
}

// FileSource reads files that are not kept in the snapshot.
type FileSource interface {
	ReadReview(project, folder string) (string, bool, error)
	ReadTicketActivity(project, folder string) (board.Activity, string, error)
	OpenAttachment(project, folder, file string) (*os.File, string, error)
	OpenProjectImage(project, file string) (*os.File, string, error)
}

// WorkstreamBrief is a workstream as listed with its project: enough to draw
// its line and progress.
type WorkstreamBrief struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Created string `json:"created"`
	Status  string `json:"status"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
}

// ProjectSummary describes one project in the rail (PRJ-6).
type ProjectSummary struct {
	Name         string            `json:"name"`
	DisplayName  string            `json:"displayName"`
	Key          string            `json:"key"`
	KeyDerived   bool              `json:"keyDerived"`
	Repos        []string          `json:"repos"`
	Counts       map[string]int    `json:"counts"`
	NeedsRepair  int               `json:"needsRepair"`
	Blocked      int               `json:"blocked"`
	Warnings     []string          `json:"warnings"`
	LastModified string            `json:"lastModified"`
	Workstreams  []WorkstreamBrief `json:"workstreams"`
	// Archived counts the project's archived tickets (EDIT-8).
	Archived int `json:"archived"`
	// Rejected counts the tickets in Done whose status is rejected (FH-58),
	// so the Overview's pipeline can count accepted work (FH-63).
	Rejected int `json:"rejected"`
	// Runs counts the project's agent runs by state (PRJ-6).
	Runs RunCountsJSON `json:"runs"`
	// Image is the URL of the project's badge image (FH-151), with a
	// version that changes with its bytes; absent when the project shows
	// its key.
	Image string `json:"image,omitempty"`
}

// ProjectsResponse is GET /api/projects.
type ProjectsResponse struct {
	Revision    uint64           `json:"revision"`
	Root        string           `json:"root"`
	RootMissing bool             `json:"rootMissing"`
	Projects    []ProjectSummary `json:"projects"`
	// Runs counts every project's runs, for the band's Needs you badge.
	Runs RunCountsJSON `json:"runs"`
	// ArchivedProjects counts the projects under <root>/.archive/ (PRJ-5).
	ArchivedProjects int `json:"archivedProjects"`
}

// Progress counts acceptance criteria.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// RefJSON names a ticket of a project, as a blocked-by reason's ticket.
type RefJSON struct {
	Project string `json:"project"`
	ID      string `json:"id"`
}

// ReasonJSON is one blocked-by explanation (CARD-4).
type ReasonJSON struct {
	Kind       string   `json:"kind"`
	Text       string   `json:"text"`
	Ticket     *RefJSON `json:"ticket,omitempty"`
	Workstream string   `json:"workstream,omitempty"`
	Column     string   `json:"column,omitempty"`
	Pending    int      `json:"pending,omitempty"`
	Missing    bool     `json:"missing"`
	Via        string   `json:"via,omitempty"`
}

// Card is a ticket as shown on a board (VIEW-6).
type Card struct {
	Project string `json:"project"`
	ID      string `json:"id"`
	Slug    string `json:"slug"`
	Column  string `json:"column"`
	// Rejected marks a ticket in Done whose status is rejected (FH-58).
	Rejected    bool         `json:"rejected"`
	Title       string       `json:"title"`
	Type        string       `json:"type"`
	Priority    string       `json:"priority"`
	Workstream  string       `json:"workstream"`
	Tags        []string     `json:"tags"`
	Created     string       `json:"created"`
	Updated     string       `json:"updated"`
	Modified    string       `json:"modified"`
	Branch      string       `json:"branch"`
	PR          string       `json:"pr"`
	DependsOn   []string     `json:"dependsOn"`
	Criteria    Progress     `json:"criteria"`
	Excerpt     string       `json:"excerpt"`
	HandoffNext string       `json:"handoffNext"`
	Attachments int          `json:"attachments"`
	HasReview   bool         `json:"hasReview"`
	Blocked     bool         `json:"blocked"`
	BlockedBy   []ReasonJSON `json:"blockedBy"`
	NeedsRepair []string     `json:"needsRepair"`
	Warnings    []string     `json:"warnings"`
	SearchText  string       `json:"searchText,omitempty"`
	// Live is the linked run that most needs attention (VIEW-8); NeedsYou
	// answers the band's Needs you filter (VIEW-2, FH-44), and AgentWorking
	// answers the Agent working State filter (FH-42).
	Live         *LiveJSON `json:"live,omitempty"`
	NeedsYou     bool      `json:"needsYou"`
	AgentWorking bool      `json:"agentWorking"`
	// Reason is why the card needs you (FH-185, D39): its linked run's
	// reason when that is about the ticket, else the oldest open question
	// about it from a live session; Kind is "" when it does not.
	Reason NeedsReasonJSON `json:"reason"`
	// OpenQuestions counts open questions about the ticket (CARD-6).
	OpenQuestions int `json:"openQuestions"`
	// ChangeRequests counts the ticket's change requests still open or
	// addressed (FH-126); zero for a ticket in Done.
	ChangeRequests RequestsJSON `json:"changeRequests"`
	// Sessions summarises the runs working for the ticket (RUN-9); absent
	// when none is linked and the ticket is not in progress.
	Sessions *SessionsJSON `json:"sessions,omitempty"`
}

// BoardResponse is GET /api/projects/{project}/board.
type BoardResponse struct {
	Revision  uint64         `json:"revision"`
	Project   ProjectSummary `json:"project"`
	Cards     []Card         `json:"cards"`
	DoneTotal int            `json:"doneTotal"`
	DoneShown int            `json:"doneShown"`
}

// AllBoardResponse is GET /api/all/board.
type AllBoardResponse struct {
	Revision  uint64           `json:"revision"`
	Projects  []ProjectSummary `json:"projects"`
	Cards     []Card           `json:"cards"`
	DoneTotal int              `json:"doneTotal"`
	DoneShown int              `json:"doneShown"`
}

// HandoffJSON is a ticket's handoff section.
type HandoffJSON struct {
	Markdown string   `json:"markdown"`
	Next     []string `json:"next"`
}

// ReviewJSON is a ticket's review file, split around its How to Verify
// steps so they show as a checklist (REV-3); Hash guards ticking them and
// is empty when read-only.
type ReviewJSON struct {
	Markdown string           `json:"markdown"`
	Hash     string           `json:"hash"`
	Before   string           `json:"before"`
	Intro    string           `json:"intro"`
	Steps    []ReviewStepJSON `json:"steps"`
	Outro    string           `json:"outro"`
	After    string           `json:"after"`
}

// ReviewStepJSON is one How to Verify step and whether it is ticked.
type ReviewStepJSON struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// CriterionJSON is one acceptance criterion and whether it is ticked.
type CriterionJSON struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

func reviewStepsJSON(steps []board.ReviewStep) []ReviewStepJSON {
	out := make([]ReviewStepJSON, 0, len(steps))
	for _, step := range steps {
		out = append(out, ReviewStepJSON{Text: step.Text, Done: step.Done})
	}
	return out
}

func criteriaJSON(criteria []board.Criterion) []CriterionJSON {
	out := make([]CriterionJSON, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, CriterionJSON{Text: criterion.Text, Done: criterion.Done})
	}
	return out
}

// AttachmentJSON is one attachment with the URL that serves it.
type AttachmentJSON struct {
	File    string `json:"file"`
	Caption string `json:"caption"`
	Kind    string `json:"kind"`
	Run     string `json:"run"`
	Added   string `json:"added"`
	URL     string `json:"url"`
}

// TicketDetail is a card plus everything the card panel shows (CARD-1).
type TicketDetail struct {
	Card
	Body                 string           `json:"body"`
	Frontmatter          string           `json:"frontmatter"`
	Session              string           `json:"session"`
	GitRef               string           `json:"gitRef"`
	DependsOnWorkstreams []string         `json:"dependsOnWorkstreams"`
	CriteriaItems        []CriterionJSON  `json:"criteriaItems"`
	Handoff              *HandoffJSON     `json:"handoff"`
	Review               *ReviewJSON      `json:"review"`
	AttachmentFiles      []AttachmentJSON `json:"attachmentFiles"`
	// Hash is the file's content hash, sent back with every edit; Raw is the
	// whole file for the raw editor (EDIT-6). Both are empty when read-only.
	Hash string `json:"hash"`
	Raw  string `json:"raw"`
	// Runs are the runs linked to the ticket, most recent first (CARD-1).
	Runs []RunJSON `json:"runs"`
	// Questions are the open questions about the ticket (CARD-6).
	Questions []QuestionJSON `json:"questions"`
	// Comments and History are the ticket's activity file, oldest first
	// (D47); ActivityHash is its content hash, sent back with a comment
	// edit, empty when read-only or when the ticket has none.
	Comments     []CommentJSON `json:"comments"`
	History      []HistoryJSON `json:"history"`
	ActivityHash string        `json:"activityHash"`
	// TellSessions are the runs a comment may be sent to now (FH-127):
	// each live session holding the ticket, then its live subagents; none
	// when no live session has claimed it.
	TellSessions []TellSessionJSON `json:"tellSessions"`
}

// TicketResponse is GET /api/tickets/{id} and
// GET /api/projects/{project}/tickets/{id}.
type TicketResponse struct {
	Revision uint64       `json:"revision"`
	Ticket   TicketDetail `json:"ticket"`
}

// WorkstreamTicket is one ticket in a workstream swimlane, in order.
type WorkstreamTicket struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Column   string `json:"column"`
	Rejected bool   `json:"rejected"`
	Blocked  bool   `json:"blocked"`
	// Held means blocked by something other than a ticket on this line or
	// its workstream-level dependencies.
	Held    bool `json:"held"`
	Missing bool `json:"missing"`
	// DependsOn are the tickets on this line it depends on, in its
	// depends-on order: the graph's edges into this station (VIEW-4).
	DependsOn []string `json:"dependsOn"`
	// Outside are its unfinished depends-on tickets not on this line.
	Outside []string `json:"outside"`
}

// WorkstreamJSON is one workstream with derived status (VIEW-4).
type WorkstreamJSON struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Status string `json:"status"`
	// Suspended means the workstream's own depends-on-workstreams are not
	// complete, so the whole line waits.
	Suspended            bool               `json:"suspended"`
	DeclaredStatus       string             `json:"declaredStatus"`
	Priority             string             `json:"priority"`
	Created              string             `json:"created"`
	Done                 int                `json:"done"`
	Total                int                `json:"total"`
	Next                 string             `json:"next"`
	BlockedBy            []ReasonJSON       `json:"blockedBy"`
	DependsOnWorkstreams []string           `json:"dependsOnWorkstreams"`
	Tags                 []string           `json:"tags"`
	NeedsRepair          []string           `json:"needsRepair"`
	Warnings             []string           `json:"warnings"`
	Tickets              []WorkstreamTicket `json:"tickets"`
}

// WorkstreamsResponse is GET /api/projects/{project}/workstreams.
type WorkstreamsResponse struct {
	Revision    uint64           `json:"revision"`
	Workstreams []WorkstreamJSON `json:"workstreams"`
}

// WorkstreamDetail is a workstream's page (FH-139): its line, its body,
// and, with the write side, its file and the hash a section save sends.
type WorkstreamDetail struct {
	WorkstreamJSON
	Body string `json:"body"`
	Hash string `json:"hash"`
	Raw  string `json:"raw"`
}

// WorkstreamResponse is GET /api/projects/{project}/workstreams/{slug}.
type WorkstreamResponse struct {
	Revision   uint64           `json:"revision"`
	Workstream WorkstreamDetail `json:"workstream"`
}

type boardAPI struct {
	board     BoardSource
	files     FileSource
	root      string
	doneLimit int
	stopping  <-chan struct{}
	longPoll  time.Duration
	write     Writer
	events    *events.Log
	now       func() time.Time
	// answering serialises answers, so a question is answered once.
	answering *sync.Mutex
	// preferring orders preference reads after any save in flight, so a page
	// reloaded straight after a change flushed on unload reads it (FH-109).
	preferring *sync.RWMutex
	// answeredBy names who answers from the board (RUN-8).
	answeredBy string
}

func (b boardAPI) register(mux *http.ServeMux) {
	mux.Handle("/api/projects", getOnly(b.projects))
	mux.Handle("/api/projects/{project}/board", getOnly(b.projectBoard))
	mux.Handle("/api/projects/{project}/tickets/{id}", getOnly(b.ticket))
	mux.Handle("/api/tickets/{id}", getOnly(b.ticket))
	mux.Handle("/api/projects/{project}/workstreams", getOnly(b.workstreams))
	mux.Handle("/api/projects/{project}/workstreams/{slug}", getOnly(b.workstream))
	mux.Handle("/api/projects/{project}/tickets/{id}/files/{file}", getOnly(b.attachment))
	mux.HandleFunc("GET /api/projects/{project}/image", b.projectImage)
	mux.Handle("/api/all/board", getOnly(b.allBoard))
	mux.Handle("/api/changes", getOnly(b.changes))
	b.registerRuns(mux)
	b.registerWrites(mux)
	b.registerArchive(mux)
	b.registerProjectArchive(mux)
}

// ChangesResponse is GET /api/changes?since=N: the revision once it is newer
// than since, or the unchanged revision when the wait ends (LIFE-3).
type ChangesResponse struct {
	Revision uint64 `json:"revision"`
}

func (b boardAPI) changes(w http.ResponseWriter, r *http.Request) {
	since, err := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "since must be a revision number")
		return
	}
	if b.snapshot(w) == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), b.longPoll)
	defer cancel()
	if b.stopping != nil {
		go func() {
			select {
			case <-b.stopping:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	writeJSON(w, http.StatusOK, ChangesResponse{Revision: b.board.Wait(ctx, since)})
}

func getOnly(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "This endpoint only supports GET")
			return
		}
		handler(w, r)
	})
}

// snapshot returns the current snapshot, or writes an error and returns nil.
// A failed rebuild still serves the previous snapshot when there is one.
func (b boardAPI) snapshot(w http.ResponseWriter) *index.Snapshot {
	snapshot, err := b.board.Current()
	if snapshot == nil {
		message := "The board could not be read"
		if err != nil {
			message += ": " + err.Error()
		}
		writeError(w, http.StatusInternalServerError, "board_unreadable", message)
		return nil
	}
	return snapshot
}

func (b boardAPI) project(w http.ResponseWriter, r *http.Request) (*index.Snapshot, *board.Project) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return nil, nil
	}
	name := r.PathValue("project")
	project, ok := snapshot.Project(name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No project named %q", name))
		return nil, nil
	}
	return snapshot, project
}

func (b boardAPI) projects(w http.ResponseWriter, _ *http.Request) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	response := ProjectsResponse{
		Revision:    snapshot.Revision,
		Root:        b.root,
		RootMissing: snapshot.RootMissing,
		Projects:    summaries(snapshot),
		Runs:        countsJSON(snapshot.RunCounts("")),

		ArchivedProjects: len(snapshot.Board.ArchivedProjects),
	}
	writeJSON(w, http.StatusOK, response)
}

func (b boardAPI) projectBoard(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	cards, total, shown := b.cards(snapshot, project, r.URL.Query().Get("done") == "all", true)
	writeJSON(w, http.StatusOK, BoardResponse{
		Revision: snapshot.Revision, Project: summary(snapshot, project),
		Cards: cards, DoneTotal: total, DoneShown: shown,
	})
}

func (b boardAPI) allBoard(w http.ResponseWriter, r *http.Request) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	response := AllBoardResponse{Revision: snapshot.Revision, Projects: summaries(snapshot), Cards: []Card{}}
	all := r.URL.Query().Get("done") == "all"
	for index := range snapshot.Board.Projects {
		cards, total, shown := b.cards(snapshot, &snapshot.Board.Projects[index], all, false)
		response.Cards = append(response.Cards, cards...)
		response.DoneTotal += total
		response.DoneShown += shown
	}
	slices.SortStableFunc(response.Cards, func(a, c Card) int {
		return cmp.Compare(columnRank(a.Column), columnRank(c.Column))
	})
	writeJSON(w, http.StatusOK, response)
}

// findTicket resolves {id}, within {project} when the route names one.
func (b boardAPI) findTicket(w http.ResponseWriter, r *http.Request) (*index.Snapshot, *board.Project, board.Ticket, bool) {
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return nil, nil, board.Ticket{}, false
	}
	id := r.PathValue("id")
	project, ticket, ok := snapshot.FindTicket(id)
	if name := r.PathValue("project"); ok && name != "" && project.Name != name {
		ok = false
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No ticket %s", id))
		return nil, nil, board.Ticket{}, false
	}
	return snapshot, project, ticket, true
}

func (b boardAPI) ticket(w http.ResponseWriter, r *http.Request) {
	snapshot, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	// With the write side, show the ticket parsed from the same bytes as the
	// hash an edit will send, so an edit never applies to content the user
	// did not see (STO-3).
	var hash, raw string
	if b.write != nil {
		if data, current, err := b.write.ReadTicket(project.Name, ticket.ID); err == nil {
			fresh := board.ParseTicket(ticket.Folder, data)
			fresh.Modified = ticket.Modified
			for _, warning := range ticket.Warnings {
				if !slices.Contains(fresh.Warnings, warning) {
					fresh.Warnings = append(fresh.Warnings, warning)
				}
			}
			ticket, hash, raw = fresh, current, string(data)
		}
	}
	detail := TicketDetail{
		Card:                 card(snapshot, project, ticket, true),
		Body:                 ticket.Body,
		Frontmatter:          ticket.FrontmatterRaw,
		Session:              ticket.Session,
		GitRef:               ticket.GitRef,
		DependsOnWorkstreams: nonNil(ticket.DependsOnWorkstreams),
		CriteriaItems:        criteriaJSON(ticket.Criteria),
		AttachmentFiles:      []AttachmentJSON{},
		Runs:                 ticketRuns(snapshot, ticket.ID),
		Questions:            questionsJSON(snapshot.Questions(ticket.ID)),
		Comments:             []CommentJSON{},
		History:              []HistoryJSON{},
		TellSessions:         tellSessions(snapshot, ticket.ID),
	}
	if b.files != nil {
		// An activity file that cannot be read shows no activity; the
		// ticket itself still opens.
		if activity, activityHash, err := b.files.ReadTicketActivity(project.Name, ticket.Folder); err == nil {
			detail.Comments, detail.History = activityJSON(activity, snapshot)
			detail.ChangeRequests = requestsJSON(board.CountRequests(activity.Comments))
			if b.write != nil {
				detail.ActivityHash = activityHash
			}
		}
	}
	if ticket.Handoff != nil {
		detail.Handoff = &HandoffJSON{Markdown: ticket.Handoff.Markdown, Next: nonNil(ticket.Handoff.Next)}
	}
	if project.Reviews[ticket.ID] && b.files != nil {
		review, found, err := b.files.ReadReview(project.Name, ticket.Folder)
		if err == nil && found {
			parts := board.ParseReview(review)
			detail.Review = &ReviewJSON{
				Markdown: review, Before: parts.Before, Intro: parts.Intro,
				Steps: reviewStepsJSON(parts.Steps), Outro: parts.Outro, After: parts.After,
			}
			if b.write != nil {
				detail.Review.Hash = store.Hash([]byte(review))
			}
		}
	}
	for _, attachment := range project.Attachments[ticket.ID] {
		detail.AttachmentFiles = append(detail.AttachmentFiles, AttachmentJSON{
			File: attachment.File, Caption: attachment.Caption, Kind: attachment.Kind,
			Run: attachment.Run, Added: attachment.Added,
			URL: filesURL(project.Name, ticket.ID) + url.PathEscape(attachment.File),
		})
	}
	detail.Hash, detail.Raw = hash, raw
	writeJSON(w, http.StatusOK, TicketResponse{Revision: snapshot.Revision, Ticket: detail})
}

func (b boardAPI) workstreams(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	response := WorkstreamsResponse{Revision: snapshot.Revision, Workstreams: []WorkstreamJSON{}}
	for _, workstream := range project.Workstreams {
		response.Workstreams = append(response.Workstreams, workstreamJSON(snapshot, project, workstream))
	}
	writeJSON(w, http.StatusOK, response)
}

// workstreamJSON is a workstream's line with its derived status (VIEW-4).
func workstreamJSON(snapshot *index.Snapshot, project *board.Project, workstream board.Workstream) WorkstreamJSON {
	state := snapshot.Analysis.Workstreams[board.Ref{Project: project.Name, ID: workstream.Slug}]
	item := WorkstreamJSON{
		Slug: workstream.Slug, Title: workstream.Title, Status: state.Status,
		Suspended:      suspended(snapshot, project.Name, workstream.DependsOnWorkstreams),
		DeclaredStatus: workstream.Status, Priority: workstream.Priority, Created: workstream.Created,
		Done: state.Done, Total: state.Total, Next: state.Next,
		BlockedBy:            reasons(project.Name, state.Reasons),
		DependsOnWorkstreams: nonNil(workstream.DependsOnWorkstreams),
		Tags:                 nonNil(workstream.Tags),
		NeedsRepair:          nonNil(workstream.Repair),
		Warnings:             nonNil(workstream.Warnings),
		Tickets:              []WorkstreamTicket{},
	}
	for _, id := range workstream.Tickets {
		entry := WorkstreamTicket{ID: id, Title: id, Missing: true, DependsOn: []string{}, Outside: []string{}}
		if owner, ticket, ok := snapshot.FindTicket(id); ok {
			ref := board.Ref{Project: owner.Name, ID: id}
			reasons := snapshot.Analysis.Blocked[ref]
			entry = WorkstreamTicket{
				ID: id, Title: ticket.Title, Column: string(ticket.Column), Rejected: ticket.Rejected,
				Blocked: len(reasons) > 0, Held: heldOutsideLine(reasons, workstream),
				DependsOn: []string{}, Outside: []string{},
			}
			for _, dependency := range ticket.DependsOn {
				switch {
				case dependency == id || slices.Contains(entry.DependsOn, dependency):
				case slices.Contains(workstream.Tickets, dependency):
					entry.DependsOn = append(entry.DependsOn, dependency)
				case slices.ContainsFunc(reasons, func(reason board.Reason) bool {
					return reason.Kind == board.TicketDependency && reason.Ticket.ID == dependency
				}):
					entry.Outside = append(entry.Outside, dependency)
				}
			}
		} else if snapshot.Archived(id) {
			entry = WorkstreamTicket{ID: id, Title: id, Column: "archived", DependsOn: []string{}, Outside: []string{}}
		}
		item.Tickets = append(item.Tickets, entry)
	}
	return item
}

func (b boardAPI) workstream(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	slug := r.PathValue("slug")
	at := slices.IndexFunc(project.Workstreams, func(workstream board.Workstream) bool { return workstream.Slug == slug })
	if at < 0 {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No workstream %s in %s", slug, project.Name))
		return
	}
	workstream := project.Workstreams[at]
	// With the write side, show the body from the same bytes as the hash a
	// section save will send, as a ticket's page does (STO-3).
	var hash, raw string
	if b.write != nil {
		if data, current, err := b.write.ReadWorkstream(project.Name, slug); err == nil {
			workstream.Body = board.ParseWorkstream(slug, data).Body
			hash, raw = current, string(data)
		}
	}
	writeJSON(w, http.StatusOK, WorkstreamResponse{Revision: snapshot.Revision, Workstream: WorkstreamDetail{
		WorkstreamJSON: workstreamJSON(snapshot, project, workstream),
		Body:           workstream.Body, Hash: hash, Raw: raw,
	}})
}

func (b boardAPI) attachment(w http.ResponseWriter, r *http.Request) {
	if b.files == nil {
		writeError(w, http.StatusNotFound, "not_found", "Attachments are unavailable")
		return
	}
	_, _, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	file, contentType, err := b.files.OpenAttachment(r.PathValue("project"), ticket.Folder, r.PathValue("file"))
	switch {
	case errors.Is(err, store.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "invalid_input", "That attachment name is not valid")
		return
	case errors.Is(err, store.ErrTypeNotAllowed):
		writeError(w, http.StatusUnsupportedMediaType, "type_not_allowed", "That attachment type is not served")
		return
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "No such attachment")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "unreadable", "The attachment could not be read")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unreadable", "The attachment could not be read")
		return
	}
	// SEC-4: the detected type, never sniffed, and a sandbox for anything a
	// browser might render on its own.
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", info.ModTime(), file)
}

// cards returns a project's cards sorted for display, with the done column
// trimmed to the most recent doneLimit unless all is set (VIEW-1).
func (b boardAPI) cards(snapshot *index.Snapshot, project *board.Project, all, withSearch bool) ([]Card, int, int) {
	tickets := slices.Clone(project.Tickets)
	slices.SortStableFunc(tickets, board.CompareOrder)
	cards := make([]Card, 0, len(tickets))
	total, shown := 0, 0
	for _, ticket := range tickets {
		if ticket.Column == board.Done {
			total++
			if !all && b.doneLimit > 0 && shown >= b.doneLimit {
				continue
			}
			shown++
		}
		cards = append(cards, card(snapshot, project, ticket, withSearch))
	}
	return cards, total, shown
}

func columnRank(column string) int {
	return slices.Index(board.Columns, board.Column(column))
}

func filesURL(project, id string) string {
	return "/api/projects/" + url.PathEscape(project) + "/tickets/" + url.PathEscape(id) + "/files/"
}

func card(snapshot *index.Snapshot, project *board.Project, ticket board.Ticket, withSearch bool) Card {
	ref := board.Ref{Project: project.Name, ID: ticket.ID}
	blockedBy := reasons(project.Name, snapshot.Analysis.Blocked[ref])
	result := Card{
		Project: project.Name, ID: ticket.ID, Slug: ticket.Slug, Column: string(ticket.Column), Rejected: ticket.Rejected,
		Title: ticket.Title, Type: ticket.Type, Priority: ticket.Priority, Workstream: ticket.Workstream,
		Tags: nonNil(ticket.Tags), Created: ticket.Created, Updated: ticket.Updated,
		Modified: timestamp(ticket.Modified), Branch: ticket.Branch, PR: ticket.PR, DependsOn: nonNil(ticket.DependsOn),
		Excerpt: ticket.Excerpt, Attachments: len(project.Attachments[ticket.ID]), HasReview: project.Reviews[ticket.ID],
		Blocked: len(blockedBy) > 0, BlockedBy: blockedBy,
		NeedsRepair: nonNil(ticket.Repair), ChangeRequests: requestsJSON(ticket.Requests),
		Warnings: nonNil(slices.Concat(ticket.Warnings, snapshot.Analysis.Warnings[ref])),
	}
	for _, criterion := range ticket.Criteria {
		result.Criteria.Total++
		if criterion.Done {
			result.Criteria.Done++
		}
	}
	if ticket.Handoff != nil && len(ticket.Handoff.Next) > 0 {
		result.HandoffNext = ticket.Handoff.Next[0]
	}
	liveBadge(snapshot, &result)
	result.Sessions = sessionsJSON(snapshot, ticket)
	if withSearch {
		text := []rune(ticket.Body)
		if len(text) > searchTextRunes {
			text = text[:searchTextRunes]
		}
		result.SearchText = string(text)
	}
	return result
}

func reasons(viewing string, list []board.Reason) []ReasonJSON {
	out := make([]ReasonJSON, 0, len(list))
	for _, reason := range list {
		item := ReasonJSON{
			Kind: string(reason.Kind), Text: reason.Describe(), Workstream: reason.Workstream,
			Column: string(reason.Column), Pending: reason.Pending, Missing: reason.Missing, Via: reason.Via,
		}
		if reason.Ticket.ID != "" {
			item.Ticket = &RefJSON{Project: reason.Ticket.Project, ID: reason.Ticket.ID}
		}
		out = append(out, item)
	}
	return out
}

// summaries lists projects, most recently modified first (PRJ-6).
func summaries(snapshot *index.Snapshot) []ProjectSummary {
	projects := make([]*board.Project, 0, len(snapshot.Board.Projects))
	for index := range snapshot.Board.Projects {
		projects = append(projects, &snapshot.Board.Projects[index])
	}
	slices.SortStableFunc(projects, func(a, b *board.Project) int {
		if c := b.LastModified.Compare(a.LastModified); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	list := make([]ProjectSummary, 0, len(projects))
	for _, project := range projects {
		list = append(list, summary(snapshot, project))
	}
	return list
}

func summary(snapshot *index.Snapshot, project *board.Project) ProjectSummary {
	result := ProjectSummary{
		Name: project.Name, DisplayName: project.DisplayName, Key: project.Key, KeyDerived: project.KeyDerived,
		Repos:  nonNil(project.Repos),
		Counts: map[string]int{}, Warnings: nonNil(project.Warnings), LastModified: timestamp(project.LastModified),
		Workstreams: []WorkstreamBrief{},
		Archived:    len(project.Archived),
		Runs:        countsJSON(snapshot.RunCounts(project.Name)),
		Image:       projectImageURL(project),
	}
	for _, workstream := range project.Workstreams {
		state := snapshot.Analysis.Workstreams[board.Ref{Project: project.Name, ID: workstream.Slug}]
		result.Workstreams = append(result.Workstreams, WorkstreamBrief{
			Slug: workstream.Slug, Title: workstream.Title, Created: workstream.Created,
			Status: state.Status, Done: state.Done, Total: state.Total,
		})
	}
	for _, column := range board.Columns {
		result.Counts[string(column)] = 0
	}
	for _, ticket := range project.Tickets {
		result.Counts[string(ticket.Column)]++
		if ticket.Rejected {
			result.Rejected++
		}
		if ticket.NeedsRepair() {
			result.NeedsRepair++
		}
		reasons := snapshot.Analysis.Blocked[board.Ref{Project: project.Name, ID: ticket.ID}]
		if len(reasons) > 0 {
			result.Blocked++
		}
	}
	return result
}

func timestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// heldOutsideLine reports whether any reason comes from outside the
// workstream: not a ticket on the line, nor its workstream-level
// dependencies. A dependency on a ticket on the line is drawn as track.
func heldOutsideLine(reasons []board.Reason, workstream board.Workstream) bool {
	return slices.ContainsFunc(reasons, func(reason board.Reason) bool {
		switch {
		case reason.Kind == board.TicketDependency && slices.Contains(workstream.Tickets, reason.Ticket.ID):
			return false
		case reason.Kind == board.WorkstreamDependency && reason.Via == workstream.Slug && reason.Workstream != workstream.Slug:
			return false
		}
		return true
	})
}

// suspended reports whether any depended-on workstream is missing or has
// tickets not yet in review or done.
func suspended(snapshot *index.Snapshot, project string, dependencies []string) bool {
	for _, name := range dependencies {
		ref := board.Ref{Project: project, ID: name}
		state, ok := snapshot.Analysis.Workstreams[ref]
		if !ok || state.Done < state.Total {
			return true
		}
	}
	return false
}
