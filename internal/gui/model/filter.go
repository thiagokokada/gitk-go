package model

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/git"
)

type FilterState struct {
	Value string
}

func FilterEntries(entries []*git.Entry, query string) []*git.Entry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return entries
	}
	var filtered []*git.Entry
	for _, entry := range entries {
		if strings.Contains(entry.SearchText, q) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
