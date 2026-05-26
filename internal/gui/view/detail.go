package view

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	tk "modernc.org/tk9.0"
)

type DiffLineTagger func(string) string

func (a *App) WriteDetailText(content string, highlightDiff bool, lineTag DiffLineTagger) {
	if a.DiffDetail == nil {
		return
	}
	a.DiffDetail.Configure(tk.State(tk.NORMAL))
	a.DiffDetail.Delete("1.0", tk.END)
	a.DiffDetail.Insert("1.0", content)
	if highlightDiff {
		a.highlightDiffLines(content, lineTag)
	} else {
		a.clearDiffLineTags()
	}
	a.DiffDetail.Configure(tk.State("disabled"))
}

func (a *App) highlightDiffLines(content string, lineTag DiffLineTagger) {
	a.clearDiffLineTags()
	if lineTag == nil {
		return
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		tag := lineTag(line)
		if tag == "" {
			continue
		}
		lineNo := i + 1
		start := textIndex(lineNo, 0)
		end := textIndex(lineNo+1, 0)
		if lineNo == len(lines) {
			end = textEndIndex(lineNo)
		}
		a.DiffDetail.TagAdd(tag, start, end)
	}
}

func (a *App) clearDiffLineTags() {
	a.DiffDetail.TagRemove("diffAdd", "1.0", tk.END)
	a.DiffDetail.TagRemove("diffDel", "1.0", tk.END)
	a.DiffDetail.TagRemove("diffHeader", "1.0", tk.END)
}

func (a *App) ScrollDiffToLine(line int) {
	if a.DiffDetail == nil || line <= 0 {
		return
	}
	totalLines := a.DetailLineCount()
	a.DiffDetail.Yviewmoveto(model.DiffScrollFraction(line, totalLines))
}

func (a *App) DetailLineCount() int {
	if a.DiffDetail == nil {
		return 0
	}
	lines, ok := TextIndexLineNumber(a.DiffDetail.Index(tk.END))
	if !ok {
		return 0
	}
	if lines > 0 {
		lines--
	}
	return lines
}

func (a *App) DiffTopLine() int {
	if a.DiffDetail == nil {
		return 0
	}
	line, ok := TextIndexLineNumber(a.DiffDetail.Index("@0,0"))
	if !ok {
		return 0
	}
	return line
}
