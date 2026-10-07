package mcpserver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rztaylor/flashheart/internal/config"
	"github.com/rztaylor/flashheart/internal/mdfile"
	"github.com/rztaylor/flashheart/internal/store"
)

func TestStoreErrorsBecomeProtocolCodes(t *testing.T) {
	t.Parallel()

	cases := map[string]error{
		"conflict":         &store.ConflictError{Name: "x"},
		"key_taken":        &store.KeyTakenError{Key: "FH", InUse: []string{"FH"}},
		"key_fixed":        store.ErrKeyFixed,
		"not_found":        fmt.Errorf("ticket: %w", store.ErrNotFound),
		"type_not_allowed": store.ErrTypeNotAllowed,
		"too_large":        store.ErrTooLarge,
		"invalid_input":    store.ErrInvalidName,
		"needs_repair":     mdfile.ErrBrokenFrontmatter,
		"busy":             store.ErrBusy,
		"unsupported":      config.ErrNewerVersion,
		"internal":         errors.New("disk on fire"),
	}
	for code, err := range cases {
		if got := asToolError(err); got.Code != code || got.Fix == "" {
			t.Errorf("%v → %+v, want code %s with a fix", err, got, code)
		}
	}
}

func TestAMissingWorkstreamNamesTheAvailableOnes(t *testing.T) {
	t.Parallel()

	got := asToolError(&store.WorkstreamNotFoundError{Project: "demo", Slug: "nope", Available: []string{"panel", "sync"}})
	if got.Code != "not_found" || got.Fix != "use one of panel, sync, or create it with create_workstream" {
		t.Fatalf("missing workstream → %+v", got)
	}
	if got := asToolError(&store.WorkstreamNotFoundError{Project: "demo", Slug: "nope"}); got.Fix != "create it with create_workstream" {
		t.Fatalf("no workstreams → %+v", got)
	}
}

func TestNewNeedsARoot(t *testing.T) {
	t.Parallel()

	if _, err := New(Options{}); err == nil {
		t.Fatal("New without a root succeeded")
	}
}
