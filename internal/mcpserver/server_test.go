package mcpserver

import (
	"errors"
	"fmt"
	"testing"

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
		"internal":         errors.New("disk on fire"),
	}
	for code, err := range cases {
		if got := asToolError(err); got.Code != code || got.Fix == "" {
			t.Errorf("%v → %+v, want code %s with a fix", err, got, code)
		}
	}
}

func TestNewNeedsARoot(t *testing.T) {
	t.Parallel()

	if _, err := New(Options{}); err == nil {
		t.Fatal("New without a root succeeded")
	}
}
