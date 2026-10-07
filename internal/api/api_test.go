package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	handler := New(Options{Info: Info{
		Version: "v0.0.1", Commit: "abc1234", BuildDate: "2026-10-04",
		ProtocolVersion: 1, Root: "/srv/board", Theme: "dark",
	}})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

func TestInfoDescribesTheRunningServer(t *testing.T) {
	t.Parallel()

	recorder := serve(t, http.MethodGet, "/api/info")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := map[string]any{
		"name": "Flashheart", "version": "v0.0.1", "commit": "abc1234", "buildDate": "2026-10-04",
		"protocolVersion": float64(1), "root": "/srv/board", "theme": "dark",
		// A server without a writer cannot record answers (FH-8).
		"answers": false,
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("%s = %#v, want %#v", key, body[key], value)
		}
	}
	if len(body) != len(want) {
		t.Errorf("info has unexpected fields: %v", body)
	}
}

func TestErrorsAreJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/api/nothing", http.StatusNotFound, "not_found"},
		{http.MethodPost, "/api/info", http.StatusMethodNotAllowed, "method_not_allowed"},
	}
	for _, test := range tests {
		recorder := serve(t, test.method, test.path)
		if recorder.Code != test.status {
			t.Errorf("%s %s status = %d, want %d", test.method, test.path, recorder.Code, test.status)
		}
		var body struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: decode %q: %v", test.method, test.path, recorder.Body.String(), err)
		}
		if body.Error.Code != test.code || body.Error.Message == "" {
			t.Errorf("%s %s error = %+v", test.method, test.path, body.Error)
		}
	}
}
