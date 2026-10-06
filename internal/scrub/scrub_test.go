package scrub

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSecretsAreRedacted(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, in string
	}{
		{"anthropic key", "key sk-ant-api03-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"},
		{"openai key", "use sk-proj-AbCdEfGhIjKlMnOpQrStUvWxYz0123 now"},
		{"aws access key", "AKIAIOSFODNN7EXAMPLE"},
		{"github token", "ghp_0123456789abcdefABCDEF0123456789abcd"},
		{"github fine-grained token", "github_pat_11ABCDEFG0123456789_abcdefghijklmnopqrstuvwxyz"},
		{"gitlab token", "glpat-abcdefghij0123456789"},
		{"slack token", "xoxb-1234567890-abcdefghij"},
		{"google key", "AIzaSyA-1234567890abcdefghijklmnopqrstu"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.c2lnbmF0dXJlc2lnbmF0dXJl"},
		{"bearer", "Authorization: Bearer abcdef0123456789abcdef"},
		{"password assignment", "password=hunter2hunter2"},
		{"token assignment", `api_key: "q8f7e6d5c4b3a2"`},
		{"url credentials", "https://robert:s3cr3t@example.com/repo.git"},
		{"env-style password", "DB_PASSWORD=hunter2hunter2"},
		{"env-style token", "GITHUB_TOKEN=abcdef0123456789abcdef"},
		{"env-style key with a suffix", "AWS_SECRET_ACCESS_KEY=q8f7e6d5c4b3a2"},
		{"env-style secret key", "STRIPE_SECRET_KEY=rk_live_q8f7e6d5c4b3a2"},
		{"private key", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk\n-----END OPENSSH PRIVATE KEY-----"},
	}
	secrets := []string{"AbCdEfGhIjKl", "IOSFODNN7", "0123456789abcdefABCDEF", "11ABCDEFG", "abcdefghij0123456789", "1234567890-abcdefghij", "SyA-1234567890", "c2lnbmF0dXJl", "abcdef0123456789abcdef", "hunter2", "q8f7e6d5c4b3a2", "s3cr3t", "b3BlbnNzaC1rZXk", "rk_live"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Secrets(tc.in)
			if !strings.Contains(got, Redacted) {
				t.Fatalf("Secrets(%q) = %q, want %s", tc.in, got, Redacted)
			}
			for _, secret := range secrets {
				if strings.Contains(got, secret) {
					t.Fatalf("Secrets(%q) = %q still contains %q", tc.in, got, secret)
				}
			}
		})
	}
}

func TestOrdinaryTextIsKept(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"Fix the header overlap on the board",
		"Edit src/app.ts and run the token tests",
		"password reset flow: add the form",
		"Review FH-42 before release",
		"https://example.com/docs/page",
		"Set TOKEN_LIMIT in the docs",
		"MAX_TOKENS: 4",
	} {
		if got := Secrets(in); got != in {
			t.Errorf("Secrets(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestTextIsOneScrubbedBoundedLine(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"trims and joins lines", "  Write notes\n\tthen  edit\r\n", 50, "Write notes then edit"},
		{"truncates by runes with an ellipsis", "ÅÅÅÅÅÅÅÅÅÅ", 5, "ÅÅÅÅ…"},
		{"keeps a string at the limit", "abcde", 5, "abcde"},
		{"drops control characters", "a\x00b\x1bc", 10, "abc"},
		{"scrubs before limiting", "token=abcdef123456 left", 100, "token=" + Redacted + " left"},
		{"repairs invalid utf-8", "a\xffb", 10, "a�b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Text(tc.in, tc.limit)
			if got != tc.want {
				t.Fatalf("Text(%q, %d) = %q, want %q", tc.in, tc.limit, got, tc.want)
			}
			if utf8.RuneCountInString(got) > tc.limit {
				t.Fatalf("Text(%q, %d) has %d runes", tc.in, tc.limit, utf8.RuneCountInString(got))
			}
		})
	}
}
