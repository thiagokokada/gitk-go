package gui

import (
	"fmt"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
)

func detailStatusText(header, body string) string {
	header = strings.TrimRight(header, "\n")
	body = strings.TrimLeft(body, "\n")
	if header == "" {
		return body
	}
	if body == "" {
		return header
	}
	return fmt.Sprintf("%s\n%s", header, body)
}

func (a *Controller) showDiffStatus(header, body string) {
	a.clearDetailText(detailStatusText(header, body))
}

func (a *Controller) showRenderedDiff(diff string, sections []git.FileSection) {
	diff, sections = model.PrepareDiffDisplay(diff, sections)
	a.writeDetailText(diff, len(sections) > 0)
	a.setFileSections(sections)
}
