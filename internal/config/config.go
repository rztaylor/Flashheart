package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FormatVersion is the board format version this build writes; version 1
// roots are read only to migrate them (MIG-1).
const FormatVersion = 2

// maxConfigBytes bounds config.yaml reads.
const maxConfigBytes = 1 << 20

// ErrNewerVersion reports a root written by a newer Flashheart.
var ErrNewerVersion = errors.New("board root format is newer than this flashheart")

// Theme values for UI.Theme.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Density values for UI.Density.
const (
	DensityCompact  = "compact"
	DensityNormal   = "normal"
	DensityDetailed = "detailed"
)

var (
	themes         = []string{ThemeSystem, ThemeLight, ThemeDark}
	densities      = []string{DensityCompact, DensityNormal, DensityDetailed}
	virtualColumns = []string{"needs-you", "agent-working"}
)

// Config is the global configuration (CFG-1, CFG-2).
type Config struct {
	Version            int         `yaml:"version"`
	AutoCreateProjects bool        `yaml:"auto_create_projects"`
	QuietMinutes       int         `yaml:"quiet_minutes"`
	EventRetentionDays int         `yaml:"event_retention_days"`
	DoneColumnLimit    int         `yaml:"done_column_limit"`
	Attachments        Attachments `yaml:"attachments"`
	UI                 UI          `yaml:"ui"`
}

// Attachments holds attachment limits.
type Attachments struct {
	MaxBytes int64 `yaml:"max_bytes"`
}

// UI holds interface preferences saved through the backend.
type UI struct {
	Theme          string   `yaml:"theme"`
	Density        string   `yaml:"density"`
	VirtualColumns []string `yaml:"virtual_columns"`
}

// Defaults returns the documented defaults.
func Defaults() Config {
	return Config{
		Version:            FormatVersion,
		AutoCreateProjects: true,
		QuietMinutes:       10,
		EventRetentionDays: 90,
		DoneColumnLimit:    20,
		Attachments:        Attachments{MaxBytes: 20 << 20},
		UI:                 UI{Theme: ThemeSystem, Density: DensityNormal, VirtualColumns: []string{"needs-you"}},
	}
}

// Path returns the global config file location for a root.
func Path(root string) string {
	return filepath.Join(root, ".flashheart", "config.yaml")
}

// Load reads the root's config.yaml. A missing file or root yields defaults.
func Load(root string) (Config, error) {
	path := Path(root)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > maxConfigBytes {
		return Config{}, fmt.Errorf("read %s: file is larger than %d bytes", path, maxConfigBytes)
	}
	config, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return config, nil
}

// Parse decodes config.yaml content over the defaults and validates it.
// Unknown keys are ignored here and preserved by writers.
func Parse(data []byte) (Config, error) {
	config := Defaults()
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Config{}, fmt.Errorf("parse config.yaml: %w", err)
	}
	if len(document.Content) == 0 {
		return config, nil
	}
	if root := document.Content[0]; root.Kind != yaml.MappingNode {
		return Config{}, fmt.Errorf("parse config.yaml: line %d: expected a mapping of settings", root.Line)
	}
	if err := document.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse config.yaml: %w", err)
	}
	return config, config.validate()
}

func (c Config) validate() error {
	var problems []string
	if c.Version > FormatVersion {
		return fmt.Errorf("%w: board root uses format version %d; this flashheart understands version %d", ErrNewerVersion, c.Version, FormatVersion)
	}
	if c.Version < 1 {
		problems = append(problems, fmt.Sprintf("version must be 1 or %d", FormatVersion))
	}
	atLeastOne := func(name string, value int64) {
		if value < 1 {
			problems = append(problems, name+" must be at least 1")
		}
	}
	atLeastOne("quiet_minutes", int64(c.QuietMinutes))
	atLeastOne("event_retention_days", int64(c.EventRetentionDays))
	atLeastOne("done_column_limit", int64(c.DoneColumnLimit))
	atLeastOne("attachments.max_bytes", c.Attachments.MaxBytes)
	oneOf := func(name, value string, allowed []string) {
		if !slices.Contains(allowed, value) {
			problems = append(problems, fmt.Sprintf("%s %q must be one of %s", name, value, strings.Join(allowed, ", ")))
		}
	}
	oneOf("ui.theme", c.UI.Theme, themes)
	oneOf("ui.density", c.UI.Density, densities)
	for _, column := range c.UI.VirtualColumns {
		oneOf("ui.virtual_columns", column, virtualColumns)
	}
	if len(problems) > 0 {
		return errors.New("invalid config.yaml: " + strings.Join(problems, "; "))
	}
	return nil
}
