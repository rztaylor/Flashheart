package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/store"
)

// A project's image (FH-151): the picture a person uploads for its badge,
// stored by the store as a 128×128 PNG in the project folder and named in
// project.yaml. It is served through the authenticated API like a ticket's
// files, never as a public path; the projects list carries each project's
// image URL, with a version that changes with its bytes.

// projectImageURL is the URL the browser loads a project's image from, or
// "" when it has none to show.
func projectImageURL(project *board.Project) string {
	if project.Image == "" {
		return ""
	}
	return "/api/projects/" + url.PathEscape(project.Name) + "/image?v=" + project.ImageVersion
}

// projectImage serves the image project.yaml names (SEC-4 headers, as an
// attachment), or 404 when the project has none to show.
func (b boardAPI) projectImage(w http.ResponseWriter, r *http.Request) {
	if b.files == nil {
		writeError(w, http.StatusNotFound, "not_found", "Project images are unavailable")
		return
	}
	_, project := b.project(w, r)
	if project == nil {
		return
	}
	if project.Image == "" {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("%s has no image", project.DisplayName))
		return
	}
	file, contentType, err := b.files.OpenProjectImage(project.Name, project.Image)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("%s's image could not be read", project.DisplayName))
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unreadable", "The image could not be read")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, "", info.ModTime(), file)
}

// putProjectImage takes the request body as the project's new image. A
// declared length over the limit is refused before reading; otherwise no
// more than the limit is read. What is not a JPEG or PNG by its bytes, or
// cannot be decoded, is refused in words with nothing written (SEC-7).
func (b boardAPI) putProjectImage(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	_, project := b.project(w, r)
	if project == nil {
		return
	}
	limit := store.MaxProjectImageBytes
	if r.ContentLength > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("The image is larger than %s, the limit for a project image", board.SizeWords(limit)))
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit+1))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("The image is larger than %s, the limit for a project image", board.SizeWords(limit)))
		return
	}
	if err := writer.SetProjectImage(project.Name, data); err != nil {
		// The store's reason, in words, as a sentence of its own.
		reason := func(sentinel error) string {
			return "The image was not saved because " + strings.TrimPrefix(strings.TrimSuffix(err.Error(), ": "+sentinel.Error()), "the image was not saved because ")
		}
		if errors.Is(err, store.ErrTypeNotAllowed) {
			writeError(w, http.StatusUnsupportedMediaType, "type_not_allowed", reason(store.ErrTypeNotAllowed)+". Choose a JPEG or PNG image.")
			return
		}
		if errors.Is(err, store.ErrTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", reason(store.ErrTooLarge)+".")
			return
		}
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}

// deleteProjectImage removes the project's image and its field.
func (b boardAPI) deleteProjectImage(w http.ResponseWriter, r *http.Request) {
	writer := b.writer(w)
	if writer == nil {
		return
	}
	_, project := b.project(w, r)
	if project == nil {
		return
	}
	if err := writer.ClearProjectImage(project.Name); err != nil {
		writeStoreError(w, err)
		return
	}
	b.saved(w, http.StatusOK, "", nil)
}
