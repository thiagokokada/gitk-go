package gui

import "fmt"

const (
	maxSyntaxHighlightBytes      = 2 * 1024 * 1024
	maxSyntaxHighlightLineLength = 8000
)

type syntaxHighlightMetrics struct {
	totalBytes int
}

func syntaxHighlightMetricsFor(content string) syntaxHighlightMetrics {
	return syntaxHighlightMetrics{
		totalBytes: len(content),
	}
}

func shouldSyntaxHighlight(metrics syntaxHighlightMetrics) (ok bool, reason string) {
	if metrics.totalBytes > maxSyntaxHighlightBytes {
		return false, fmt.Sprintf(
			"diff size %d bytes exceeds %d byte limit",
			metrics.totalBytes,
			maxSyntaxHighlightBytes,
		)
	}
	return true, ""
}

func shouldHighlightCodeLine(code string) bool {
	return len(code) <= maxSyntaxHighlightLineLength
}
