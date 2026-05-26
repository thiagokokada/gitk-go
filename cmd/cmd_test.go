package cmd

import "testing"

func TestThemeModeDefault(t *testing.T) {
	t.Setenv(themeModeEnvVar, "light")
	if got := themeModeDefault(); got != "light" {
		t.Fatalf("expected light env default, got %q", got)
	}

	t.Setenv(themeModeEnvVar, "invalid")
	if got := themeModeDefault(); got != "dark" {
		t.Fatalf("expected invalid env default fallback to dark, got %q", got)
	}
}
