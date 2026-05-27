package view

import (
	"strings"
	"testing"
)

func TestSyntaxHighlightMetricsForSmallContent(t *testing.T) {
	content := "a\nbb\nccc"
	metrics := SyntaxHighlightMetricsFor(content)
	if metrics.TotalBytes != len(content) {
		t.Fatalf("TotalBytes=%d want=%d", metrics.TotalBytes, len(content))
	}
}

func TestShouldSyntaxHighlightLargeDiff(t *testing.T) {
	content := strings.Repeat("a", MaxSyntaxHighlightBytes+1)
	metrics := SyntaxHighlightMetricsFor(content)
	ok, _ := ShouldSyntaxHighlight(metrics)
	if ok {
		t.Fatalf("expected syntax highlight to be disabled for large diff")
	}
}

func TestShouldHighlightCodeLine(t *testing.T) {
	okLine := strings.Repeat("a", MaxSyntaxHighlightLineLength)
	if !ShouldHighlightCodeLine(okLine) {
		t.Fatalf("expected line at limit to be highlighted")
	}
	longLine := strings.Repeat("a", MaxSyntaxHighlightLineLength+1)
	if ShouldHighlightCodeLine(longLine) {
		t.Fatalf("expected line over limit to be skipped")
	}
}
