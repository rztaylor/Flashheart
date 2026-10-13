package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

// Writer is the store's write side (STO-3), writing as the person at the
// board (store.As), so each ticket write appends its history and comments
// are theirs (D47). Every ticket edit carries the content hash the client
// read; a stale hash is a 409 with the current file.
type Writer interface {
	ReadTicket(project, id string) ([]byte, string, error)
	UpdateTicket(project, id string, pre store.Precondition, edit store.Edit) (string, error)
	EditTicket(project, id string, pre store.Precondition, changes board.FieldChanges, edit store.Edit) (string, error)
	UpdateReview(project, id string, pre store.Precondition, edit store.Edit) (string, error)
	PlaceTicket(project, id string, pre store.Precondition, after string, edit store.Edit) (string, bool, error)
	UpdateWorkstream(project, slug string, pre store.Precondition, edit store.Edit) (string, error)
	CreateTicket(project string, input store.NewTicket) (store.Created, error)
	SetProjectKey(project, key string) error
	Archive(project, id string) error
	Unarchive(project, id string) error
	AddComment(project, id string, pre store.Precondition, comment store.NewComment) (board.Comment, string, error)
	EditComment(project, id, comment string, pre store.Precondition, body string) (board.Comment, string, error)
	RequestChanges(project, id string, comments []string, added *store.NewComment) (store.RequestedChanges, error)
	SetCommentState(project, id, comment string, to board.CommentState) (board.Comment, string, error)
	UploadAttachment(project, id string, input store.Upload) (store.StoredFile, error)
	ReadConfig() (config.Config, error)
	UpdateConfig(edit store.Edit) error
}

// maxBody bounds request bodies; the largest is a raw ticket.
const maxBody = store.MaxFileBytes + 64<<10

// WriteResponse reports a saved ticket: its new hash and the board revision.
type WriteResponse struct {
	Hash     string   `json:"hash"`
	Revision uint64   `json:"revision"`
	Warnings []string `json:"warnings"`
}

// ConflictJSON is the current file returned with a 409 conflict (EDIT-7).
type ConflictJSON struct {
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

func (b boardAPI) registerWrites(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tickets/{id}/move", b.move)
	mux.HandleFunc("PATCH /api/tickets/{id}", b.patchTicket)
	mux.HandleFunc("PUT /api/tickets/{id}/section", b.putSection)
	mux.HandleFunc("PUT /api/tickets/{id}/raw", b.putRaw)
	mux.HandleFunc("POST /api/tickets/{id}/criteria", b.setCriterion)
	mux.HandleFunc("POST /api/tickets/{id}/review/steps", b.setReviewStep)
	mux.HandleFunc("POST /api/tickets/{id}/archive", b.archive)
	mux.HandleFunc("POST /api/tickets/{id}/unarchive", b.unarchive)
	mux.HandleFunc("POST /api/tickets/{id}/files", b.upload)
	mux.HandleFunc("POST /api/projects/{project}/tickets", b.createTicket)
	mux.HandleFunc("PUT /api/projects/{project}/workstreams/{slug}/order", b.reorderWorkstream)
	mux.HandleFunc("PUT /api/projects/{project}/key", b.setKey)
	mux.HandleFunc("POST /api/questions/{id}/answer", b.answer)
	mux.HandleFunc("GET /api/preferences", b.preferences)
	mux.HandleFunc("PUT /api/preferences", b.savePreferences)
	b.registerComments(mux)
}

// decode reads a bounded JSON body into v, rejecting unknown fields.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "The request body is not valid: "+err.Error())
		return false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_input", "The request body has trailing data")
		return false
	}
	return true
}

