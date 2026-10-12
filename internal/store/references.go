package store

import (
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// The reference scan both permanent deletes share (EDIT-8, PRJ-5). It finds
// every ticket, live or archived, in every live project, whose depends-on
// names an id being deleted. It reads each ticket file once, through a
// handle on its tickets directory: opening files from the root, as
// ReadProject and ReadArchived do for the folder, the file and its files/
// folder, is what made a scan slow, not parsing. Only the frontmatter's
// depends-on decides a match; a match is then hashed and parsed for its id
// and title, and its path and hash go into the token.
//
// The scan takes no lock, and needs none: what it reads is only trusted
// once a scan under the locks agrees with it. A delete scans without a lock
// for its preview and to learn which projects it rewrites, then takes the
// root lock and those projects' locks and scans again; a different token
// (a ticket that gained, lost or changed a reference, a workstream that
// lists the id, a change to the folder) refuses it. The projects it
// rewrites cannot change until its writes are done. A project it does not
// lock can still gain a reference after that scan, as it can after the
// delete; the id is retired by then, which counts as done, so nothing waits
// on it.

// reference is a ticket whose depends-on names ids being deleted.
type reference struct {
	project, id, title string
	// matched are the ids being deleted that its depends-on names.
	matched    []string
	file, hash string
}

// ticketDirs are a project's ticket folders' parents, live then archived.
var ticketDirs = []string{"tickets", path.Join(".archive", "tickets")}

// scanReferences returns, in project order and then by id, the tickets
// whose depends-on names any of ids, other than the folder skip (relative to
// the root), and writes a "ticket <file> <hash>" line for each to token.
// Ticket folders that are symbolic links (Flashheart never writes through
// one) and files that cannot be read whole are left out.
func (s *Store) scanReferences(ids []string, skip string, token hash.Hash) ([]reference, error) {
	root, _, err := s.handle()
	if err != nil {
		return nil, err
	}
	projects, err := s.Projects()
	if err != nil {
		return nil, err
	}
	var out []reference
	for _, project := range projects {
		for _, dir := range ticketDirs {
			found := scanDir(root, path.Join(project, dir), ids, skip)
			for index := range found {
				found[index].project = project
				fmt.Fprintf(token, "ticket %s %s\n", found[index].file, found[index].hash)
			}
			out = append(out, found...)
		}
	}
	return out, nil
}

// scanDir scans one tickets directory. One that is missing or cannot be
// read has no references, as it has no tickets on the board.
func scanDir(root *os.Root, dir string, ids []string, skip string) []reference {
	tickets, err := root.OpenRoot(dir)
	if err != nil {
		return nil
	}
	defer tickets.Close()
	entries, err := fs.ReadDir(tickets.FS(), ".")
	if err != nil {
		return nil
	}
	var out []reference
	for _, entry := range entries {
		folder := entry.Name()
		if !entry.IsDir() || !ValidName(folder) || path.Join(dir, folder) == skip {
			continue
		}
		if found := scanTicket(tickets, dir, folder, ids); found != nil {
			out = append(out, *found)
		}
	}
	slices.SortStableFunc(out, func(a, b reference) int { return board.CompareIDs(a.id, b.id) })
	return out
}

// scanTicket reads one ticket folder's file and returns it as a reference
// when its depends-on names any of ids.
func scanTicket(tickets *os.Root, dir, folder string, ids []string) *reference {
	name, data, ok := readTicketFile(tickets, folder)
	if !ok {
		return nil
	}
	var matched []string
	for _, id := range mdfile.Parse(data).List("depends-on") {
		if slices.Contains(ids, id) && !slices.Contains(matched, id) {
			matched = append(matched, id)
		}
	}
	if len(matched) == 0 {
		return nil
	}
	ticket := board.ParseTicket(folder, data)
	id := ticket.ID
	if id == "" {
		id, _, _ = board.ParseFolder(folder)
	}
	return &reference{
		id: id, title: ticket.Title, matched: matched,
		file: path.Join(dir, folder, name), hash: Hash(data),
	}
}

// readTicketFile reads a ticket folder's file (board.TicketFile), trying the
// usual name before listing the folder. ok is false when there is none or
// it cannot be read whole.
func readTicketFile(tickets *os.Root, folder string) (string, []byte, bool) {
	name := folder + ".md"
	data, err := readBounded(tickets, path.Join(folder, name))
	if errors.Is(err, fs.ErrNotExist) {
		entries, err := fs.ReadDir(tickets.FS(), folder)
		if err != nil {
			return "", nil, false
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		var ok bool
		if name, _, ok = board.TicketFile(folder, names); !ok {
			return "", nil, false
		}
		data, err = readBounded(tickets, path.Join(folder, name))
	}
	if err != nil {
		return "", nil, false
	}
	return name, data, true
}

// readBounded reads a regular file of at most MaxFileBytes.
func readBounded(root *os.Root, name string) ([]byte, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("%s is larger than %d KiB", name, MaxFileBytes>>10)
	}
	return data, nil
}
