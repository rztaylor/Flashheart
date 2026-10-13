package api

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/events"
	"github.com/rztaylor/flashheart/internal/store"
)

// UploadResponse is POST /api/tickets/{id}/files: the stored file, what
// it was judged to be, and the board revision that lists it.
type UploadResponse struct {
	// File is the name under the ticket's files/; the ticket links it as
	// files/<File>.
	File string `json:"file"`
	Kind string `json:"kind"`
	// Image says the file shows as an image (REV-2's PNG, JPEG, GIF, WebP).
	Image bool `json:"image"`
	// Reused says the ticket already had these bytes, under File.
	Reused   bool   `json:"reused"`
	Revision uint64 `json:"revision"`
}

// upload stores the request body in the ticket's files/ (FH-18, REV-2):
// name, and optionally kind and caption, are query parameters, and the
// body is the file's bytes. A declared length over the limit is refused
// before reading; otherwise the store reads no further than the limit.
// The ticket's own file is not written: the editor saves its reference
// through the normal section save.
func (b boardAPI) upload(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	query := r.URL.Query()
	name := strings.TrimSpace(query.Get("name"))
	kind := query.Get("kind")
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "An upload needs the file's name")
		return
	}
	if kind != "" && !slices.Contains(board.AttachmentKinds, kind) {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("%q is not a kind of attachment (%s)", kind, strings.Join(board.AttachmentKinds, ", ")))
		return
	}
	_, project, ticket, ok := b.findTicket(w, r)
	if !ok {
		return
	}
	settings, err := writer.ReadConfig()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	limit := settings.Attachments.MaxBytes
	if limit <= 0 || limit > store.DefaultMaxFileCopy {
		limit = store.DefaultMaxFileCopy
	}
	if r.ContentLength > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("%s is larger than %s, the limit for an attachment", name, board.SizeWords(limit)))
		return
	}
	stored, err := writer.UploadAttachment(project.Name, ticket.ID, store.Upload{
		Name: name, Body: http.MaxBytesReader(w, r.Body, limit+1), Caption: query.Get("caption"), Kind: kind, MaxBytes: limit,
	})
	if err != nil {
		if errors.Is(err, store.ErrTypeNotAllowed) {
			writeError(w, http.StatusUnsupportedMediaType, "type_not_allowed",
				strings.TrimSuffix(err.Error(), ": "+store.ErrTypeNotAllowed.Error())+". Attach PNG, JPEG, GIF, WebP, PDF, text, markdown, JSON or log files.")
			return
		}
		if errors.Is(err, store.ErrTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", strings.TrimSuffix(err.Error(), ": "+store.ErrTooLarge.Error()))
			return
		}
		writeStoreError(w, err)
		return
	}
	if !stored.Reused {
		b.recordHuman(project.Name, events.TicketUpdated, events.TicketData{Ticket: ticket.ID, Fields: []string{"files"}})
	}
	contentType, _ := board.AttachmentType(stored.File)
	revision := uint64(0)
	if snapshot, _ := b.board.Rebuild(); snapshot != nil {
		revision = snapshot.Revision
	}
	writeJSON(w, http.StatusCreated, UploadResponse{
		File: stored.File, Kind: board.AttachmentKindFor(kind, stored.File),
		Image: strings.HasPrefix(contentType, "image/"), Reused: stored.Reused, Revision: revision,
	})
}