// writeStoreError maps store errors to HTTP responses.
func writeStoreError(w http.ResponseWriter, err error) {
	var conflict *store.ConflictError
	var taken *store.KeyTakenError
	switch {
	case errors.As(err, &conflict):
		type body struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		message := "This ticket changed since you opened it."
		if conflict.Name == "workstream" || strings.Contains(conflict.Name, "/workstreams/") {
			message = "This workstream's tickets changed since you opened it."
		}
		if conflict.Name == "delete" {
			message = "Something this delete touches changed since you looked. Review it again."
		}
		if path.Base(conflict.Name) == board.ActivityFile {
			message = "This ticket's comments changed since you opened them."
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   body{Code: "conflict", Message: message},
			"current": ConflictJSON{Hash: conflict.Hash, Content: string(conflict.Current)},
		})
	case errors.As(err, &taken):
		type body struct {
			Code    string   `json:"code"`
			Message string   `json:"message"`
			InUse   []string `json:"inUse"`
		}
		writeJSON(w, http.StatusConflict, map[string]body{"error": {Code: "key_taken", Message: taken.Error(), InUse: taken.InUse}})
	case errors.Is(err, store.ErrExists):
		writeError(w, http.StatusConflict, "exists", strings.TrimPrefix(err.Error(), store.ErrExists.Error()+": "))
	case errors.Is(err, store.ErrNotInReview):
		writeError(w, http.StatusConflict, "not_in_review", "This ticket is no longer in review, so changes cannot be requested on it.")
	case errors.Is(err, store.ErrKeyFixed):
		writeError(w, http.StatusConflict, "key_fixed", err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, store.ErrInvalidName), errors.Is(err, store.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, mdfile.ErrBrokenFrontmatter):
		writeError(w, http.StatusUnprocessableEntity, "needs_repair", "The ticket's frontmatter does not parse; fix it in the raw editor first.")
	case errors.Is(err, mdfile.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, store.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", err.Error())
	case errors.Is(err, store.ErrBusy):
		writeError(w, http.StatusServiceUnavailable, "busy", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "write_failed", "The change could not be saved: "+err.Error())
	}
}

// writer returns the write side, or answers 501 when this server is read-only.
func (b boardAPI) writer(w http.ResponseWriter) Writer {
	if b.write == nil {
		writeError(w, http.StatusNotImplemented, "read_only", "This server does not save changes")
	}
	return b.write
}

// saved rebuilds the snapshot so the response carries the new revision.
func (b boardAPI) saved(w http.ResponseWriter, status int, hash string, warnings []string) {
	revision := uint64(0)
	if snapshot, _ := b.board.Rebuild(); snapshot != nil {
		revision = snapshot.Revision
	}
	writeJSON(w, status, WriteResponse{Hash: hash, Revision: revision, Warnings: nonNil(warnings)})
}

// recordHuman records that the human changed a ticket on the board: its id
// and at most the columns or field names, never their values (agent-protocol
// §3). The ticket is already saved and the board files are the source of
// truth, so a failure to record does not fail the write.
func (b boardAPI) recordHuman(project string, kind events.Kind, data events.TicketData) {
	if b.events != nil {
		_ = b.events.Append(events.ByHuman(b.now(), project, kind, data))
	}
}

// MoveRequest is POST /api/tickets/{id}/move (EDIT-1).
type MoveRequest struct {
	Base string `json:"base"`
	// To is the new status: a column, or rejected (Done, FH-58).
	To string `json:"to"`
	// Reason confirms moving a blocked ticket into In progress (EDIT-2).
	Reason string `json:"reason"`
	// After places the ticket directly after this ticket in its new column
	// ("" for the top; EDIT-9). Absent, the ticket keeps its rank.
	After *string `json:"after,omitempty"`
}

func (b boardAPI) move(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request MoveRequest
	if !decode(w, r, &request) {
		return
	}
	to, _, ok := board.ParseStatus(request.To)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%q is not a status", request.To))
		return
	}
	if request.After != nil && *request.After != "" {
		if _, _, ok := board.ParseID(*request.After); !ok {
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%q is not a ticket id", *request.After))
			return
		}
	}
	snapshot, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	reasons := snapshot.Analysis.Blocked[board.Ref{Project: project.Name, ID: ticket.ID}]
	reason := strings.TrimSpace(request.Reason)
	startsBlocked := board.StartsBlocked(ticket.Column, to, len(reasons) > 0)
	if startsBlocked && reason == "" {
		var described []string
		for _, item := range reasons {
			described = append(described, item.Describe())
		}
		type body struct {
			Code      string   `json:"code"`
			Message   string   `json:"message"`
			BlockedBy []string `json:"blockedBy"`
		}
		writeJSON(w, http.StatusConflict, map[string]body{"error": {
			Code: "confirm_blocked", Message: "This ticket is blocked. Give a reason to start it anyway.", BlockedBy: described,
		}})
		return
	}
	edit := func(data []byte) ([]byte, error) {
		next, err := mdfile.SetScalar(data, "status", request.To)
		if err != nil || !startsBlocked {
			return next, err
		}
		return mdfile.AppendToSection(next, board.SectionNotes, board.BlockedStartNote(b.now(), reason))
	}
	// A move sets only status (and a note), so without a hash it merges
	// with the file as it is then (STO-3); with one it must still match.
	pre := store.Merge
	if request.Base != "" {
		pre = store.Unchanged(request.Base)
	}
	var warnings []string
	var hash string
	var err error
	if request.After != nil && to != board.Done {
		var placed bool
		hash, placed, err = writer.PlaceTicket(project.Name, ticket.ID, pre, *request.After, edit)
		if err == nil && !placed {
			warnings = append(warnings, fmt.Sprintf("%s is no longer in %s, so %s kept its place.", *request.After, to.Title(), ticket.ID))
		}
	} else {
		hash, err = writer.UpdateTicket(project.Name, ticket.ID, pre, edit)
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.TicketMoved, events.TicketData{Ticket: ticket.ID, From: ticket.Status(), To: request.To})
	if to == board.Review && ticket.Column != board.Review {
		review := ""
		var requests []board.Comment
		if b.files != nil {
			if project.Reviews[ticket.ID] {
				review, _, _ = b.files.ReadReview(project.Name, ticket.Folder)
			}
			if activity, _, err := b.files.ReadTicketActivity(project.Name, ticket.Folder); err == nil {
				requests = board.OpenRequests(activity.Comments)
			}
		}
		warnings = append(warnings, board.ReviewWarnings(ticket, review, project.Reviews[ticket.ID], requests)...)
	}
	b.saved(w, http.StatusOK, hash, warnings)
}

