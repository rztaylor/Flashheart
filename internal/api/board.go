package api

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"time"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/index"
	"github.com/rztaylor/flashheart/internal/store"
)

// searchTextRunes bounds the body text sent with project board cards for
// client-side search (VIEW-7).
const searchTextRunes = 1000

// BoardSource supplies the current board snapshot.
type BoardSource interface {
	Current() (*index.Snapshot, error)
}

// FileSource reads files that are not kept in the snapshot.
type FileSource interface {
	ReadReview(project, slug string) (string, bool, error)
	OpenAttachment(project, ticket, file string) (*os.File, string, error)
}

// ProjectSummary describes one project in the rail (PRJ-6).
type ProjectSummary struct {
	Name         string         `json:"name"`
	DisplayName  string         `json:"displayName"`
	Repos        []string       `json:"repos"`
	Counts       map[string]int `json:"counts"`
	NeedsRepair  int            `json:"needsRepair"`
	Blocked      int            `json:"blocked"`
	Warnings     []string       `json:"warnings"`
	LastModified string         `json:"lastModified"`
}

// ProjectsResponse is GET /api/projects.
type ProjectsResponse struct {
	Revision    uint64           `json:"revision"`
	Root        string           `json:"root"`
	RootMissing bool             `json:"rootMissing"`
	Projects    []ProjectSummary `json:"projects"`
}

// Progress counts acceptance criteria.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// ReasonJSON is one blocked-by explanation (CARD-4).
type ReasonJSON struct {
	Kind       string     `json:"kind"`
	Text       string     `json:"text"`
	Ticket     *board.Ref `json:"ticket,omitempty"`
	Workstream string     `json:"workstream,omitempty"`
	Column     string     `json:"column,omitempty"`
	Pending    int        `json:"pending,omitempty"`
	Missing    bool       `json:"missing"`
	Via        string     `json:"via,omitempty"`
}

// Card is a ticket as shown on a board (VIEW-6).
type Card struct {
	Project     string       `json:"project"`
	Slug        string       `json:"slug"`
	Column      string       `json:"column"`
	Title       string       `json:"title"`
	Type        string       `json:"type"`
	Priority    string       `json:"priority"`
	Workstream  string       `json:"workstream"`
	Tags        []string     `json:"tags"`
	Created     string       `json:"created"`
	Updated     string       `json:"updated"`
	Modified    string       `json:"modified"`
	Branch      string       `json:"branch"`
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

// ReviewJSON is a ticket's review file.
type ReviewJSON struct {
	Markdown string `json:"markdown"`
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
	Body                 string            `json:"body"`
	Frontmatter          string            `json:"frontmatter"`
	Session              string            `json:"session"`
	GitRef               string            `json:"gitRef"`
	DependsOnWorkstreams []string          `json:"dependsOnWorkstreams"`
	CriteriaItems        []board.Criterion `json:"criteriaItems"`
	Handoff              *HandoffJSON      `json:"handoff"`
	Review               *ReviewJSON       `json:"review"`
	AttachmentFiles      []AttachmentJSON  `json:"attachmentFiles"`
}

// TicketResponse is GET /api/projects/{project}/tickets/{slug}.
type TicketResponse struct {
	Revision uint64       `json:"revision"`
	Ticket   TicketDetail `json:"ticket"`
}

// WorkstreamTicket is one ticket in a workstream swimlane, in order.
type WorkstreamTicket struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Column  string `json:"column"`
	Blocked bool   `json:"blocked"`
	Missing bool   `json:"missing"`
}

