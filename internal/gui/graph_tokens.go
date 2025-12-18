package gui

import "strings"

func parseGraphTokens(raw string, maxCols int) []string {
	tokens := strings.Fields(strings.TrimSpace(raw))
	if len(tokens) == 0 {
		// Keep the graph legible even if the backend didn't populate a graph string yet.
		// This matches the list view's textual fallback.
		return []string{"*"}
	}
	if maxCols > 0 && len(tokens) > maxCols {
		tokens = tokens[:maxCols]
	}
	return tokens
}
