package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go.yaml.in/yaml/v3"

	"github.com/rztaylor/flashheart/internal/board"
)

// DefaultMaxFileCopy is the attachment size limit when the caller gives
// none (REV-2, config attachments.max_bytes).
const DefaultMaxFileCopy = 20 << 20

var (
	// ErrTooLarge reports a file over its size limit.
	ErrTooLarge = errors.New("file is too large")
	// ErrNotRegular reports a source that is not a regular file.
	ErrNotRegular = errors.New("not a regular file")
)

// FileCopy is a local file to copy into a ticket's files/ (REV-1, REV-5).
type FileCopy struct {
	// Source is an absolute path outside or inside the root.
	Source  string
	Caption string
	// Kind is screenshot, log or other.
	Kind string
	Run  string
	// MaxBytes is the size limit; zero means DefaultMaxFileCopy.
	MaxBytes int64
}

// StoredFile reports where a copy landed. Reused means an identical copy
// was already in the ticket and nothing was written.
type StoredFile struct {
	File   string
	Reused bool
}

// CopyIntoTicket copies an allow-listed regular file into
// tickets/<folder>/files/ under the project lock and records it in
// files/index.yaml with its sha256. Copying the same content again reuses
// the earlier copy (MCP-5).
func (s *Store) CopyIntoTicket(project, id string, input FileCopy) (StoredFile, error) {
	if err := s.checkTicket(project, id); err != nil {
		return StoredFile{}, err
	}
	data, err := readSource(input)
	if err != nil {
		return StoredFile{}, err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])

	release, err := s.lock(project)
	if err != nil {
		return StoredFile{}, err
	}
	defer release()
	ticket, err := s.TicketFile(project, id)
	if err != nil {
		return StoredFile{}, err
	}
	dir := path.Join(path.Dir(ticket), "files")
	indexName := path.Join(dir, "index.yaml")
	entries, err := s.readIndex(indexName)
	if err != nil {
		return StoredFile{}, err
	}
	for _, entry := range entries {
		if entry.SHA256 == digest && s.Exists(path.Join(dir, entry.File)) {
			return StoredFile{File: entry.File, Reused: true}, nil
		}
	}
	now := s.now()
	name := storedName(now, filepath.Base(input.Source), func(name string) bool { return s.Exists(path.Join(dir, name)) })
	if err := s.WriteFileAtomic(path.Join(dir, name), data); err != nil {
		return StoredFile{}, err
	}
	kind := input.Kind
	if kind != "screenshot" && kind != "log" {
		kind = "other"
	}
	entries = append(entries, attachmentEntry{
		File: name, Caption: board.OneLine(input.Caption, 300), Kind: kind, Source: input.Source,
		Run: input.Run, Added: now.Format(time.RFC3339), SHA256: digest,
	})
	index, err := yaml.Marshal(entries)
	if err != nil {
		return StoredFile{}, err
	}
	if err := s.WriteFileAtomic(indexName, index); err != nil {
		return StoredFile{}, err
	}
	return StoredFile{File: name}, nil
}

// readSource reads an allow-listed, regular, bounded local file (SEC-2).
func readSource(input FileCopy) ([]byte, error) {
	if !filepath.IsAbs(input.Source) {
		return nil, fmt.Errorf("%w: %q is not an absolute path", ErrInvalidInput, input.Source)
	}
	if _, ok := board.AttachmentType(input.Source); !ok {
		return nil, fmt.Errorf("%s: %w (allowed: png, jpeg, gif, webp, pdf, txt, log, md, json)", filepath.Base(input.Source), ErrTypeNotAllowed)
	}
	limit := input.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxFileCopy
	}
	file, err := os.Open(input.Source)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", input.Source, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", input.Source, ErrNotRegular)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is %d bytes; the limit is %d: %w", filepath.Base(input.Source), info.Size(), limit, ErrTooLarge)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s grew past the limit of %d bytes: %w", filepath.Base(input.Source), limit, ErrTooLarge)
	}
	return data, nil
}

func (s *Store) readIndex(name string) ([]attachmentEntry, error) {
	data, err := s.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []attachmentEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("%s is not a list of files; fix it before adding more: %w", name, err)
	}
	return entries, nil
}

// storedName is <UTC minute>-<sanitised original name>, with -2, -3 …
// before the extension when that name is taken.
func storedName(now time.Time, original string, taken func(string) bool) string {
	ext := strings.ToLower(filepath.Ext(original))
	base := strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return unicode.ToLower(r)
		}
		return '-'
	}, strings.TrimSuffix(original, filepath.Ext(original)))
	base = strings.Trim(strings.Join(strings.FieldsFunc(base, func(r rune) bool { return r == '-' }), "-"), "-")
	if base == "" {
		base = "file"
	}
	if len(base) > 80 {
		base = base[:80]
	}
	stem := now.UTC().Format("20060102T1504") + "-" + base
	name := stem + ext
	for n := 2; taken(name); n++ {
		name = stem + "-" + strconv.Itoa(n) + ext
	}
	return name
}

// WriteReview creates or replaces a ticket's review.md (REV-4) under the
// project lock.
func (s *Store) WriteReview(project, id string, data []byte) error {
	if len(data) > MaxFileBytes {
		return fmt.Errorf("a review is limited to %d bytes: %w", MaxFileBytes, ErrTooLarge)
	}
	if err := s.checkTicket(project, id); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	ticket, err := s.TicketFile(project, id)
	if err != nil {
		return err
	}
	return s.WriteFileAtomic(path.Join(path.Dir(ticket), "review.md"), data)
}
