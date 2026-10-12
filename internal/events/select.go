package events

import (
	"bytes"
	"errors"
	"io"
	"time"
)

// Header is an event's envelope without its data: what a selective read
// decides on before it decodes the line (ReadSelected).
type Header struct {
	Time time.Time
	Run  string
	Kind Kind
}

// HeaderOf is the event's header.
func HeaderOf(e Event) Header { return Header{Time: e.Time, Run: e.Run, Kind: e.Kind} }

// ReadSelected is Read for a reader that needs only some events: keep sees
// the header of every line, and fn gets the events it keeps, decoded in
// full. A line in the envelope's own field order, as Append writes it, is
// judged on its header alone, so the data of a line keep refuses is never
// decoded; any other line is decoded first. fn gets exactly the events Read
// would give it whose header keep accepts, in the same order. keep may also
// see the header of a well-formed envelope whose data turns out malformed,
// which Read skips; and a line that repeats a header key, which Append
// never writes, is judged by the first.
func (l *Log) ReadSelected(project string, since time.Time, keep func(Header) bool, fn func(Event)) error {
	runs := map[string]string{}
	return l.read(project, since, func(file string) error {
		_, err := l.lines(project, file, 0, func(line []byte) {
			if h, ok := judge(line, runs); ok && keep(h) {
				if e, ok := parseLine(line); ok {
					fn(e)
				}
			}
		})
		return err
	})
}

// Headers is a project's readable event lines held in memory with their
// headers, for a reader that selects from them more than once: each line
// is read and judged once (hooks' recovery note picks the previous session
// from the headers, then decodes two sessions' events).
type Headers struct {
	lines [][]byte
	list  []Header
}

