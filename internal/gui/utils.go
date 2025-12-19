package gui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	evalext "modernc.org/tk9.0/extensions/eval"

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
	for i, entry := range entries {
		if entry == nil || entry.Commit == nil {
			continue
		}
		msg, author, when := commitListColumns(entry)
		graph := formatGraphValue(entry, labels[entry.Commit.Hash.String()], graphCanvas)
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

func tkEval(format string, a ...any) (string, error) {
	var eval = fmt.Sprintf(format, a...)
	r, err := evalext.Eval(eval)
	if err != nil {
		return "", fmt.Errorf("tk eval=%s; err=%w", eval, err)
	}
	return r, nil
}

func tkEvalOrEmpty(format string, a ...any) string {
	out, err := tkEval(format, a...)
	if err != nil {
		slog.Error(err.Error())
		return ""
	}
	return out
}

func tkMustEval(format string, a ...any) string {
	r, err := tkEval(format, a...)
	if err != nil {
		panic(err)
	}
	return r
}

func tkAtoi(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		if f, ferr := strconv.ParseFloat(raw, 64); ferr == nil {
			return int(f)
		}
		return 0
	}
	return v
}
