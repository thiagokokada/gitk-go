package gui

import (
	"fmt"
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

func treeRowValues(entry *git.Entry, labels map[string][]string, graphCanvas bool) []string {
	if entry == nil || entry.Commit == nil {
		return nil
	}
	graph := ""
	if !graphCanvas {
		graph = formatGraphValue(entry, labels[entry.Commit.Hash], graphCanvas)
	}
	msg, author, when := entry.ListColumns()
	return []string{graph, msg, author, when}
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
	return fmt.Sprintf(" [%s]", strings.Join(labels, ", "))
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
