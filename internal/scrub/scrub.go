package scrub

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Redacted replaces every secret found.
const Redacted = "[redacted]"

// tokens are whole-match secret shapes.
var tokens = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(-----END [A-Z ]*PRIVATE KEY-----|$)`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\b(AKIA|ASIA)[A-Z0-9]{16}\b`),
	regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\bAIza[A-Za-z0-9_-]{30,}`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
}

// bearer keeps the scheme word and redacts the credential after it.
var bearer = regexp.MustCompile(`(?i)\b(bearer|basic|token)(\s+)[A-Za-z0-9._~+/=-]{12,}`)

// assignment keeps a secret-named key and redacts its value. The name may
// be part of an environment-style name (DB_PASSWORD, AWS_SECRET_ACCESS_KEY,
// GITHUB_TOKEN), which \b alone misses after an underscore.
var assignment = regexp.MustCompile(`(?i)\b((?:[A-Za-z0-9]+_)*(?:password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|auth)(?:_[A-Za-z0-9]+)*)(["']?\s*[:=]\s*["']?)([^\s"',;]{4,})`)

// urlCredentials redacts user:password@ in URLs.
var urlCredentials = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/\s:@]+:[^/\s@]+@`)

// Secrets redacts likely secrets in s and leaves everything else unchanged.
func Secrets(s string) string {
	for _, pattern := range tokens {
		s = pattern.ReplaceAllString(s, Redacted)
	}
	s = bearer.ReplaceAllString(s, "${1}${2}"+Redacted)
	s = assignment.ReplaceAllString(s, "${1}${2}"+Redacted)
	return urlCredentials.ReplaceAllString(s, "${1}"+Redacted+"@")
}

// Text returns s scrubbed of secrets as one line of at most limit runes:
// whitespace runs become single spaces, control characters are dropped,
// invalid UTF-8 is repaired, and a cut string ends with an ellipsis.
func Text(s string, limit int) string {
	s = Secrets(strings.ToValidUTF8(s, "�"))
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			space = b.Len() > 0
			continue
		case unicode.IsControl(r):
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return Limit(b.String(), limit)
}

// Limit cuts s to at most limit runes, ending a cut string with an ellipsis.
func Limit(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit-1]) + "…"
}
