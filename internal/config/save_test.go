package config

import (
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
	ui.Scopes = map[string]Scope{
		"all":        {View: "board", State: "all"},
		"flashheart": {View: "table", Type: "bug", State: "blocked", HideLater: true},
	}
	got, err := SetUI(data, ui)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Flashheart settings\n", "quiet_minutes: 5 # minutes\n", "custom_key: kept\n", "  extra: preserved\n", "  theme: dark\n", "  virtual_columns: [needs-you]\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("SetUI lost %q:\n%s", want, got)
		}
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
	} {
		if _, err := SetUI(nil, ui); err == nil {
			t.Errorf("SetUI(%+v) accepted invalid preferences", ui)
		}
	}
}
