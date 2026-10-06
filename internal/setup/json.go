package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// value is a JSON value that keeps object key order and the exact bytes of
// scalars, so a settings file Flashheart edits changes only where it edits
// it.
type value struct {
	object  []member
	array   []*value
	scalar  json.RawMessage
	isObj   bool
	isArray bool
}

type member struct {
	key   string
	value *value
}

func parseJSON(data []byte) (*value, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	v, err := decodeValue(decoder, data)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data after the JSON value")
	}
	return v, nil
}

// decodeValue reads one value; scalars keep the bytes they were written
// with (escapes, number spelling), sliced from data by decoder offsets.
func decodeValue(decoder *json.Decoder, data []byte) (*value, error) {
	start := decoder.InputOffset()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch t := token.(type) {
	case json.Delim:
		switch t {
		case '{':
			v := &value{isObj: true}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, _ := keyToken.(string)
				child, err := decodeValue(decoder, data)
				if err != nil {
					return nil, err
				}
				v.object = append(v.object, member{key: key, value: child})
			}
			_, err := decoder.Token()
			return v, err
		case '[':
			v := &value{isArray: true}
			for decoder.More() {
				child, err := decodeValue(decoder, data)
				if err != nil {
					return nil, err
				}
				v.array = append(v.array, child)
			}
			_, err := decoder.Token()
			return v, err
		}
		return nil, fmt.Errorf("unexpected %v", t)
	default:
		// The offset before a token may sit before the separator and space
		// that precede it.
		raw := bytes.TrimLeft(data[start:decoder.InputOffset()], " \t\r\n,:")
		return &value{scalar: append(json.RawMessage(nil), raw...)}, nil
	}
}

func marshalString(s string) ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func stringValue(s string) *value {
	raw, _ := marshalString(s)
	return &value{scalar: raw}
}

func numberValue(n int) *value { return &value{scalar: json.RawMessage(fmt.Sprint(n))} }

func (v *value) get(key string) *value {
	if v == nil || !v.isObj {
		return nil
	}
	for _, m := range v.object {
		if m.key == key {
			return m.value
		}
	}
	return nil
}

func (v *value) set(key string, child *value) {
	for index := range v.object {
		if v.object[index].key == key {
			v.object[index].value = child
			return
		}
	}
	v.object = append(v.object, member{key: key, value: child})
}

func (v *value) remove(key string) {
	for index := range v.object {
		if v.object[index].key == key {
			v.object = append(v.object[:index], v.object[index+1:]...)
			return
		}
	}
}

// text returns a scalar string's value, or "".
func (v *value) text() string {
	if v == nil || v.scalar == nil {
		return ""
	}
	var s string
	if json.Unmarshal(v.scalar, &s) != nil {
		return ""
	}
	return s
}

// equal compares two values structurally (scalars by meaning).
func (v *value) equal(other *value) bool {
	switch {
	case v == nil || other == nil:
		return v == other
	case v.isObj != other.isObj || v.isArray != other.isArray:
		return false
	case v.isObj:
		if len(v.object) != len(other.object) {
			return false
		}
		for index, m := range v.object {
			if other.object[index].key != m.key || !m.value.equal(other.object[index].value) {
				return false
			}
		}
		return true
	case v.isArray:
		if len(v.array) != len(other.array) {
			return false
		}
		for index, child := range v.array {
			if !child.equal(other.array[index]) {
				return false
			}
		}
		return true
	}
	var a, b any
	_ = json.Unmarshal(v.scalar, &a)
	_ = json.Unmarshal(other.scalar, &b)
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// encode writes v the way Claude Code writes its settings: one member per
// line, with the file's indent; empty objects and arrays stay on one line.
func (v *value) encode(indent string) []byte {
	var b bytes.Buffer
	v.write(&b, indent, 0)
	b.WriteByte('\n')
	return b.Bytes()
}

func (v *value) write(b *bytes.Buffer, indent string, depth int) {
	pad := strings.Repeat(indent, depth+1)
	switch {
	case v.isObj:
		if len(v.object) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for index, m := range v.object {
			key, _ := marshalString(m.key)
			b.WriteString(pad)
			b.Write(key)
			b.WriteString(": ")
			m.value.write(b, indent, depth+1)
			if index < len(v.object)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(indent, depth) + "}")
	case v.isArray:
		if len(v.array) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for index, child := range v.array {
			b.WriteString(pad)
			child.write(b, indent, depth+1)
			if index < len(v.array)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(indent, depth) + "]")
	default:
		b.Write(v.scalar)
	}
}

// indentOf finds the indent a JSON file uses, defaulting to two spaces.
func indentOf(data []byte) string {
	for _, line := range strings.Split(string(data), "\n")[1:] {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != "" && len(trimmed) < len(line) {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "  "
}
