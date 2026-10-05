package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultsMatchBoardFormat(t *testing.T) {
	t.Parallel()

	// docs/dev/specs/board-format.md §config.yaml.
	want := Config{
		Version:            2,
		AutoCreateProjects: true,
		QuietMinutes:       10,
		EventRetentionDays: 90,
		DoneColumnLimit:    20,
		Attachments:        Attachments{MaxBytes: 20 << 20},
		UI:                 UI{Theme: ThemeSystem, Density: DensityNormal, ColourBy: "type", VirtualColumns: []string{"needs-you"}},
	}
	if got := Defaults(); !reflect.DeepEqual(got, want) {
		t.Errorf("Defaults() = %+v, want %+v", got, want)
	}
}

func TestLoadSampleBoard(t *testing.T) {
	t.Parallel()

	got, err := Load(filepath.Join("..", "..", "testdata", "boards", "sample"))
	if err != nil {
		t.Fatalf("Load(sample): %v", err)
	}
	if !reflect.DeepEqual(got, Defaults()) {
		t.Errorf("sample config = %+v, want the documented defaults", got)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	t.Parallel()

	for _, root := range []string{t.TempDir(), filepath.Join(t.TempDir(), "not-created")} {
		got, err := Load(root)
		if err != nil {
			t.Fatalf("Load(%q): %v", root, err)
		}
		if !reflect.DeepEqual(got, Defaults()) {
			t.Errorf("Load(%q) = %+v, want defaults", root, got)
		}
	}
}

func TestLoadDoesNotCreateAnything(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "board")
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load created the root (stat err = %v)", err)
	}
}

func TestParseOverridesOnlyPresentKeys(t *testing.T) {
	t.Parallel()

	got, err := Parse([]byte(`
# a comment
quiet_minutes: 5
unknown_key: kept for later writers
ui:
  theme: dark
  virtual_columns: [needs-you, agent-working]
`))
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}
	want := Defaults()
	want.QuietMinutes = 5
	want.UI.Theme = ThemeDark
	want.UI.VirtualColumns = []string{"needs-you", "agent-working"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse() = %+v, want %+v", got, want)
	}
}

func TestParseEmptyDocumentUsesDefaults(t *testing.T) {
	t.Parallel()

	for _, data := range []string{"", "\n", "# only a comment\n"} {
		got, err := Parse([]byte(data))
		if err != nil {
			t.Fatalf("Parse(%q): %v", data, err)
		}
		if !reflect.DeepEqual(got, Defaults()) {
			t.Errorf("Parse(%q) = %+v", data, got)
		}
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want string
	}{
		{"newer version", "version: 3", "board root uses format version 3; this flashheart understands version 2"},
		{"zero version", "version: 0", "version must be 1 or 2"},
		{"theme", "ui: {theme: sepia}", `ui.theme "sepia" must be one of system, light, dark`},
		{"density", "ui: {density: roomy}", `ui.density "roomy" must be one of compact, normal, detailed`},
		{"virtual column", "ui: {virtual_columns: [blocked]}", `ui.virtual_columns "blocked" must be one of needs-you, agent-working`},
		{"quiet minutes", "quiet_minutes: 0", "quiet_minutes must be at least 1"},
		{"retention", "event_retention_days: -1", "event_retention_days must be at least 1"},
		{"done limit", "done_column_limit: 0", "done_column_limit must be at least 1"},
		{"attachment size", "attachments: {max_bytes: 0}", "attachments.max_bytes must be at least 1"},
		{"wrong type", "quiet_minutes: soon", "parse config.yaml"},
		{"not a mapping", "- a\n- b", "parse config.yaml"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(test.data))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("Parse(%q) error = %v, want %q", test.data, err, test.want)
			}
		})
	}
}

func TestVersionOneIsReadableForMigration(t *testing.T) {
	t.Parallel()

	config, err := Parse([]byte("version: 1\n"))
	if err != nil || config.Version != 1 {
		t.Errorf("Parse(version 1) = %+v, %v", config, err)
	}
}

func TestNewerVersionIsDistinguishable(t *testing.T) {
	t.Parallel()

	if _, err := Parse([]byte("version: 4")); !errors.Is(err, ErrNewerVersion) {
		t.Errorf("err = %v, want ErrNewerVersion", err)
	}
}

func TestLoadNamesTheFileOnError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".flashheart", "config.yaml")
	if err := os.WriteFile(path, []byte("ui: {theme: sepia}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("Load() error = %v, want it to name %s", err, path)
	}
}

func TestLoadRefusesOversizedFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".flashheart"), 0o755); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("# padding\n", (maxConfigBytes/10)+1)
	if err := os.WriteFile(filepath.Join(root, ".flashheart", "config.yaml"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("Load() error = %v", err)
	}
}
