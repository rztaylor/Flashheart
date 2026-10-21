package main

import (
	"errors"
	"strings"
	"testing"
)

func TestNewestReleaseReadsTheFirstReleasedSection(t *testing.T) {
	t.Parallel()

	changelog := `# Changelog

Intro text.

## Unreleased

### Added

- Something not released yet.

## [0.2.0] - 2026-11-02

### Added

- Downloads for macOS and Linux.

### Fixed

- A bug.

## [0.1.0] - 2026-10-21

- The first release.

[0.2.0]: https://github.com/rztaylor/Flashheart/releases/tag/v0.2.0
`
	got, err := NewestRelease(changelog)
	if err != nil {
		t.Fatalf("NewestRelease: %v", err)
	}
	if got.Version != "0.2.0" || got.Date != "2026-11-02" {
		t.Errorf("version, date = %q, %q; want 0.2.0, 2026-11-02", got.Version, got.Date)
	}
	wantNotes := "### Added\n\n- Downloads for macOS and Linux.\n\n### Fixed\n\n- A bug.\n"
	if got.Notes != wantNotes {
		t.Errorf("notes = %q, want %q", got.Notes, wantNotes)
	}
}

func TestNewestReleaseLeavesLinkDefinitionsOutOfTheNotes(t *testing.T) {
	t.Parallel()

	changelog := "## [Unreleased]\n\n## [1.0.0] - 2027-01-05\n\n- Stable.\n\n[Unreleased]: https://example.invalid/compare\n[1.0.0]: https://example.invalid/v1.0.0\n"
	got, err := NewestRelease(changelog)
	if err != nil {
		t.Fatalf("NewestRelease: %v", err)
	}
	if got.Version != "1.0.0" || got.Notes != "- Stable.\n" {
		t.Errorf("got %q with notes %q; want 1.0.0 with %q", got.Version, got.Notes, "- Stable.\n")
	}
}

func TestNewestReleaseWithOnlyUnreleasedFails(t *testing.T) {
	t.Parallel()

	for _, changelog := range []string{
		"# Changelog\n\n## Unreleased\n\n### Added\n\n- Work.\n",
		"# Changelog\n\n## [Unreleased]\n\n- Work.\n",
		"",
	} {
		_, err := NewestRelease(changelog)
		if !errors.Is(err, ErrNoRelease) {
			t.Errorf("NewestRelease(%q) error = %v, want ErrNoRelease", changelog, err)
		}
	}
}

func TestNewestReleaseRefusesMalformedHeadings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		heading string
	}{
		{"no brackets", "## 0.1.0 - 2026-10-21"},
		{"no date", "## [0.1.0]"},
		{"two-part version", "## [0.1] - 2026-10-21"},
		{"v prefix", "## [v0.1.0] - 2026-10-21"},
		{"pre-release suffix", "## [0.1.0-rc.1] - 2026-10-21"},
		{"leading zero", "## [0.01.0] - 2026-10-21"},
		{"impossible date", "## [0.1.0] - 2026-13-01"},
		{"short date", "## [0.1.0] - 2026-1-5"},
		{"em dash", "## [0.1.0] — 2026-10-21"},
		{"trailing text", "## [0.1.0] - 2026-10-21 (yanked)"},
		{"other heading", "## Notes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changelog := "# Changelog\n\n## Unreleased\n\n" + test.heading + "\n\n- Notes.\n\n## [0.0.1] - 2026-01-01\n\n- Older.\n"
			_, err := NewestRelease(changelog)
			if err == nil {
				t.Fatalf("NewestRelease accepted %q", test.heading)
			}
			if errors.Is(err, ErrNoRelease) {
				t.Fatalf("error = %v, want a malformed-heading error", err)
			}
			if !strings.Contains(err.Error(), "line 5") || !strings.Contains(err.Error(), test.heading) {
				t.Errorf("error %q does not name line 5 and the heading %q", err, test.heading)
			}
		})
	}
}

func TestNewestReleaseRefusesAnEmptySection(t *testing.T) {
	t.Parallel()

	_, err := NewestRelease("## Unreleased\n\n## [0.1.0] - 2026-10-21\n\n## [0.0.1] - 2026-01-01\n\n- Older.\n")
	if err == nil || !strings.Contains(err.Error(), "no notes") {
		t.Fatalf("error = %v, want one saying the section has no notes", err)
	}
}

func TestNewestReleaseRefusesNotesGitHubWouldReject(t *testing.T) {
	t.Parallel()

	// "- é\n" is four characters (five bytes).
	long := "## [0.1.0] - 2026-10-21\n\n" + strings.Repeat("- é\n", MaxNotes/4+1)
	_, err := NewestRelease(long)
	if err == nil || !strings.Contains(err.Error(), "125000") {
		t.Fatalf("error = %v, want one naming GitHub's 125000-character limit", err)
	}
	fits := "## [0.1.0] - 2026-10-21\n\n" + strings.Repeat("- é\n", MaxNotes/4-1)
	if _, err := NewestRelease(fits); err != nil {
		t.Fatalf("notes under the limit: %v", err)
	}
}

func TestNewestReleaseIgnoresHeadingsInsideCodeFences(t *testing.T) {
	t.Parallel()

	changelog := "## Unreleased\n\n```\n## not a heading\n```\n\n## [0.3.1] - 2026-12-01\n\n- Fix.\n"
	got, err := NewestRelease(changelog)
	if err != nil {
		t.Fatalf("NewestRelease: %v", err)
	}
	if got.Version != "0.3.1" {
		t.Errorf("version = %q, want 0.3.1", got.Version)
	}
}

func TestTheRepositoryChangelogParses(t *testing.T) {
	t.Parallel()

	// The real CHANGELOG.md must always be readable: either a released
	// section or, before the first release, only Unreleased.
	data := readRepoFile(t, "CHANGELOG.md")
	if _, err := NewestRelease(string(data)); err != nil && !errors.Is(err, ErrNoRelease) {
		t.Fatalf("CHANGELOG.md: %v", err)
	}
}