// WorkstreamJSON is one workstream with derived status (VIEW-4).
type WorkstreamJSON struct {
	Slug                 string             `json:"slug"`
	Title                string             `json:"title"`
	Status               string             `json:"status"`
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

type boardAPI struct {
	board     BoardSource
	files     FileSource
	root      string
	doneLimit int
}

func (b boardAPI) register(mux *http.ServeMux) {
	mux.Handle("/api/projects", getOnly(b.projects))
	mux.Handle("/api/projects/{project}/board", getOnly(b.projectBoard))
	mux.Handle("/api/projects/{project}/tickets/{slug}", getOnly(b.ticket))
	mux.Handle("/api/projects/{project}/workstreams", getOnly(b.workstreams))
	mux.Handle("/api/projects/{project}/attachments/{ticket}/{file}", getOnly(b.attachment))
	mux.Handle("/api/all/board", getOnly(b.allBoard))
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
	writeJSON(w, http.StatusOK, ProjectsResponse{
		Revision:    snapshot.Revision,
		Root:        b.root,
		RootMissing: snapshot.RootMissing,
		Projects:    summaries(snapshot),
	})
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

func (b boardAPI) ticket(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	slug := r.PathValue("slug")
	ticket, ok := snapshot.Ticket(project.Name, slug)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No ticket %q in project %s", slug, project.Name))
		return
	}
	detail := TicketDetail{
		Card:                 card(snapshot, project, ticket, true),
		Body:                 ticket.Body,
		Frontmatter:          ticket.FrontmatterRaw,
		Session:              ticket.Session,
		GitRef:               ticket.GitRef,
		DependsOnWorkstreams: nonNil(ticket.DependsOnWorkstreams),
		CriteriaItems:        nonNil(ticket.Criteria),
		AttachmentFiles:      []AttachmentJSON{},
	}
	if ticket.Handoff != nil {
		detail.Handoff = &HandoffJSON{Markdown: ticket.Handoff.Markdown, Next: nonNil(ticket.Handoff.Next)}
	}
	if project.Reviews[ticket.Slug] && b.files != nil {
		review, found, err := b.files.ReadReview(project.Name, ticket.Slug)
		if err == nil && found {
			detail.Review = &ReviewJSON{Markdown: review}
		}
	}
	for _, attachment := range project.Attachments[ticket.Slug] {
		detail.AttachmentFiles = append(detail.AttachmentFiles, AttachmentJSON{
			File: attachment.File, Caption: attachment.Caption, Kind: attachment.Kind,
			Run: attachment.Run, Added: attachment.Added,
			URL: "/api/projects/" + url.PathEscape(project.Name) + "/attachments/" + url.PathEscape(ticket.Slug) + "/" + url.PathEscape(attachment.File),
		})
	}
	writeJSON(w, http.StatusOK, TicketResponse{Revision: snapshot.Revision, Ticket: detail})
}

func (b boardAPI) workstreams(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	response := WorkstreamsResponse{Revision: snapshot.Revision, Workstreams: []WorkstreamJSON{}}
	for _, workstream := range project.Workstreams {
		state := snapshot.Analysis.Workstreams[board.Ref{Project: project.Name, Slug: workstream.Slug}]
		item := WorkstreamJSON{
			Slug: workstream.Slug, Title: workstream.Title, Status: state.Status,
			DeclaredStatus: workstream.Status, Priority: workstream.Priority, Created: workstream.Created,
			Done: state.Done, Total: state.Total, Next: state.Next,
			BlockedBy:            reasons(project.Name, state.Reasons),
			DependsOnWorkstreams: nonNil(workstream.DependsOnWorkstreams),
			Tags:                 nonNil(workstream.Tags),
			NeedsRepair:          nonNil(workstream.Repair),
			Warnings:             nonNil(workstream.Warnings),
			Tickets:              []WorkstreamTicket{},
		}
		for _, slug := range workstream.Tickets {
			entry := WorkstreamTicket{Slug: slug, Title: slug, Missing: true}
			if ticket, ok := snapshot.Ticket(project.Name, slug); ok {
				ref := board.Ref{Project: project.Name, Slug: slug}
				entry = WorkstreamTicket{Slug: slug, Title: ticket.Title, Column: string(ticket.Column), Blocked: len(snapshot.Analysis.Blocked[ref]) > 0}
			} else if slices.Contains(project.Archived, slug) {
				entry = WorkstreamTicket{Slug: slug, Title: slug, Column: "archived"}
			}
			item.Tickets = append(item.Tickets, entry)
		}
		response.Workstreams = append(response.Workstreams, item)
	}
	writeJSON(w, http.StatusOK, response)
}

func (b boardAPI) attachment(w http.ResponseWriter, r *http.Request) {
	if b.files == nil {
		writeError(w, http.StatusNotFound, "not_found", "Attachments are unavailable")
		return
	}
	file, contentType, err := b.files.OpenAttachment(r.PathValue("project"), r.PathValue("ticket"), r.PathValue("file"))
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
	slices.SortStableFunc(tickets, compareTickets)
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

var priorityRank = map[string]int{"high": 0, "medium": 1, "low": 2}

func columnRank(column string) int {
	return slices.Index(board.Columns, board.Column(column))
}

func compareTickets(a, b board.Ticket) int {
	if c := cmp.Compare(columnRank(string(a.Column)), columnRank(string(b.Column))); c != 0 {
		return c
	}
	if a.Column == board.Done {
		if c := b.Modified.Compare(a.Modified); c != 0 {
			return c
		}
		return cmp.Compare(a.Slug, b.Slug)
	}
	rank := func(t board.Ticket) int {
		if r, ok := priorityRank[t.Priority]; ok {
			return r
		}
		return len(priorityRank)
	}
	if c := cmp.Compare(rank(a), rank(b)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Created, b.Created); c != 0 {
		return c
	}
	return cmp.Compare(a.Slug, b.Slug)
}

func card(snapshot *index.Snapshot, project *board.Project, ticket board.Ticket, withSearch bool) Card {
	ref := board.Ref{Project: project.Name, Slug: ticket.Slug}
	blockedBy := reasons(project.Name, snapshot.Analysis.Blocked[ref])
	result := Card{
		Project: project.Name, Slug: ticket.Slug, Column: string(ticket.Column),
		Title: ticket.Title, Type: ticket.Type, Priority: ticket.Priority, Workstream: ticket.Workstream,
		Tags: nonNil(ticket.Tags), Created: ticket.Created, Updated: ticket.Updated,
		Modified: timestamp(ticket.Modified), Branch: ticket.Branch, DependsOn: nonNil(ticket.DependsOn),
		Excerpt: ticket.Excerpt, Attachments: len(project.Attachments[ticket.Slug]), HasReview: project.Reviews[ticket.Slug],
		Blocked: len(blockedBy) > 0, BlockedBy: blockedBy,
		NeedsRepair: nonNil(ticket.Repair),
		Warnings:    slices.Concat([]string{}, ticket.Warnings, snapshot.Analysis.Warnings[ref]),
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
			Kind: string(reason.Kind), Text: reason.Describe(viewing), Workstream: reason.Workstream,
			Column: string(reason.Column), Pending: reason.Pending, Missing: reason.Missing, Via: reason.Via,
		}
		if reason.Ticket.Slug != "" {
			ticket := reason.Ticket
			item.Ticket = &ticket
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
		Name: project.Name, DisplayName: project.DisplayName, Repos: nonNil(project.Repos),
		Counts: map[string]int{}, Warnings: nonNil(project.Warnings), LastModified: timestamp(project.LastModified),
	}
	for _, column := range board.Columns {
		result.Counts[string(column)] = 0
	}
	for _, ticket := range project.Tickets {
		result.Counts[string(ticket.Column)]++
		if ticket.NeedsRepair() {
			result.NeedsRepair++
		}
		if len(snapshot.Analysis.Blocked[board.Ref{Project: project.Name, Slug: ticket.Slug}]) > 0 {
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
