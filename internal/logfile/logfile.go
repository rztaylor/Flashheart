package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultMaxBytes is the rotation size used for Flashheart's logs.
const DefaultMaxBytes = 1 << 20

// Writer is a lazily opened, size-rotated log file safe for concurrent use.
type Writer struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	file     *os.File
	size     int64
	now      func() time.Time
}

// ServeLog returns the writer for <root>/.flashheart/serve.log.
func ServeLog(root string) *Writer {
	return New(filepath.Join(root, ".flashheart", "serve.log"), DefaultMaxBytes)
}

// HookErrors returns the writer for <root>/.flashheart/hook-errors.log (HOOK-1).
func HookErrors(root string) *Writer {
	return New(filepath.Join(root, ".flashheart", "hook-errors.log"), DefaultMaxBytes)
}

// New returns a writer for path that rotates to path+".1" past maxBytes.
func New(path string, maxBytes int64) *Writer {
	return &Writer{path: path, maxBytes: maxBytes, now: time.Now}
}

// Write appends p as one timestamped entry, ending it with a newline.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	entry := make([]byte, 0, len(p)+22)
	entry = append(entry, w.now().UTC().Format(time.RFC3339)...)
	entry = append(entry, ' ')
	entry = append(entry, p...)
	if len(p) == 0 || p[len(p)-1] != '\n' {
		entry = append(entry, '\n')
	}
	if err := w.open(); err != nil {
		return 0, err
	}
	if w.size > 0 && w.size+int64(len(entry)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(entry)
	w.size += int64(n)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close releases the file if it was opened.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *Writer) open() error {
	if w.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(w.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("stat log: %w", err)
	}
	w.file, w.size = file, info.Size()
	return nil
}

func (w *Writer) rotate() error {
	w.file.Close()
	w.file = nil
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return fmt.Errorf("rotate log: %w", err)
	}
	return w.open()
}
