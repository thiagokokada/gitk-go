package gui

import (
	"strings"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func fileSectionIndexForLine(sections []git.FileSection, line int) int {
	if len(sections) == 0 || line <= 0 {
		return 0
	}
	target := 0
	for i, sec := range sections {
		if line < sec.Line {
			break
		}
		target = i
	}
	return target
}

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

func prepareDiffDisplay(content string, sections []git.FileSection) (string, []git.FileSection) {
	if content == "" {
		return content, sections
	}
	lines := strings.Split(content, "\n")
	var b strings.Builder
	newSections := make([]git.FileSection, len(sections))
	copy(newSections, sections)
	extraLines := 0
	nextSection := 0
	for i, line := range lines {
		lineNo := i + 1
		for nextSection < len(newSections) && newSections[nextSection].Line == lineNo {
			newSections[nextSection].Line = lineNo + extraLines
			nextSection++
		}
		if strings.HasPrefix(line, "diff --git ") && b.Len() > 0 {
			b.WriteString("\n")
			extraLines++
		}
		b.WriteString(line)
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	for nextSection < len(newSections) {
		newSections[nextSection].Line += extraLines
		nextSection++
	}
	return b.String(), newSections
}
