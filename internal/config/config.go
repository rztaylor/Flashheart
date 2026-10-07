package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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
	colourBys      = []string{"type", "priority", "age", "none"}
	views          = []string{"", "board", "agents", "workstreams", "table"}
	states         = []string{"", "all", "blocked", "unblocked", "repair"}
	scopeName      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,254}$`)
)

// Config is the global configuration (CFG-1, CFG-2).
type Config struct {
	Version            int  `yaml:"version"`
	AutoCreateProjects bool `yaml:"auto_create_projects"`
	QuietMinutes       int  `yaml:"quiet_minutes"`
	// LeaseMinutes is how long a claim outlives its run's activity.
	LeaseMinutes int `yaml:"lease_minutes"`
	// EnforceHandoff blocks a stop once for a checkpoint (HOOK-6); a
	// project's settings.enforce_handoff overrides it.
	EnforceHandoff     bool        `yaml:"enforce_handoff"`
	EventRetentionDays int         `yaml:"event_retention_days"`
	DoneColumnLimit    int         `yaml:"done_column_limit"`
	Attachments        Attachments `yaml:"attachments"`
	UI                 UI          `yaml:"ui"`
	// UserName is who answers agents' questions from the board (RUN-8);
	// empty means the computer account's name.
	UserName string `yaml:"user_name"`
}

// Answerer is the name an answer from the board is recorded under: the
// configured user_name, else the account's full name, else its login.
func (c Config) Answerer(account func() (*user.User, error)) string {
	if name := strings.TrimSpace(c.UserName); name != "" {
		return name
	}
	if current, err := account(); err == nil && current != nil {
		if name := strings.TrimSpace(current.Name); name != "" {
			return name
		}
		if login := strings.TrimSpace(current.Username); login != "" {
			return login
		}
	}
	return "the user"
}

// Attachments holds attachment limits.
type Attachments struct {
	MaxBytes int64 `yaml:"max_bytes"`
}

// UI holds interface preferences saved through the backend (CFG-2).
type UI struct {
	Theme          string   `yaml:"theme"`
	Density        string   `yaml:"density"`
	ColourBy       string   `yaml:"colour_by"`
	VirtualColumns []string `yaml:"virtual_columns"`
	// Scopes remembers the view and filters per project name, or "all" for
	// All projects (VIEW-7).
	Scopes map[string]Scope `yaml:"scopes,omitempty"`
}

// Scope is the remembered view and filters of one project or All projects.
type Scope struct {
	View       string `yaml:"view,omitempty" json:"view"`
	Type       string `yaml:"type,omitempty" json:"type"`
	Priority   string `yaml:"priority,omitempty" json:"priority"`
	Workstream string `yaml:"workstream,omitempty" json:"workstream"`
	State      string `yaml:"state,omitempty" json:"state"`
	HideLater  bool   `yaml:"hide_later,omitempty" json:"hideLater"`
}

// Defaults returns the documented defaults.
func Defaults() Config {
	return Config{
		Version:            FormatVersion,
		AutoCreateProjects: true,
		QuietMinutes:       10,
		LeaseMinutes:       30,
		EventRetentionDays: 90,
		DoneColumnLimit:    20,
		Attachments:        Attachments{MaxBytes: 20 << 20},
		UI:                 UI{Theme: ThemeSystem, Density: DensityNormal, ColourBy: "type", VirtualColumns: []string{"needs-you"}},
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
	atLeastOne("lease_minutes", int64(c.LeaseMinutes))
	atLeastOne("event_retention_days", int64(c.EventRetentionDays))
	atLeastOne("done_column_limit", int64(c.DoneColumnLimit))
	atLeastOne("attachments.max_bytes", c.Attachments.MaxBytes)
	if name := c.UserName; utf8.RuneCountInString(name) > 80 || strings.ContainsFunc(name, unicode.IsControl) {
		problems = append(problems, "user_name must be one line of at most 80 characters")
	}
	problems = append(problems, c.UI.problems()...)
	if len(problems) > 0 {
		return errors.New("invalid config.yaml: " + strings.Join(problems, "; "))
	}
	return nil
}

func (u UI) problems() []string {
	var problems []string
	oneOf := func(name, value string, allowed []string) {
		if !slices.Contains(allowed, value) {
			problems = append(problems, fmt.Sprintf("%s %q must be one of %s", name, value, strings.Join(slices.DeleteFunc(slices.Clone(allowed), func(v string) bool { return v == "" }), ", ")))
		}
	}
	oneOf("ui.theme", u.Theme, themes)
	oneOf("ui.density", u.Density, densities)
	oneOf("ui.colour_by", u.ColourBy, colourBys)
	for _, column := range u.VirtualColumns {
		oneOf("ui.virtual_columns", column, virtualColumns)
	}
	for name, scope := range u.Scopes {
		if !scopeName.MatchString(name) || strings.Contains(name, "..") {
			problems = append(problems, fmt.Sprintf("ui.scopes %q is not a project name", name))
			continue
		}
		oneOf("ui.scopes."+name+".view", scope.View, views)
		oneOf("ui.scopes."+name+".state", scope.State, states)
		for _, value := range []string{scope.Type, scope.Priority, scope.Workstream} {
			if len(value) > 200 {
				problems = append(problems, "ui.scopes."+name+" has a filter longer than 200 characters")
			}
		}
	}
	return problems
}
