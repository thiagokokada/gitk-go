package gui

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/view"
)

func diffLineTag(line string) string {
	switch {
	case strings.HasPrefix(line, "diff --git"):
		return view.DiffTagHeader
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return view.DiffTagAdd
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return view.DiffTagDelete
	default:
		return ""
	}
}
