package view

import (
	"fmt"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
	tk "modernc.org/tk9.0"
)

type DiffLineTagger func(string) string

func (a *App) WriteDetailText(content string, highlightDiff bool, lineTag DiffLineTagger) {
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
	a.DiffDetail.TagRemove(DiffTagAdd, "1.0", tk.END)
	a.DiffDetail.TagRemove(DiffTagDelete, "1.0", tk.END)
	a.DiffDetail.TagRemove(DiffTagHeader, "1.0", tk.END)
}

func (a *App) ClearSyntaxTags(tags map[string]string) {
	for _, tag := range tags {
		a.DiffDetail.TagRemove(tag, "1.0", tk.END)
	}
}

func (a *App) ConfigureSyntaxTag(tag string, color string) {
	if tag == "" || color == "" {
		return
	}
	a.DiffDetail.TagConfigure(tag, tk.Foreground(color))
}

func (a *App) ApplySyntaxSpan(tag string, line int, startCol int, endCol int) {
	if tag == "" {
		return
	}
	a.DiffDetail.TagAdd(tag, textIndex(line, startCol), textIndex(line, endCol))
}

func (a *App) ScrollDiffToLine(line int) {
	if line <= 0 {
		return
	}
	totalLines := a.DetailLineCount()
	a.DiffDetail.Yviewmoveto(model.DiffScrollFraction(line, totalLines))
}

func (a *App) ScrollDiff(delta int, unit ScrollUnit) error {
	if delta == 0 {
		return nil
	}
	switch unit {
	case ScrollPages, ScrollUnits:
	default:
		return fmt.Errorf("unsupported diff scroll unit %q", unit)
	}
	_, err := tkutil.Evalf("%s yview scroll %d %s", a.DiffDetail, delta, unit)
	return err
}

func (a *App) SelectedDiffText() string {
	ranges := a.DiffDetail.TagRanges("sel")
	if len(ranges) < 2 {
		return ""
	}
	values := a.DiffDetail.Get(ranges[0], ranges[1])
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func StripDiffLineMarkers(text string) string {
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 0 && (line[0] == '+' || line[0] == '-') {
			line = line[1:]
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func (a *App) DetailLineCount() int {
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
	line, ok := TextIndexLineNumber(a.DiffDetail.Index("@0,0"))
	if !ok {
		return 0
	}
	return line
}
