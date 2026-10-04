package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestServesCompiledIndexWithoutCaching(t *testing.T) {
	t.Parallel()

	handler, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	recorder := get(t, handler, "/")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "<title>Flashheart</title>") {
		t.Error("compiled index does not contain the Flashheart title")
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestHashedAssetsAreImmutable(t *testing.T) {
	t.Parallel()

	handler, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	index := get(t, handler, "/").Body.String()
	start := strings.Index(index, `src="/assets/`)
	if start < 0 {
		t.Fatalf("index has no hashed script:\n%s", index)
	}
	path := index[start+len(`src="`):]
	path = path[:strings.Index(path, `"`)]
	recorder := get(t, handler, path)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d", path, recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestBuildNotesAreNotServed(t *testing.T) {
	t.Parallel()

	handler, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if recorder := get(t, handler, "/README.txt"); recorder.Code != http.StatusNotFound {
		t.Errorf("README.txt status = %d, want 404", recorder.Code)
	}
}
