package gui

import (
	"fmt"
	"strconv"
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

func buildTreeRows(entries []*git.Entry, labels map[string][]string) []treeRow {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]treeRow, 0, len(entries))
	for i, entry := range entries {
		if entry == nil || entry.Commit == nil {
			continue
		}
		msg, author, when := commitListColumns(entry)
		graph := formatGraphValue(entry, labels[entry.Commit.Hash.String()])
		rows = append(rows, treeRow{
			ID:     strconv.Itoa(i),
			Graph:  graph,
			Commit: msg,
			Author: author,
			Date:   when,
		})
	}
	return rows
}

func commitListColumns(entry *git.Entry) (msg, author, when string) {
	firstLine := strings.SplitN(strings.TrimSpace(entry.Commit.Message), "\n", 2)[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:77] + "..."
	}
	msg = fmt.Sprintf("%s  %s", entry.Commit.Hash.String()[:7], firstLine)
	author = fmt.Sprintf("%s <%s>", entry.Commit.Author.Name, entry.Commit.Author.Email)
	when = entry.Commit.Committer.When.Format("2006-01-02 15:04")
	return
}

func formatGraphValue(entry *git.Entry, labels []string) string {
	graph := strings.TrimRight(entry.Graph, " ")
	if graph == "" {
		graph = "*"
	}
	if len(labels) != 0 {
		label := fmt.Sprintf("[%s]", strings.Join(labels, ", "))
		if graph != "" {
			graph += " "
		}
		graph += label
	}
	return graph
}

func tclList(values ...string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("\"%s\"", escapeTclString(v))
	}
	return strings.Join(parts, " ")
}

func escapeTclString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
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