// requireBase refuses an edit without the hash it was based on (STO-3).
func requireBase(w http.ResponseWriter, base string) bool {
	if base == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Edits must send the hash of the ticket they were based on")
		return false
	}
	return true
}

// PatchRequest is PATCH /api/tickets/{id}: typed frontmatter fields and the
// title (EDIT-6). Values are strings or lists of strings.
type PatchRequest struct {
	Base   string                     `json:"base"`
	Fields map[string]json.RawMessage `json:"fields"`
}

func (b boardAPI) patchTicket(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request PatchRequest
	if !decode(w, r, &request) || !requireBase(w, request.Base) {
		return
	}
	// Decoding is the API's; which fields change and how they are checked
	// and written is the one rule agents share (board.NewFieldChanges,
	// store.EditTicket; FH-85).
	var requested []board.FieldChange
	for key, raw := range request.Fields {
		list, editable := board.EditableField(key)
		change := board.FieldChange{Key: key}
		switch {
		case !editable:
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%q cannot be edited here; use the raw editor", key))
			return
		case list:
			if err := json.Unmarshal(raw, &change.List); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", key+" must be a list of strings")
				return
			}
		default:
			if err := json.Unmarshal(raw, &change.Value); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", key+" must be a string")
				return
			}
		}
		requested = append(requested, change)
	}
	changes, err := board.NewFieldChanges(requested)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	// A workstream change also moves the ticket between the workstreams'
	// tickets lists, in the same write under the same base hash, so a stale
	// base changes no file (board-format.md, Workstream membership).
	hash, err := writer.EditTicket(project.Name, ticket.ID, store.Unchanged(request.Base), changes, nil)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID, Fields: changes.Keys()})
	b.saved(w, http.StatusOK, hash, nil)
}

// SectionRequest is PUT /api/tickets/{id}/section: one section's text,
// edited in place on the ticket's page (FH-66). A missing section is
// added in the template's place; Handoff and Notes are refused.
type SectionRequest struct {
	Base    string `json:"base"`
	Section string `json:"section"`
	Content string `json:"content"`
}

func (b boardAPI) putSection(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request SectionRequest
	if !decode(w, r, &request) || !requireBase(w, request.Base) {
		return
	}
	// Which sections may be rewritten, and where a missing one goes, is
	// the one rule agents will share (board.SectionChange).
	change, err := board.NewSectionChange(request.Section, request.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	hash, err := writer.EditTicket(project.Name, ticket.ID, store.Unchanged(request.Base), board.FieldChanges{}, change.Apply)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// The section's own text again writes nothing, so there is nothing to
	// record.
	if hash != request.Base {
		b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID, Fields: []string{change.Field()}})
	}
	b.saved(w, http.StatusOK, hash, nil)
}

// RawRequest is PUT /api/tickets/{id}/raw: the whole file (EDIT-6).
type RawRequest struct {
	Base    string `json:"base"`
	Content string `json:"content"`
}

func (b boardAPI) putRaw(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request RawRequest
	if !decode(w, r, &request) || !requireBase(w, request.Base) {
		return
	}
	if len(request.Content) > store.MaxFileBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "A ticket file is limited to 1 MiB")
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	var warnings []string
	if doc := mdfile.Parse([]byte(request.Content)); doc.FrontmatterError != nil {
		warnings = append(warnings, "The frontmatter does not parse, so the ticket now needs repair: "+doc.FrontmatterError.Error())
	}
	hash, err := writer.UpdateTicket(project.Name, ticket.ID, store.Unchanged(request.Base), func([]byte) ([]byte, error) {
		return []byte(request.Content), nil
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// A raw edit names no fields: what it changed is not worked out.
	b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID})
	b.saved(w, http.StatusOK, hash, warnings)
}

