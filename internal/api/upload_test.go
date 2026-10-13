package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/events"
)

const uploadPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"

// upload posts body to the ticket's files endpoint as the board does.
func upload(t *testing.T, handler http.Handler, id, query string, body io.Reader, length int64) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/tickets/"+id+"/files?"+query, body)
	request.ContentLength = length
	request.Header.Set("Content-Type", "application/octet-stream")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// POST /api/tickets/{id}/files stores the bytes as what they are (FH-18):
// a PNG named .txt is a PNG, served back with the SEC-4 headers, listed
// with the ticket, and the human's event names the ticket and no content.
// The ticket's own file is not rewritten.
func TestUploadStoresAFileFromTheBrowser(t *testing.T) {
	t.Parallel()

	handler, log := activityAPI(t)
	before := ticketDetail(t, handler, "AL-4")
	response := upload(t, handler, "AL-4", "name=notes.txt&kind=screenshot", strings.NewReader(uploadPNG), int64(len(uploadPNG)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
	var uploaded UploadResponse
	if err := json.Unmarshal(response.Body.Bytes(), &uploaded); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(uploaded.File, "-notes.png") || !uploaded.Image || uploaded.Kind != "screenshot" || uploaded.Reused || uploaded.Revision == 0 {
		t.Fatalf("uploaded = %+v", uploaded)
	}

	after := ticketDetail(t, handler, "AL-4")
	if after.Hash != before.Hash {
		t.Errorf("an upload rewrote the ticket")
	}
	if len(after.AttachmentFiles) != 1 || after.AttachmentFiles[0].File != uploaded.File || after.AttachmentFiles[0].Caption != "notes.txt" {
		t.Fatalf("attachments = %+v", after.AttachmentFiles)
	}
	served := httptest.NewRecorder()
	handler.ServeHTTP(served, httptest.NewRequest(http.MethodGet, after.AttachmentFiles[0].URL, nil))
	if served.Code != http.StatusOK || served.Header().Get("Content-Type") != "image/png" ||
		served.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(served.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("served %d %v", served.Code, served.Header())
	}

	var human []events.Event
	for _, e := range boardLog(t, log) {
		if e.Run == events.Human {
			human = append(human, e)
		}
	}
	if len(human) != 1 || human[0].Kind != events.TicketUpdated {
		t.Fatalf("human events = %+v", human)
	}
	var data events.TicketData
	_ = human[0].Decode(&data)
	if data.Ticket != "AL-4" || len(data.Fields) != 1 || data.Fields[0] != "files" {
		t.Errorf("event data = %+v", data)
	}

	// A file other than an image is a file, not a screenshot.
	response = upload(t, handler, "AL-4", "name=spec.pdf", strings.NewReader("%PDF-1.7\n"), -1)
	if err := json.Unmarshal(response.Body.Bytes(), &uploaded); err != nil || response.Code != http.StatusCreated || uploaded.Image || uploaded.Kind != "other" || !strings.HasSuffix(uploaded.File, "-spec.pdf") {
		t.Errorf("pdf = %d %+v", response.Code, uploaded)
	}
}

// Refusals say why in words, write nothing and record nothing: markup by
// content, a body over the limit (by its length, or as it streams), a
// missing name or ticket, a read-only server.
func TestUploadRefusals(t *testing.T) {
	t.Parallel()

	handler, root := writableAPI(t)
	dir := filepath.Join(root, "alpha", "tickets", "AL-4-drag-and-drop", "files")
	for _, tc := range []struct {
		label, id, query, body string
		length                 int64
		status                 int
		code, words            string
	}{
		{"svg named png", "AL-4", "name=drawing.png", `<svg xmlns="http://www.w3.org/2000/svg"/>`, -1, http.StatusUnsupportedMediaType, "type_not_allowed", "SVG"},
		{"html", "AL-4", "name=page.html", "<html><body>", -1, http.StatusUnsupportedMediaType, "type_not_allowed", "HTML"},
		{"declared too large", "AL-4", "name=big.png", uploadPNG, 21 << 20, http.StatusRequestEntityTooLarge, "too_large", "20 MB"},
		{"no name", "AL-4", "", uploadPNG, -1, http.StatusBadRequest, "invalid_input", "name"},
		{"bad kind", "AL-4", "name=a.png&kind=evil", uploadPNG, -1, http.StatusBadRequest, "invalid_input", "kind"},
		{"no ticket", "AL-99", "name=a.png", uploadPNG, -1, http.StatusNotFound, "not_found", "AL-99"},
	} {
		response := upload(t, handler, tc.id, tc.query, strings.NewReader(tc.body), tc.length)
		var body struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		if response.Code != tc.status || body.Error.Code != tc.code || !strings.Contains(body.Error.Message, tc.words) {
			t.Errorf("%s: %d %+v, want %d %s naming %q", tc.label, response.Code, body.Error, tc.status, tc.code, tc.words)
		}
	}

	// A body that streams past the limit with no length given is cut off
	// there, not read whole first.
	streamed := &readCounter{r: io.MultiReader(strings.NewReader(uploadPNG), io.LimitReader(zeroReader{}, 40<<20))}
	if response := upload(t, handler, "AL-4", "name=huge.png", streamed, -1); response.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("streamed: %d %s", response.Code, response.Body)
	}
	if streamed.read > 21<<20 {
		t.Errorf("read %d bytes of an oversized body", streamed.read)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Errorf("refusals left files/: %v", entries)
	}

	readOnlyHandler, _ := sampleAPI(t, nil)
	readOnly := upload(t, readOnlyHandler, "AL-4", "name=a.png", strings.NewReader(uploadPNG), -1)
	if readOnly.Code != http.StatusNotImplemented {
		t.Errorf("read-only upload status = %d", readOnly.Code)
	}
}

type readCounter struct {
	r    io.Reader
	read int64
}

func (c *readCounter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read += int64(n)
	return n, err
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
