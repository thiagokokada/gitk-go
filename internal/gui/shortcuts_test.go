package gui

import (
	"strings"
	"testing"
)

func TestFormatShortcutsHelpText(t *testing.T) {
	bindings := []shortcutBinding{
		{category: "General", display: "/", description: "Focus the filter box"},
		{category: "General", display: "F5", description: "Reload commits"},
		{category: "", display: "x", description: "ignored (no category)"},
		{category: "Other", display: "", description: "ignored (no display)"},
		{category: "Commit list", display: "j", description: "Move down"},
	}
	got := formatShortcutsHelpText(bindings)

	if !strings.Contains(got, "General\n") {
		t.Fatalf("expected General category header, got %q", got)
	}
	if !strings.Contains(got, "Commit list\n") {
		t.Fatalf("expected Commit list category header, got %q", got)
	}

	if !strings.Contains(got, "  / — Focus the filter box") {
		t.Fatalf("expected formatted entry, got %q", got)
	}
	if !strings.Contains(got, "  F5 — Reload commits") {
		t.Fatalf("expected formatted entry, got %q", got)
	}
	if !strings.Contains(got, "  j — Move down") {
		t.Fatalf("expected formatted entry, got %q", got)
	}

	if strings.Contains(got, "ignored") {
		t.Fatalf("expected ignored bindings to be absent, got %q", got)
	}

	if !strings.Contains(got, "General\n  / — Focus the filter box\n  F5 — Reload commits\n\nCommit list\n") {
		t.Fatalf("expected blank line between categories, got %q", got)
	}
}
