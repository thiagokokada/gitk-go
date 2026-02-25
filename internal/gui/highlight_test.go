package gui

import (
	"strings"
	"testing"
)

func TestSyntaxHighlightMetricsForSmallContent(t *testing.T) {
	content := "a\nbb\nccc"
	metrics := syntaxHighlightMetricsFor(content)
	if metrics.totalBytes != len(content) {
		t.Fatalf("totalBytes=%d want=%d", metrics.totalBytes, len(content))
	}
}

func TestShouldSyntaxHighlightLargeDiff(t *testing.T) {
	content := strings.Repeat("a", maxSyntaxHighlightBytes+1)
	metrics := syntaxHighlightMetricsFor(content)
	ok, _ := shouldSyntaxHighlight(metrics)
	if ok {
		t.Fatalf("expected syntax highlight to be disabled for large diff")
	}
}

func TestShouldHighlightCodeLine(t *testing.T) {
	okLine := strings.Repeat("a", maxSyntaxHighlightLineLength)
	if !shouldHighlightCodeLine(okLine) {
		t.Fatalf("expected line at limit to be highlighted")
	}
	longLine := strings.Repeat("a", maxSyntaxHighlightLineLength+1)
	if shouldHighlightCodeLine(longLine) {
		t.Fatalf("expected line over limit to be skipped")
	}
}