// CriterionRequest is POST /api/tickets/{id}/criteria (CARD-3).
type CriterionRequest struct {
	Base    string `json:"base"`
	Index   int    `json:"index"`
	Checked bool   `json:"checked"`
}

func (b boardAPI) setCriterion(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request CriterionRequest
	if !decode(w, r, &request) || !requireBase(w, request.Base) {
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	hash, err := writer.UpdateTicket(project.Name, ticket.ID, store.Unchanged(request.Base), func(data []byte) ([]byte, error) {
		return mdfile.SetCheckbox(data, board.SectionCriteria, request.Index, request.Checked)
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID, Fields: []string{"criteria"}})
	b.saved(w, http.StatusOK, hash, nil)
}

func (b boardAPI) archive(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	if err := writer.Archive(project.Name, ticket.ID); err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID, Fields: []string{"archived"}})
	b.saved(w, http.StatusOK, "", nil)
}

func (b boardAPI) unarchive(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	snapshot := b.snapshot(w)
	if snapshot == nil {
		return
	}
	id := r.PathValue("id")
	key, _, ok := board.ParseID(id)
	if !ok || !snapshot.Archived(id) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No archived ticket %s", id))
		return
	}
	for _, project := range snapshot.Board.Projects {
		if project.Key == key && slices.Contains(project.Archived, id) {
			if err := writer.Unarchive(project.Name, id); err != nil {
				writeStoreError(w, err)
				return
			}
			b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: id, Fields: []string{"archived"}})
			b.saved(w, http.StatusOK, "", nil)
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("No archived ticket %s", id))
}

// CreateRequest is POST /api/projects/{project}/tickets (EDIT-5).
type CreateRequest struct {
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Status      string   `json:"status"`
	Workstream  string   `json:"workstream"`
	Description string   `json:"description"`
	Criteria    []string `json:"criteria"`
	DependsOn   []string `json:"dependsOn"`
	Tags        []string `json:"tags"`
	// Key is used only when the project has no key yet (KEY-5).
	Key string `json:"key"`
}

// CreateResponse reports a new ticket.
type CreateResponse struct {
	ID       string `json:"id"`
	Hash     string `json:"hash"`
	Key      string `json:"key"`
	Revision uint64 `json:"revision"`
}

