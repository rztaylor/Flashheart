package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Info is the GET /api/info response.
type Info struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	BuildDate       string `json:"buildDate"`
	ProtocolVersion int    `json:"protocolVersion"`
	Root            string `json:"root"`
	Theme           string `json:"theme"`
}

// Options configures the API handler. Board endpoints are served when Board
// is set.
type Options struct {
	Info  Info
	Board BoardSource
	Files FileSource
	// DoneLimit is how many done tickets a board shows by default (VIEW-1);
	// zero shows all.
	DoneLimit int
	// Stopping is closed when the server starts shutting down; it releases
	// long-polls so they never hold up the drain.
	Stopping <-chan struct{}
	// LongPoll bounds how long GET /api/changes waits (default 20s).
	LongPoll time.Duration
}

// New returns the handler for every /api/ route.
func New(options Options) http.Handler {
	info := options.Info
	info.Name = "Flashheart"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET for /api/info")
			return
		}
		writeJSON(w, http.StatusOK, info)
	})
	if options.Board != nil {
		if options.LongPoll <= 0 {
			options.LongPoll = 20 * time.Second
		}
		boardAPI{
			board: options.Board, files: options.Files, root: info.Root, doneLimit: options.DoneLimit,
			stopping: options.Stopping, longPoll: options.LongPoll,
		}.register(mux)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "No such API endpoint")
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	type body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	writeJSON(w, status, map[string]body{"error": {Code: code, Message: message}})
}
