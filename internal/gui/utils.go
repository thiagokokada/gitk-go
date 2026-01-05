package gui

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/git"
)

type treeRow struct {
	ID     string
	Graph  string
	Commit string
	Author string
	Date   string
}

func (r treeRow) values() []string {
	return []string{r.Graph, r.Commit, r.Author, r.Date}
}

func buildTreeRows(entries []*git.Entry, labels map[string][]string, graphCanvas bool) []treeRow {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]treeRow, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.Commit == nil {
			continue
		}
		id := commitRowID(entry)
		if id == "" {
			continue
		}
		msg, author, when := entry.ListColumns()
		graph := formatGraphValue(entry, labels[entry.Commit.Hash], graphCanvas)
		if graphCanvas {
			graph = ""
		}
		rows = append(rows, treeRow{
			ID:     id,
			Graph:  graph,
			Commit: msg,
			Author: author,
			Date:   when,
		})
	}
	return rows
}

func treeRowData(entry *git.Entry, labels map[string][]string, graphCanvas bool) (treeRow, bool) {
	if entry == nil || entry.Commit == nil {
		return treeRow{}, false
	}
	graph := ""
	if !graphCanvas {
		graph = formatGraphValue(entry, labels[entry.Commit.Hash], graphCanvas)
	}
	msg, author, when := entry.ListColumns()
	return treeRow{
		ID:     entry.Commit.Hash,
		Graph:  graph,
		Commit: msg,
		Author: author,
		Date:   when,
	}, true
}

func formatGraphValue(entry *git.Entry, labels []string, graphCanvas bool) string {
	graph := strings.TrimRight(entry.Graph, " ")
	if graph == "" {
		graph = "*"
	}
	if graphCanvas {
		return graph
	}
	graph += formatLabelSuffix(labels)
	return graph
}

func formatLabelSuffix(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return " [" + strings.Join(labels, ", ") + "]"
}

func filterEntries(entries []*git.Entry, query string) []*git.Entry {
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
