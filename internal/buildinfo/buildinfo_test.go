package buildinfo

import "testing"

func TestStringNamesEveryField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info Info
		want string
	}{
		{"release", Info{Version: "v0.1.0", Commit: "abc1234", BuildDate: "2026-10-04"}, "flashheart v0.1.0 (commit abc1234, built 2026-10-04)"},
		{"empty values", Info{}, "flashheart unknown (commit unknown, built unknown)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.info.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCurrentDefaultsToDevelopmentBuild(t *testing.T) {
	t.Parallel()

	if got := Current(); got.Version != "dev" || got.Commit != "unknown" {
		t.Errorf("Current() = %+v, want development defaults", got)
	}
}
