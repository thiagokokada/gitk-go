package gui

import "strings"

func diffLineTag(line string) string {
	switch {
	case strings.HasPrefix(line, "diff --git"):
		return "diffHeader"
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return "diffAdd"
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return "diffDel"
	default:
		return ""
	}
}