// ReadHeaders reads the lines Read would read, as they are now, with
// their headers, decoding in full only the lines not in the envelope's own
// field order.
func (l *Log) ReadHeaders(project string, since time.Time) (*Headers, error) {
	h := &Headers{}
	runs := map[string]string{}
	err := l.read(project, since, func(file string) error {
		data, err := l.readFile(project, file)
		if err != nil {
			return err
		}
		for {
			end := bytes.IndexByte(data, '\n')
			if end < 0 {
				// A line still being written is not read yet (ReadFrom).
				return nil
			}
			line := data[:end]
			data = data[end+1:]
			// Too long to be an event: ReadFrom's buffer skips it.
			if end+1 > MaxLineBytes {
				continue
			}
			if header, ok := judge(line, runs); ok {
				h.lines, h.list = append(h.lines, line), append(h.list, header)
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return h, nil
}

// readFile reads a whole event file.
func (l *Log) readFile(project, file string) ([]byte, error) {
	handle, err := l.store.OpenEventFile(project, file)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	info, err := handle.Stat()
	if err != nil {
		return nil, err
	}
	// The size now: what is appended later is read next time, as by Read.
	data := make([]byte, info.Size())
	n, err := io.ReadFull(handle, data)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return data[:n], nil
}

// Each calls fn with every header, in order.
func (h *Headers) Each(fn func(Header)) {
	for _, header := range h.list {
		fn(header)
	}
}

// Select calls fn, in order, with the events whose header keep accepts, as
// ReadSelected would.
func (h *Headers) Select(keep func(Header) bool, fn func(Event)) {
	for i, header := range h.list {
		h.pick(i, header, keep, fn)
	}
}

// SelectBackward is Select from the last line to the first.
func (h *Headers) SelectBackward(keep func(Header) bool, fn func(Event)) {
	for i := len(h.list) - 1; i >= 0; i-- {
		h.pick(i, h.list[i], keep, fn)
	}
}

func (h *Headers) pick(i int, header Header, keep func(Header) bool, fn func(Event)) {
	if keep(header) {
		if e, ok := parseLine(h.lines[i]); ok {
			fn(e)
		}
	}
}

// judge reads a line's header: by peek, or else by decoding the line in
// full; ok is false for a line Read skips (or, by peek, one whose data
// turns out malformed).
func judge(line []byte, runs map[string]string) (Header, bool) {
	if h, ok := peek(line, runs); ok {
		return h, true
	}
	if e, ok := parseLine(line); ok {
		return HeaderOf(e), true
	}
	return Header{}, false
}

// kindNames finds a known kind by its name without allocating.
var kindNames = func() map[string]Kind {
	names := make(map[string]Kind, len(kinds))
	for kind := range kinds {
		names[string(kind)] = kind
	}
	return names
}()

// The envelope's header as MarshalLine writes it: its start, the prefix
// of each string field in order (ts, run, agent, kind, project), and the
// data's key.
var (
	headerStart  = []byte(`{"v":`)
	headerFields = [...][]byte{[]byte(`,"ts":"`), []byte(`,"run":"`), []byte(`,"agent":"`), []byte(`,"kind":"`), []byte(`,"project":"`)}
	headerData   = []byte(`,"data":`)
)

// peek reads the header of a line in the envelope's own field order,
// {"v":…,"ts":"…","run":"…","agent":"…","kind":"…","project":"…","data":…},
// with no escapes in its strings, and accepts it as parseLine would. ok is
// false for any other line, which the caller decodes in full. runs interns
// run ids, which repeat line after line.
func peek(line []byte, runs map[string]string) (Header, bool) {
	rest, ok := bytes.CutPrefix(line, headerStart)
	if !ok {
		return Header{}, false
	}
	// The version: a JSON integer of at least 1.
	comma := bytes.IndexByte(rest, ',')
	if comma < 1 || comma > 9 || rest[0] < '1' || rest[0] > '9' {
		return Header{}, false
	}
	for _, c := range rest[1:comma] {
		if c < '0' || c > '9' {
			return Header{}, false
		}
	}
	rest = rest[comma:]
	var values [len(headerFields)][]byte
	for i, prefix := range headerFields {
		value, found := bytes.CutPrefix(rest, prefix)
		if !found {
			return Header{}, false
		}
		// An escape ends a value early at an escaped quote, which the next
		// prefix then misses, and fails the time and the kind; only the run
		// id is taken as it stands, so it must have none.
		end := bytes.IndexByte(value, '"')
		if end < 0 || i == 1 && bytes.IndexByte(value[:end], '\\') >= 0 {
			return Header{}, false
		}
		values[i], rest = value[:end], value[end+1:]
	}
	ts, run, kindName := values[0], values[1], values[3]
	if len(run) == 0 || !bytes.HasPrefix(rest, headerData) || rest[len(rest)-1] != '}' {
		return Header{}, false
	}
	kind, known := kindNames[string(kindName)]
	if !known {
		return Header{}, false
	}
	at, ok := parseTime(ts)
	if !ok {
		return Header{}, false
	}
	id, seen := runs[string(run)]
	if !seen {
		id = string(run)
		runs[id] = id
	}
	return Header{Time: at, Run: id, Kind: kind}, true
}

// parseTime reads a time as parseLine does, the layout Append writes
// without time.Parse.
func parseTime(ts []byte) (time.Time, bool) {
	if len(ts) == len(timeLayout) && ts[4] == '-' && ts[7] == '-' && ts[10] == 'T' && ts[13] == ':' && ts[16] == ':' && ts[19] == '.' && ts[23] == 'Z' {
		number := func(from, to int) int {
			n := 0
			for _, c := range ts[from:to] {
				if c < '0' || c > '9' {
					return -1
				}
				n = n*10 + int(c-'0')
			}
			return n
		}
		year, month, day := number(0, 4), number(5, 7), number(8, 10)
		hour, minute, second, milli := number(11, 13), number(14, 16), number(17, 19), number(20, 23)
		if year >= 0 && month >= 1 && month <= 12 && day >= 1 && day <= 31 && hour >= 0 && hour < 24 && minute >= 0 && minute < 60 && second >= 0 && second < 60 && milli >= 0 {
			at := time.Date(year, time.Month(month), day, hour, minute, second, milli*int(time.Millisecond), time.UTC)
			// A day past the month's end would roll over; time.Parse refuses it.
			if at.Day() == day {
				return at, true
			}
		}
	}
	at, err := time.Parse(time.RFC3339Nano, string(ts))
	if err != nil {
		return time.Time{}, false
	}
	return at.UTC(), true
}
