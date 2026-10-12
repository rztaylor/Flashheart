package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/config"
)

// configPath is config.yaml's location on disk for root.
func configPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(ConfigFile))
}

func TestReadConfigParsesTheRootsSettings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, configPath(root), "quiet_minutes: 7\n")
	settings, err := New(root).ReadConfig()
	if err != nil || settings.QuietMinutes != 7 || settings.LeaseMinutes != config.Defaults().LeaseMinutes {
		t.Errorf("ReadConfig = %+v, %v", settings, err)
	}
}

// A root that does not exist yet, or has no config.yaml, has the defaults.
func TestReadConfigDefaultsWithoutAFileOrRoot(t *testing.T) {
	t.Parallel()

	for name, root := range map[string]string{"no file": t.TempDir(), "no root": filepath.Join(t.TempDir(), "absent")} {
		settings, err := New(root).ReadConfig()
		if err != nil || !reflect.DeepEqual(settings, config.Defaults()) {
			t.Errorf("%s: ReadConfig = %+v, %v; want the defaults", name, settings, err)
		}
	}
	// Reading creates nothing, not even a missing root.
	root := filepath.Join(t.TempDir(), "board")
	if _, err := New(root).ReadConfig(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ReadConfig created the root (stat err = %v)", err)
	}
}

// config.yaml is read through the root, so a symlink out of it is refused
// rather than followed (SEC-2, FH-108).
func TestReadConfigRefusesASymlinkOutOfTheRoot(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "config.yaml")
	write(t, outside, "quiet_minutes: 5\n")
	if err := os.MkdirAll(filepath.Dir(configPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, configPath(root)); err != nil {
		t.Fatal(err)
	}
	settings, err := New(root).ReadConfig()
	if err == nil {
		t.Fatalf("ReadConfig followed a symlink out of the root: %+v", settings)
	}
	if !strings.Contains(err.Error(), configPath(root)) {
		t.Errorf("error = %v, want it to name %s", err, configPath(root))
	}

	// A relative symlink that stays inside the root is read.
	write(t, filepath.Join(root, "settings.yaml"), "quiet_minutes: 6\n")
	if err := os.Remove(configPath(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "settings.yaml"), configPath(root)); err != nil {
		t.Fatal(err)
	}
	if settings, err := New(root).ReadConfig(); err != nil || settings.QuietMinutes != 6 {
		t.Errorf("symlink inside the root: ReadConfig = %+v, %v", settings, err)
	}
}

func TestReadConfigNamesTheFileOnError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, configPath(root), "ui: {theme: sepia}\n")
	if _, err := New(root).ReadConfig(); err == nil || !strings.Contains(err.Error(), configPath(root)) {
		t.Errorf("ReadConfig error = %v, want it to name %s", err, configPath(root))
	}
	write(t, configPath(root), "version: 9\n")
	if _, err := New(root).ReadConfig(); !errors.Is(err, config.ErrNewerVersion) {
		t.Errorf("newer version: %v, want ErrNewerVersion", err)
	}
}

func TestReadConfigRefusesAnOversizedFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, configPath(root), strings.Repeat("# padding\n", MaxFileBytes/10+1))
	if _, err := New(root).ReadConfig(); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("ReadConfig error = %v", err)
	}
}

func TestUpdateConfigWritesUnderTheRootLock(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.UpdateConfig(func(data []byte) ([]byte, error) {
		return append(data, []byte("lease_minutes: 45\n")...), nil
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath(root))
	if err != nil || !strings.HasSuffix(string(data), "lease_minutes: 45\n") || !strings.Contains(string(data), "virtual_columns") {
		t.Errorf("config.yaml = %q, %v", data, err)
	}
	if settings, err := s.ReadConfig(); err != nil || settings.LeaseMinutes != 45 {
		t.Errorf("ReadConfig = %+v, %v", settings, err)
	}
	if err := os.Remove(configPath(root)); err != nil {
		t.Fatal(err)
	}
	var seen []byte
	if err := s.UpdateConfig(func(data []byte) ([]byte, error) { seen = data; return data, nil }); err != nil || seen != nil {
		t.Errorf("missing config: edit saw %q, %v", seen, err)
	}
}

func TestEnsureRootCreatesAMissingRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "reports", "Kanban")
	s := New(root)
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatalf("root after EnsureRoot: %v, %v", info, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o022 != 0 {
		t.Errorf("root mode %04o is writable by others", info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("EnsureRoot created %v in the root", entries)
	}
	// Existing roots are left as they are, and the store can use the root.
	if err := s.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Projects(); err != nil {
		t.Errorf("Projects after EnsureRoot: %v", err)
	}

	file := filepath.Join(t.TempDir(), "file")
	write(t, file, "")
	if err := New(file).EnsureRoot(); err == nil {
		t.Error("EnsureRoot accepted a file as the root")
	}
}
