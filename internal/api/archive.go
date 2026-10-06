package api

import (
	"net/http"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/store"
)

// Archived tickets (EDIT-8): GET /api/projects/{project}/archive lists them;
// restoring is POST /api/tickets/{id}/unarchive. A ticket is deleted
// permanently only once archived: GET .../archive/{id} shows what the delete
// touches and POST .../archive/{id}/delete carries that preview's token and
// the typed id. There is no delete for agents.

// Archive is the store's archive side.
type Archive interface {
	ReadArchived(project string) ([]store.ArchivedTicket, error)
	PlanDelete(project, id string) (store.DeletePlan, error)
	DeleteArchived(project, id, token string) (store.DeletePlan, error)
}

// ArchivedJSON is one archived ticket in the archive view.
type ArchivedJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Priority   string `json:"priority"`
	Workstream string `json:"workstream"`
	// Column is where Restore returns it.
	Column string `json:"column"`
	// Archived is when it was archived: its last update.
	Archived    string `json:"archived"`
	Attachments int    `json:"attachments"`
	HasReview   bool   `json:"hasReview"`
}

// ArchiveResponse is GET /api/projects/{project}/archive.
type ArchiveResponse struct {
	Revision uint64         `json:"revision"`
	Tickets  []ArchivedJSON `json:"tickets"`
}

// ReferenceJSON is a ticket or workstream a delete rewrites.
type ReferenceJSON struct {
	Project string `json:"project,omitempty"`
	ID      string `json:"id,omitempty"`
	Slug    string `json:"slug,omitempty"`
	Title   string `json:"title"`
}

// DeletePlanJSON is GET /api/projects/{project}/archive/{id}.
type DeletePlanJSON struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Files       []string        `json:"files"`
	Tickets     []ReferenceJSON `json:"tickets"`
	Workstreams []ReferenceJSON `json:"workstreams"`
	Token       string          `json:"token"`
}

// DeleteRequest is POST /api/projects/{project}/archive/{id}/delete.
type DeleteRequest struct {
	Token string `json:"token"`
	// Confirm is the ticket id as the user typed it.
	Confirm string `json:"confirm"`
}

func (b boardAPI) registerArchive(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects/{project}/archive", b.archived)
	mux.HandleFunc("GET /api/projects/{project}/archive/{id}", b.deletePlan)
	mux.HandleFunc("POST /api/projects/{project}/archive/{id}/delete", b.deleteArchived)
}

// archiveSide returns the archive side, or answers 501 when this server cannot
// read or change archives.
func (b boardAPI) archiveSide(w http.ResponseWriter) Archive {
	archive, ok := b.write.(Archive)
	if b.write == nil || !ok {
		writeError(w, http.StatusNotImplemented, "read_only", "This server does not manage archived tickets")
		return nil
	}
	return archive
}

func (b boardAPI) archived(w http.ResponseWriter, r *http.Request) {
	snapshot, project := b.project(w, r)
	if project == nil {
		return
	}
	archive := b.archiveSide(w)
	if archive == nil {
		return
	}
	items, err := archive.ReadArchived(project.Name)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response := ArchiveResponse{Revision: snapshot.Revision, Tickets: []ArchivedJSON{}}
	for _, item := range items {
		archived := item.Ticket.Updated
		if archived == "" {
			archived = timestamp(item.Ticket.Modified)
		}
		response.Tickets = append(response.Tickets, ArchivedJSON{
			ID: item.Ticket.ID, Title: item.Ticket.Title, Type: item.Ticket.Type, Priority: item.Ticket.Priority,
			Workstream: item.Ticket.Workstream, Column: string(item.Ticket.Column), Archived: archived,
			Attachments: item.Attachments, HasReview: item.Review,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

// archivedID reads {id}, refusing anything but a ticket id.
func archivedID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if _, _, ok := board.ParseID(id); !ok {
		writeError(w, http.StatusBadRequest, "invalid_input", "Give an archived ticket's id")
		return "", false
	}
	return id, true
}

func (b boardAPI) deletePlan(w http.ResponseWriter, r *http.Request) {
	_, project := b.project(w, r)
	if project == nil {
		return
	}
	id, ok := archivedID(w, r)
	if !ok {
		return
	}
	archive := b.archiveSide(w)
	if archive == nil {
		return
	}
	plan, err := archive.PlanDelete(project.Name, id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response := DeletePlanJSON{ID: plan.ID, Files: nonNil(plan.Files), Tickets: []ReferenceJSON{}, Workstreams: []ReferenceJSON{}, Token: plan.Token}
	if items, err := archive.ReadArchived(project.Name); err == nil {
		for _, item := range items {
			if item.Ticket.ID == id {
				response.Title = item.Ticket.Title
			}
		}
	}
	for _, reference := range plan.Tickets {
		response.Tickets = append(response.Tickets, ReferenceJSON{Project: reference.Project, ID: reference.ID, Title: reference.Title})
	}
	for _, slug := range plan.Workstreams {
		title := slug
		for _, workstream := range project.Workstreams {
			if workstream.Slug == slug && workstream.Title != "" {
				title = workstream.Title
			}
		}
		response.Workstreams = append(response.Workstreams, ReferenceJSON{Slug: slug, Title: title})
	}
	writeJSON(w, http.StatusOK, response)
}

func (b boardAPI) deleteArchived(w http.ResponseWriter, r *http.Request) {
	archive := b.archiveSide(w)
	if archive == nil {
		return
	}
	_, project := b.project(w, r)
	if project == nil {
		return
	}
	id, ok := archivedID(w, r)
	if !ok {
		return
	}
	var request DeleteRequest
	if !decode(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Confirm) != id {
		writeError(w, http.StatusBadRequest, "invalid_input", "Type the ticket's id to delete it permanently")
		return
	}
	if _, err := archive.DeleteArchived(project.Name, id, request.Token); err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}
