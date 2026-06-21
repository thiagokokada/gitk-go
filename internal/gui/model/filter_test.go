package model

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestFilterEntries(t *testing.T) {
	entries := []git.Entry{
		{SearchText: "hello world"},
		{SearchText: "feature branch"},
	}
	filtered := FilterEntries(entries, "HELLO")
	if len(filtered) != 1 || filtered[0].SearchText != entries[0].SearchText {
		t.Fatalf("expected first entry match, got %#v", filtered)
	}
	filtered = FilterEntries(entries, " ")
	if len(filtered) != len(entries) {
		t.Fatalf("expected no filtering on blank query")
	}
}