func (b boardAPI) createTicket(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request CreateRequest
	if !decode(w, r, &request) {
		return
	}
	// The board and agents share one limit (FH-81); the heading demotion
	// that keeps the description in its section is board.RenderTicket's.
	if len(request.Description) > board.MaxSectionText {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("The description is limited to %d KiB", board.MaxSectionText>>10))
		return
	}
	created, err := writer.CreateTicket(r.PathValue("project"), store.NewTicket{
		Title: board.OneLine(request.Title, board.MaxTitle), Type: request.Type, Priority: request.Priority,
		Status: board.Column(request.Status), Workstream: request.Workstream,
		Description: request.Description, Criteria: request.Criteria,
		DependsOn: request.DependsOn, Tags: request.Tags, Session: "flashheart-ui", Key: request.Key,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(r.PathValue("project"), events.TicketCreated, events.TicketData{Ticket: created.ID})
	revision := uint64(0)
	if snapshot, _ := b.board.Rebuild(); snapshot != nil {
		revision = snapshot.Revision
	}
	writeJSON(w, http.StatusCreated, CreateResponse{ID: created.ID, Hash: created.Hash, Key: created.Key, Revision: revision})
}

// OrderRequest is PUT /api/projects/{project}/workstreams/{slug}/order: the
// workstream's tickets in their new order (EDIT-4).
type OrderRequest struct {
	Tickets []string `json:"tickets"`
}

func (b boardAPI) reorderWorkstream(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request OrderRequest
	if !decode(w, r, &request) {
		return
	}
	hash, err := writer.UpdateWorkstream(r.PathValue("project"), r.PathValue("slug"), store.Merge, func(data []byte) ([]byte, error) {
		current := mdfile.Parse(data).List("tickets")
		// Only a reordering is allowed; any other difference means the list
		// changed since the client read it.
		if !slices.Equal(sorted(current), sorted(request.Tickets)) {
			return nil, &store.ConflictError{Name: "workstream", Hash: store.Hash(data), Current: data}
		}
		return mdfile.SetList(data, "tickets", request.Tickets)
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, hash, nil)
}

func sorted(items []string) []string {
	out := slices.Clone(items)
	slices.Sort(out)
	return out
}

// KeyRequest is PUT /api/projects/{project}/key (KEY-5).
type KeyRequest struct {
	Key string `json:"key"`
}

func (b boardAPI) setKey(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request KeyRequest
	if !decode(w, r, &request) {
		return
	}
	if err := writer.SetProjectKey(r.PathValue("project"), strings.TrimSpace(request.Key)); err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}

// Preferences are the UI preferences saved in config.yaml (CFG-2).
type Preferences struct {
	Theme    string `json:"theme"`
	Density  string `json:"density"`
	ColourBy string `json:"colourBy"`
	// HiddenColumns lists the real columns hidden from the Board (FH-41).
	HiddenColumns []string `json:"hiddenColumns"`
	// Editor is how the section editor opens: formatted or markdown (FH-16).
	Editor string                  `json:"editor"`
	Scopes map[string]config.Scope `json:"scopes"`
	// ExpandedWorkstreams lists per project the finished workstreams shown
	// expanded (FH-68).
	ExpandedWorkstreams map[string][]string `json:"expandedWorkstreams"`
}

func (b boardAPI) currentUI(w http.ResponseWriter) (config.UI, bool) {
	settings, err := b.write.ReadConfig()
	if err == nil {
		return settings.UI, true
	}
	writeError(w, http.StatusInternalServerError, "config_unreadable", "Settings could not be read: "+err.Error())
	return config.UI{}, false
}

func (b boardAPI) preferences(w http.ResponseWriter, _ *http.Request) {
	if b.writer(w) == nil {
		return
	}
	b.preferring.RLock()
	defer b.preferring.RUnlock()
	ui, ok := b.currentUI(w)
	if !ok {
		return
	}
	scopes := ui.Scopes
	if scopes == nil {
		scopes = map[string]config.Scope{}
	}
	expanded := ui.ExpandedWorkstreams
	if expanded == nil {
		expanded = map[string][]string{}
	}
	writeJSON(w, http.StatusOK, Preferences{Theme: ui.Theme, Density: ui.Density, ColourBy: ui.ColourBy, HiddenColumns: nonNil(ui.HiddenColumns), Editor: ui.Editor, Scopes: scopes, ExpandedWorkstreams: expanded})
}

// savePreferences holds back preference reads until it answers: the save
// a page flushes as it unloads reaches the server before the reloaded page
// asks for preferences, but may take longer to write than that page takes to
// ask (FH-109).
func (b boardAPI) savePreferences(w http.ResponseWriter, r *http.Request) {
	if b.writer(w) == nil {
		return
	}
	b.preferring.Lock()
	defer b.preferring.Unlock()
	var request Preferences
	if !decode(w, r, &request) {
		return
	}
	ui, ok := b.currentUI(w)
	if !ok {
		return
	}
	ui.Theme, ui.Density, ui.ColourBy, ui.Scopes = request.Theme, request.Density, request.ColourBy, request.Scopes
	ui.HiddenColumns = nonNil(request.HiddenColumns)
	ui.Editor = request.Editor
	ui.ExpandedWorkstreams = request.ExpandedWorkstreams
	if len(ui.Scopes) > 500 || len(ui.ExpandedWorkstreams) > 500 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Too many remembered views or workstreams")
		return
	}
	// Validate before taking the lock, so only real write failures remain.
	if _, err := config.SetUI(nil, ui); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	err := b.write.UpdateConfig(func(data []byte) ([]byte, error) { return config.SetUI(data, ui) })
	if err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReviewStepRequest is POST /api/tickets/{id}/review/steps: tick or clear
// How to Verify step Index (from 0) of the review read at Base (REV-3).
type ReviewStepRequest struct {
	Base    string `json:"base"`
	Index   int    `json:"index"`
	Checked bool   `json:"checked"`
}

func (b boardAPI) setReviewStep(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	var request ReviewStepRequest
	if !decode(w, r, &request) || !requireBase(w, request.Base) {
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	hash, err := writer.UpdateReview(project.Name, ticket.ID, store.Unchanged(request.Base), func(data []byte) ([]byte, error) {
		next, err := board.SetReviewStep(string(data), request.Index, request.Checked)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", store.ErrInvalidInput, err)
		}
		return []byte(next), nil
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID, Fields: []string{"review"}})
	b.saved(w, http.StatusOK, hash, nil)
}
