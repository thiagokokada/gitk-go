package model

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func BuildTreeRows(entries []*git.Entry, labels map[string][]string, graphCanvas bool) []TreeRow {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]TreeRow, 0, len(entries))
	for _, entry := range entries {
		row, ok := TreeRowData(entry, labels, graphCanvas)
		if !ok {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func TreeRowData(entry *git.Entry, labels map[string][]string, graphCanvas bool) (TreeRow, bool) {
	if entry == nil || entry.Commit == nil {
		return TreeRow{}, false
	}
	graph := ""
	if !graphCanvas {
		graph = FormatGraphValue(entry, labels[entry.Commit.Hash], graphCanvas)
	}
	msg, author, when := entry.ListColumns()
	return TreeRow{
		ID:     entry.Commit.Hash,
		Graph:  graph,
		Commit: msg,
		Author: author,
		Date:   when,
	}, true
}

func FormatGraphValue(entry *git.Entry, labels []string, graphCanvas bool) string {
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
