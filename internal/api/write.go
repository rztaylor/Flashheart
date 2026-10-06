package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

// Writer is the store's write side (STO-3). Every ticket edit carries the
// content hash the client read; a stale hash is a 409 with the current file.
type Writer interface {
	ReadTicket(project, id string) ([]byte, string, error)
	UpdateTicket(project, id, base string, edit store.Edit) (string, error)
	UpdateWorkstream(project, slug, base string, edit store.Edit) (string, error)
	CreateTicket(project string, input store.NewTicket) (store.Created, error)
	SetProjectKey(project, key string) error
	Archive(project, id string) error
	Unarchive(project, id string) error
	ReadConfig() ([]byte, error)
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
	mux.HandleFunc("PUT /api/tickets/{id}/raw", b.putRaw)
	mux.HandleFunc("POST /api/tickets/{id}/criteria", b.setCriterion)
	mux.HandleFunc("POST /api/tickets/{id}/archive", b.archive)
	mux.HandleFunc("POST /api/tickets/{id}/unarchive", b.unarchive)
	mux.HandleFunc("POST /api/projects/{project}/tickets", b.createTicket)
	mux.HandleFunc("PUT /api/projects/{project}/workstreams/{slug}/order", b.reorderWorkstream)
	mux.HandleFunc("PUT /api/projects/{project}/key", b.setKey)
	mux.HandleFunc("GET /api/preferences", b.preferences)
	mux.HandleFunc("PUT /api/preferences", b.savePreferences)
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

// MoveRequest is POST /api/tickets/{id}/move (EDIT-1).
type MoveRequest struct {
	Base string `json:"base"`
	To   string `json:"to"`
	// Reason confirms moving a blocked ticket into In progress (EDIT-2).
	Reason string `json:"reason"`
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
	to, ok := board.ParseColumn(request.To)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%q is not a column", request.To))
		return
	}
	snapshot, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	reasons := snapshot.Analysis.Blocked[board.Ref{Project: project.Name, ID: ticket.ID}]
	reason := strings.TrimSpace(request.Reason)
	if to == board.InProgress && ticket.Column != board.InProgress && len(reasons) > 0 && reason == "" {
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
	hash, err := writer.UpdateTicket(project.Name, ticket.ID, request.Base, func(data []byte) ([]byte, error) {
		next, err := mdfile.SetScalar(data, "status", string(to))
		if err != nil || reason == "" || len(reasons) == 0 || to != board.InProgress {
			return next, err
		}
		return mdfile.AppendToSection(next, "Notes", "Started while blocked: "+oneLine(reason, 500))
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	var warnings []string
	if to == board.Review && ticket.Column != board.Review {
		if !project.Reviews[ticket.ID] {
			warnings = append(warnings, "There is no review file yet.")
		}
		open := 0
		for _, criterion := range ticket.Criteria {
			if !criterion.Done {
				open++
			}
		}
		if open > 0 {
			warnings = append(warnings, fmt.Sprintf("%d acceptance %s not ticked.", open, plural(open, "criterion is", "criteria are")))
		}
	}
	b.saved(w, http.StatusOK, hash, warnings)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// oneLine joins whitespace and keeps at most limit characters.
func oneLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if runes := []rune(value); len(runes) > limit {
		value = string(runes[:limit])
	}
	return value
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

var (
	listFields   = []string{"depends-on", "depends-on-workstreams", "tags"}
	scalarFields = []string{"title", "status", "type", "priority", "created", "branch", "workstream"}
	datePattern  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	slugPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,99}$`)
)

func validateField(key, value string) error {
	switch key {
	case "title":
		if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 300 {
			return errors.New("title must be 1 to 300 characters")
		}
	case "status":
		if _, ok := board.ParseColumn(value); !ok {
			return fmt.Errorf("%q is not a column", value)
		}
	case "type":
		if !slices.Contains(board.TicketTypes, value) {
			return fmt.Errorf("type must be one of %s", strings.Join(board.TicketTypes, ", "))
		}
	case "priority":
		if !slices.Contains(board.Priorities, value) {
			return fmt.Errorf("priority must be one of %s", strings.Join(board.Priorities, ", "))
		}
	case "created":
		if !datePattern.MatchString(value) {
			return errors.New("created must be a date like 2026-10-05")
		}
	case "workstream", "depends-on-workstreams":
		if value != "" && !slugPattern.MatchString(value) {
			return fmt.Errorf("%q is not a workstream slug", value)
		}
	case "depends-on":
		if _, _, ok := board.ParseID(value); !ok {
			return fmt.Errorf("%q is not a ticket id like FH-42", value)
		}
	case "branch", "tags":
		if strings.ContainsAny(value, "\n\r") || utf8.RuneCountInString(value) > 200 {
			return fmt.Errorf("%s must be one line of at most 200 characters", key)
		}
	}
	return nil
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
	type change struct {
		key    string
		value  string
		list   []string
		isList bool
	}
	var changes []change
	for key, raw := range request.Fields {
		c := change{key: key, isList: slices.Contains(listFields, key)}
		switch {
		case c.isList:
			if err := json.Unmarshal(raw, &c.list); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", key+" must be a list of strings")
				return
			}
			for index, item := range c.list {
				c.list[index] = strings.TrimSpace(item)
				if err := validateField(key, c.list[index]); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
					return
				}
			}
			c.list = slices.DeleteFunc(c.list, func(item string) bool { return item == "" })
		case slices.Contains(scalarFields, key):
			if err := json.Unmarshal(raw, &c.value); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", key+" must be a string")
				return
			}
			c.value = strings.TrimSpace(c.value)
			if err := validateField(key, c.value); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%q cannot be edited here; use the raw editor", key))
			return
		}
		changes = append(changes, c)
	}
	slices.SortFunc(changes, func(a, b change) int { return strings.Compare(a.key, b.key) })
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	hash, err := writer.UpdateTicket(project.Name, ticket.ID, request.Base, func(data []byte) ([]byte, error) {
		var err error
		for _, c := range changes {
			switch {
			case c.key == "title":
				data, err = mdfile.SetTitle(data, c.value)
			case c.isList:
				data, err = mdfile.SetList(data, c.key, c.list)
			default:
				data, err = mdfile.SetScalar(data, c.key, c.value)
			}
			if err != nil {
				return nil, err
			}
		}
		return data, nil
	})
	if err != nil {
		writeStoreError(w, err)
		return
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
	hash, err := writer.UpdateTicket(project.Name, ticket.ID, request.Base, func([]byte) ([]byte, error) {
		return []byte(request.Content), nil
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
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
	hash, err := writer.UpdateTicket(project.Name, ticket.ID, request.Base, func(data []byte) ([]byte, error) {
		return mdfile.SetCheckbox(data, "Acceptance Criteria", request.Index, request.Checked)
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
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
	if len(request.Description) > 64<<10 {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "The description is limited to 64 KiB")
		return
	}
	created, err := writer.CreateTicket(r.PathValue("project"), store.NewTicket{
		Title: oneLine(request.Title, 300), Type: request.Type, Priority: request.Priority,
		Status: board.Column(request.Status), Workstream: request.Workstream,
		Description: request.Description, Criteria: request.Criteria,
		DependsOn: request.DependsOn, Tags: request.Tags, Session: "flashheart-ui", Key: request.Key,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
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
	hash, err := writer.UpdateWorkstream(r.PathValue("project"), r.PathValue("slug"), "", func(data []byte) ([]byte, error) {
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
	// VirtualColumns lists the virtual columns shown (VIEW-2).
	VirtualColumns []string                `json:"virtualColumns"`
	Scopes         map[string]config.Scope `json:"scopes"`
}

func (b boardAPI) currentUI(w http.ResponseWriter) (config.UI, bool) {
	data, err := b.write.ReadConfig()
	if err == nil {
		var settings config.Config
		if settings, err = config.Parse(data); err == nil {
			return settings.UI, true
		}
	}
	writeError(w, http.StatusInternalServerError, "config_unreadable", "Settings could not be read: "+err.Error())
	return config.UI{}, false
}

func (b boardAPI) preferences(w http.ResponseWriter, _ *http.Request) {
	if b.writer(w) == nil {
		return
	}
	ui, ok := b.currentUI(w)
	if !ok {
		return
	}
	scopes := ui.Scopes
	if scopes == nil {
		scopes = map[string]config.Scope{}
	}
	writeJSON(w, http.StatusOK, Preferences{Theme: ui.Theme, Density: ui.Density, ColourBy: ui.ColourBy, VirtualColumns: nonNil(ui.VirtualColumns), Scopes: scopes})
}

func (b boardAPI) savePreferences(w http.ResponseWriter, r *http.Request) {
	if b.writer(w) == nil {
		return
	}
	var request Preferences
	if !decode(w, r, &request) {
		return
	}
	ui, ok := b.currentUI(w)
	if !ok {
		return
	}
	ui.Theme, ui.Density, ui.ColourBy, ui.Scopes = request.Theme, request.Density, request.ColourBy, request.Scopes
	ui.VirtualColumns = nonNil(request.VirtualColumns)
	if len(ui.Scopes) > 500 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Too many remembered views")
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
