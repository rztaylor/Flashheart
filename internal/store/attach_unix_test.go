//go:build unix

package store

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCopyIntoTicketRefusesAFIFOWithoutBlocking(t *testing.T) {
	t.Parallel()

	s, _ := writable(t)
	fifo := filepath.Join(t.TempDir(), "stream.log")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	if _, err := s.CopyIntoTicket("alpha", "AL-4", FileCopy{Source: fifo}); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("fifo: err = %v", err)
	}
}
