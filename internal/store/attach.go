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

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/scrub"
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

// CopyIntoTicket copies an allow-listed regular local file into a ticket
// (AddAttachment): readSource is its input adapter for a path (REV-1,
// REV-5).
func (s *Store) CopyIntoTicket(project, id string, input FileCopy) (StoredFile, error) {
	if err := s.checkTicket(project, id); err != nil {
		return StoredFile{}, err
	}
	data, err := readSource(input)
	if err != nil {
		return StoredFile{}, err
	}
	return s.AddAttachment(project, id, NewAttachment{
		Name: filepath.Base(input.Source), Data: data, Caption: input.Caption, Kind: input.Kind,
		Source: input.Source, Run: input.Run, MaxBytes: input.MaxBytes,
	})
}

// NewAttachment is a file's content to store in a ticket, from wherever it
// came (a local path, the browser).
type NewAttachment struct {
	// Name is the file's original name; its extension decides whether it
	// is allowed (REV-2) and its stored name follows it.
	Name string
	Data []byte
	// Caption is one line; Kind is screenshot, log or other (from Name
	// when empty).
	Caption, Kind string
	// Source is where it was copied from, if anywhere; Run who added it.
	Source, Run string
	// MaxBytes is the size limit; zero means DefaultMaxFileCopy.
	MaxBytes int64
}

// AddAttachment stores a file in tickets/<folder>/files/ under the project
// lock and records it in files/index.yaml with its sha256, keeping the
// index's other content. The file must be allow-listed and within its size
// limit (REV-2, SEC-2), and text is scrubbed of likely secrets first
// (SEC-3). Adding the same content again reuses the earlier copy (MCP-5).
func (s *Store) AddAttachment(project, id string, input NewAttachment) (StoredFile, error) {
	if err := s.checkTicket(project, id); err != nil {
		return StoredFile{}, err
	}
	contentType, ok := board.AttachmentType(input.Name)
	if !ok {
		return StoredFile{}, fmt.Errorf("%s: %w (allowed: %s)", input.Name, ErrTypeNotAllowed, board.AllowedAttachments())
	}
	if limit := maxBytes(input.MaxBytes); int64(len(input.Data)) > limit {
		return StoredFile{}, fmt.Errorf("%s is %d bytes; the limit is %d: %w", input.Name, len(input.Data), limit, ErrTooLarge)
	}
	data := input.Data
	if board.AttachmentScrubbed(contentType) {
		data = []byte(scrub.Secrets(string(data)))
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
	index, err := s.readIndex(indexName)
	if err != nil {
		return StoredFile{}, err
	}
	for _, entry := range index.entries {
		if entry.SHA256 == digest && s.Exists(path.Join(dir, entry.File)) {
			return StoredFile{File: entry.File, Reused: true}, nil
		}
	}
	now := s.now()
	name := storedName(now, input.Name, func(name string) bool { return s.Exists(path.Join(dir, name)) })
	if err := s.WriteFileAtomic(path.Join(dir, name), data); err != nil {
		return StoredFile{}, err
	}
	err = index.add(attachmentEntry{
		File: name, Caption: board.OneLine(input.Caption, 300), Kind: board.AttachmentKindFor(input.Kind, input.Name),
		Source: input.Source, Run: input.Run, Added: now.Format(time.RFC3339), SHA256: digest,
	})
	if err != nil {
		return StoredFile{}, err
	}
	encoded, err := index.encode()
	if err != nil {
		return StoredFile{}, err
	}
	if err := s.WriteFileAtomic(indexName, encoded); err != nil {
		return StoredFile{}, err
	}
	return StoredFile{File: name}, nil
}

func maxBytes(limit int64) int64 {
	if limit <= 0 {
		return DefaultMaxFileCopy
	}
	return limit
}

// readSource reads an allow-listed, regular, bounded local file (SEC-2),
// the path adapter of AddAttachment, which checks and scrubs the bytes.
// The file is judged by what it is, not only by the name given: a link's
// target must be allow-listed too, and anything but a regular file (a
// directory, a FIFO that would block) is refused before it is opened.
func readSource(input FileCopy) ([]byte, error) {
	if !filepath.IsAbs(input.Source) {
		return nil, fmt.Errorf("%w: %q is not an absolute path", ErrInvalidInput, input.Source)
	}
	if _, ok := board.AttachmentType(input.Source); !ok {
		return nil, fmt.Errorf("%s: %w (allowed: %s)", filepath.Base(input.Source), ErrTypeNotAllowed, board.AllowedAttachments())
	}
	limit := maxBytes(input.MaxBytes)
	info, err := os.Stat(input.Source)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", input.Source, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", input.Source, ErrNotRegular)
	}
	target, err := filepath.EvalSymlinks(input.Source)
	if err != nil {
		return nil, err
	}
	if _, ok := board.AttachmentType(target); !ok {
		return nil, fmt.Errorf("%s links to %s: %w", filepath.Base(input.Source), filepath.Base(target), ErrTypeNotAllowed)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("%s is %d bytes; the limit is %d: %w", filepath.Base(input.Source), info.Size(), limit, ErrTooLarge)
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if opened, err := file.Stat(); err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("%s changed while it was being copied: %w", input.Source, ErrNotRegular)
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

// readIndex reads a files index to add to; one that is not a list of
// files is refused rather than replaced.
func (s *Store) readIndex(name string) (filesIndex, error) {
	data, err := s.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return filesIndex{}, nil
	}
	if err != nil {
		return filesIndex{}, err
	}
	index, err := parseFilesIndex(data)
	if err != nil {
		return filesIndex{}, fmt.Errorf("%s is not a list of files; fix it before adding more: %w", name, err)
	}
	return index, nil
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

// UpdateReview applies edit to a ticket's review.md under the project lock,
// with the content-hash precondition of UpdateTicket (REV-3: ticking How to
// Verify steps). A ticket with no review is ErrNotFound.
func (s *Store) UpdateReview(project, id, base string, edit Edit) (string, error) {
	if err := s.checkTicket(project, id); err != nil {
		return "", err
	}
	release, err := s.lock(project)
	if err != nil {
		return "", err
	}
	defer release()
	ticket, err := s.TicketFile(project, id)
	if err != nil {
		return "", err
	}
	name := path.Join(path.Dir(ticket), "review.md")
	if !s.Exists(name) {
		return "", fmt.Errorf("review of %s: %w", id, ErrNotFound)
	}
	return s.updateLocked(name, base, edit, false)
}
