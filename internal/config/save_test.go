package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSetUIKeepsOtherSettingsAndComments(t *testing.T) {
	t.Parallel()

	data := []byte(`# Flashheart settings
version: 2
quiet_minutes: 5 # minutes
custom_key: kept
ui:
  theme: system
  density: normal
  virtual_columns: [needs-you]
  extra: preserved
`)
	ui := Defaults().UI
	ui.Theme = ThemeDark
	ui.Density = DensityCompact
	ui.ColourBy = "priority"
	ui.HiddenColumns = []string{"backlog"}
	ui.Scopes = map[string]Scope{
		"all":        {View: "board", State: "working"},
		"flashheart": {View: "table", Type: Choice{Include: []string{"bug", "spike"}}, Workstream: Choice{Exclude: []string{"board-ui"}}, Age: Choice{Include: []string{"today"}}, State: "blocked"},
	}
	got, err := SetUI(data, ui)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Flashheart settings\n", "quiet_minutes: 5 # minutes\n", "custom_key: kept\n", "  extra: preserved\n", "  theme: dark\n", "  hidden_columns: [backlog]\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("SetUI lost %q:\n%s", want, got)
		}
	}
	// The retired virtual columns setting goes on the next save (FH-44).
	if strings.Contains(string(got), "virtual_columns") {
		t.Errorf("SetUI kept virtual_columns:\n%s", got)
	}
	parsed, err := Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed.UI, ui) || parsed.QuietMinutes != 5 {
		t.Errorf("round trip UI = %+v, want %+v", parsed.UI, ui)
	}
}

func TestSetUIOnAMissingFile(t *testing.T) {
	t.Parallel()

	ui := Defaults().UI
	ui.ColourBy = "none"
	got, err := SetUI(nil, ui)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(got)
	if err != nil || parsed.Version != FormatVersion || parsed.UI.ColourBy != "none" {
		t.Errorf("SetUI(nil) = %s (%+v, %v)", got, parsed, err)
	}
}

func TestUIPreferencesAreValidated(t *testing.T) {
	t.Parallel()

	for _, ui := range []UI{
		{Theme: "sepia", Density: DensityNormal, ColourBy: "type"},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "rainbow"},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "type", Scopes: map[string]Scope{"../x": {View: "board"}}},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "type", Scopes: map[string]Scope{"alpha": {View: "map"}}},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "type", Scopes: map[string]Scope{"alpha": {State: "weird"}}},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "type", Scopes: map[string]Scope{"alpha": {Age: Choice{Include: []string{"decade"}}}}},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "type", Scopes: map[string]Scope{"alpha": {Type: Choice{Exclude: []string{strings.Repeat("x", 201)}}}}},
		{Theme: ThemeLight, Density: DensityNormal, ColourBy: "type", Scopes: map[string]Scope{"alpha": {Priority: Choice{Include: make([]string, 101)}}}},
	} {
		if _, err := SetUI(nil, ui); err == nil {
			t.Errorf("SetUI(%+v) accepted invalid preferences", ui)
		}
	}
}

func TestScopesReadTheSingleValueForm(t *testing.T) {
	t.Parallel()

	// Before FH-39 a filter was one value, and Hide later a flag.
	parsed, err := Parse([]byte(`version: 2
ui:
  scopes:
    alpha: {view: board, type: bug, priority: "", state: blocked, hide_later: true}
`))
	if err != nil {
		t.Fatal(err)
	}
	want := Scope{View: "board", Type: Choice{Include: []string{"bug"}}, State: "blocked"}
	if got := parsed.UI.Scopes["alpha"]; !reflect.DeepEqual(got, want) {
		t.Errorf("legacy scope = %+v, want %+v", got, want)
	}
}

func TestChoiceJSONHasLists(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Scope{View: "board", Type: Choice{Include: []string{"bug"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type":{"include":["bug"],"exclude":[]}`, `"priority":{"include":[],"exclude":[]}`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON %s lacks %s", data, want)
		}
	}
	if strings.Contains(string(data), "hideLater") {
		t.Errorf("JSON %s still carries hideLater", data)
	}
}
