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

func diffPathFromLine(line string) (string, bool) {
	const prefix = "diff --git "
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	segment := strings.TrimSpace(line[len(prefix):])
	tokens := diffLineTokens(segment)
	if len(tokens) < 2 {
		return "", true
	}
	return normalizeDiffPath(tokens[1]), true
}

func diffLineTokens(s string) []string {
	var tokens []string
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			break
		}
		if s[0] == '"' {
			var buf strings.Builder
			escaped := false
			i := 1
			for i < len(s) {
				ch := s[i]
				if escaped {
					buf.WriteByte(ch)
					escaped = false
					i++
					continue
				}
				if ch == '\\' {
					escaped = true
					i++
					continue
				}
				if ch == '"' {
					i++
					break
				}
				buf.WriteByte(ch)
				i++
			}
			tokens = append(tokens, buf.String())
			s = s[i:]
			continue
		}
		j := 0
		for j < len(s) && s[j] != ' ' && s[j] != '\t' {
			j++
		}
		tokens = append(tokens, s[:j])
		s = s[j:]
	}
	return tokens
}

func normalizeDiffPath(token string) string {
	token = strings.TrimPrefix(token, "a/")
	token = strings.TrimPrefix(token, "b/")
	return token
}

func diffLineCode(line string) (string, int, bool) {
	if line == "" {
		return "", 0, false
	}
	switch line[0] {
	case '+', '-', ' ':
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			return "", 0, false
		}
		if strings.HasPrefix(line, "\\ ") {
			return "", 0, false
		}
		return line[1:], 1, true
	default:
		return "", 0, false
	}
}
