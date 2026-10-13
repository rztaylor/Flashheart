package setup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// How Codex keys and hashes the trust it saves for a hook (agent-protocol
// §5.4), recorded from Codex 0.160.0 by scripts/record-codex-hook-trust.sh
// (testdata/setup/codex/trusted) and matching codex-rs
// hooks/src/engine/discovery.rs at rust-v0.160.0. Trust is
// [hooks.state."<key>"] trusted_hash = "<hash>" in $CODEX_HOME/config.toml.

// codexEventLabel is an event as Codex names it in trust keys and hashes:
// SessionStart is session_start.
func codexEventLabel(event string) string {
	var b strings.Builder
	for index, r := range event {
		if unicode.IsUpper(r) {
			if index > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// codexHookKey is the key Codex saves a hook's trust under: hooks.json's
// path, the event, and the hook's group and handler positions in the
// file (counting every group and handler, from 0).
func codexHookKey(hooksFile, event string, group, handler int) string {
	return fmt.Sprintf("%s:%s:%d:%d", hooksFile, codexEventLabel(event), group, handler)
}

// codexMatcherEvents are the events whose group matcher Codex uses (and
// hashes); it ignores the matcher of the others.
var codexMatcherEvents = []string{
	"PreToolUse", "PermissionRequest", "PostToolUse", "SessionStart", "SessionEnd",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact",
}

// codexContextEvents are the events whose additionalContextLimit Codex
// keeps (and hashes), unless it is the default.
var codexContextEvents = []string{"PreToolUse", "PostToolUse", "SessionStart", "UserPromptSubmit", "SubagentStart"}

const codexDefaultContextLimit = 2500

// codexHookHash is the hash Codex compares with a command hook's saved
// trust: SHA-256 of the compact JSON, keys sorted, of
// {"event_name", "matcher" (if used), "hooks": [the handler]}, with the
// handler normalised to type, command, timeout (defaulted and clamped as
// Codex runs it), async, and statusMessage and additionalContextLimit when
// set. It returns "" for a handler Codex would not trust this way or not
// load (not a command, a value of the wrong type). Codex on Windows would
// hash commandWindows instead of command; Flashheart writes neither there.
func codexHookHash(event string, group, handler *value) string {
	if handler.get("type").text() != "command" {
		return ""
	}
	command, ok := jsonString(handler.get("command"))
	if !ok {
		return ""
	}
	fields := map[string]any{"type": "command", "command": command}
	timeout, ok := jsonUint(handler.get("timeout"))
	if !ok {
		return ""
	}
	switch event {
	case "SessionEnd", "Interrupt":
		if jsonNull(handler.get("timeout")) {
			timeout = 1
		}
		timeout = min(max(timeout, 1), 3)
	default:
		if jsonNull(handler.get("timeout")) {
			timeout = 600
		}
		timeout = max(timeout, 1)
	}
	fields["timeout"] = timeout
	async, ok := jsonBool(handler.get("async"))
	if !ok {
		return ""
	}
	fields["async"] = async
	if status := handler.get("statusMessage"); !jsonNull(status) {
		text, ok := jsonString(status)
		if !ok {
			return ""
		}
		fields["statusMessage"] = text
	}
	if limit := handler.get("additionalContextLimit"); !jsonNull(limit) {
		n, ok := jsonUint(limit)
		if !ok {
			return ""
		}
		if slices.Contains(codexContextEvents, event) && n != codexDefaultContextLimit {
			fields["additionalContextLimit"] = n
		}
	}
	identity := map[string]any{"event_name": codexEventLabel(event), "hooks": []any{fields}}
	if matcher := group.get("matcher"); slices.Contains(codexMatcherEvents, event) && !jsonNull(matcher) {
		text, ok := jsonString(matcher)
		if !ok {
			return ""
		}
		identity["matcher"] = text
	}
	var b bytes.Buffer
	writeCanonicalJSON(&b, identity)
	sum := sha256.Sum256(b.Bytes())
	return "sha256:" + hex.EncodeToString(sum[:])
}

func jsonNull(v *value) bool {
	return v == nil || string(v.scalar) == "null"
}

func jsonString(v *value) (string, bool) {
	var s string
	if v == nil || v.scalar == nil || json.Unmarshal(v.scalar, &s) != nil {
		return "", false
	}
	return s, true
}

// jsonUint reads a non-negative integer; absent or null is 0.
func jsonUint(v *value) (uint64, bool) {
	if jsonNull(v) {
		return 0, true
	}
	if v.scalar == nil {
		return 0, false
	}
	n, err := strconv.ParseUint(string(v.scalar), 10, 64)
	return n, err == nil
}

// jsonBool reads a boolean; absent is false.
func jsonBool(v *value) (bool, bool) {
	if v == nil {
		return false, true
	}
	switch string(v.scalar) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// writeCanonicalJSON writes v as Rust's serde_json does, compact with
// object keys sorted: strings escape only quotes, backslashes and control
// characters (Go's encoder also escapes <, >, & and U+2028/U+2029).
func writeCanonicalJSON(b *bytes.Buffer, v any) {
	switch v := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		b.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				b.WriteByte(',')
			}
			writeCanonicalJSON(b, key)
			b.WriteByte(':')
			writeCanonicalJSON(b, v[key])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for index, item := range v {
			if index > 0 {
				b.WriteByte(',')
			}
			writeCanonicalJSON(b, item)
		}
		b.WriteByte(']')
	case string:
		b.WriteByte('"')
		for _, r := range v {
			switch r {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if r < 0x20 {
					fmt.Fprintf(b, `\u%04x`, r)
				} else {
					b.WriteRune(r)
				}
			}
		}
		b.WriteByte('"')
	case uint64:
		b.WriteString(strconv.FormatUint(v, 10))
	case bool:
		b.WriteString(strconv.FormatBool(v))
	}
}
