package gui

import "testing"

func TestParseGraphTokens(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got := parseGraphTokens("", 10)
		want := []string{"*"}
		if len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	})
	t.Run("trims and splits", func(t *testing.T) {
		got := parseGraphTokens("  *  |  |  ", 10)
		want := []string{"*", "|", "|"}
		if len(got) != len(want) {
			t.Fatalf("expected %d tokens, got %#v", len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("expected token[%d]=%q, got %q", i, want[i], got[i])
			}
		}
	})
	t.Run("caps columns", func(t *testing.T) {
		got := parseGraphTokens("* | | |", 2)
		want := []string{"*", "|"}
		if len(got) != len(want) {
			t.Fatalf("expected %d tokens, got %#v", len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("expected token[%d]=%q, got %q", i, want[i], got[i])
			}
		}
	})
}
