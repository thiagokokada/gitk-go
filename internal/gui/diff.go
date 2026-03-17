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

type diffFileChunk struct {
	path   string
	lineNo int
	header []string
	hunks  []diffHunkChunk
}

type diffHunkChunk struct {
	header string
	lineNo int
	lines  []string
}

func parseDiffChunks(diff string) []diffFileChunk {
	lines := strings.Split(diff, "\n")
	var chunks []diffFileChunk
	var current *diffFileChunk
	var currentHunk *diffHunkChunk
	for i, line := range lines {
		lineNo := i + 1
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				chunks = append(chunks, *current)
			}
			path, _ := diffPathFromLine(line)
			current = &diffFileChunk{
				path:   path,
				lineNo: lineNo,
				header: []string{line},
			}
			currentHunk = nil
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			current.hunks = append(current.hunks, diffHunkChunk{header: line, lineNo: lineNo})
			currentHunk = &current.hunks[len(current.hunks)-1]
			continue
		}
		if line == "" {
			currentHunk = nil
			continue
		}
		if currentHunk == nil {
			current.header = append(current.header, line)
			continue
		}
		currentHunk.lines = append(currentHunk.lines, line)
	}
	if current != nil {
		chunks = append(chunks, *current)
	}
	return chunks
}

func diffSectionsFromText(diffText string) []git.FileSection {
	lines := strings.Split(diffText, "\n")
	var sections []git.FileSection
	for i, line := range lines {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		path, ok := diffPathFromLine(line)
		if !ok || path == "" {
			continue
		}
		sections = append(sections, git.FileSection{Path: path, Line: i + 1})
	}
	return sections
}

func removePatchFromDiffText(diffText string, patch string) (string, []git.FileSection, bool) {
	if strings.TrimSpace(diffText) == "" || strings.TrimSpace(patch) == "" {
		return "", nil, false
	}
	lines := strings.Split(diffText, "\n")
	firstDiff := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			firstDiff = i
			break
		}
	}
	if firstDiff == -1 {
		return "", nil, false
	}
	preamble := strings.Join(lines[:firstDiff], "\n")
	bodyText := strings.Join(lines[firstDiff:], "\n")
	chunks := parseDiffChunks(bodyText)
	if len(chunks) == 0 {
		return "", nil, false
	}
	removed := false
	remaining := make([]diffFileChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if filePatch, ok := buildFilePatch(chunk); ok && filePatch == patch {
			removed = true
			continue
		}
		if len(chunk.hunks) > 0 {
			hunks := make([]diffHunkChunk, 0, len(chunk.hunks))
			for _, hunk := range chunk.hunks {
				if hunkPatch, ok := buildHunkPatch(chunk, hunk); ok && hunkPatch == patch {
					removed = true
					continue
				}
				hunks = append(hunks, hunk)
			}
			chunk.hunks = hunks
		}
		if len(chunk.hunks) == 0 {
			continue
		}
		remaining = append(remaining, chunk)
	}
	if !removed {
		return "", nil, false
	}
	var body strings.Builder
	for _, chunk := range remaining {
		if filePatch, ok := buildFilePatch(chunk); ok {
			body.WriteString(filePatch)
		}
	}
	rawText := preamble
	if rawText != "" && body.Len() > 0 {
		if !strings.HasSuffix(rawText, "\n") {
			rawText += "\n"
		}
	}
	rawText += body.String()
	sections := diffSectionsFromText(rawText)
	displayText, displaySections := prepareDiffDisplay(rawText, sections)
	return displayText, displaySections, true
}

func buildFilePatch(chunk diffFileChunk) (string, bool) {
	if len(chunk.header) == 0 {
		return "", false
	}
	lines := make([]string, 0, len(chunk.header)+len(chunk.hunks)*4)
	lines = append(lines, chunk.header...)
	for _, hunk := range chunk.hunks {
		if hunk.header == "" {
			continue
		}
		lines = append(lines, hunk.header)
		lines = append(lines, hunk.lines...)
	}
	if len(lines) == 0 {
		return "", false
	}
	return strings.Join(lines, "\n") + "\n", true
}

func buildHunkPatch(chunk diffFileChunk, hunk diffHunkChunk) (string, bool) {
	if len(chunk.header) == 0 || hunk.header == "" {
		return "", false
	}
	lines := make([]string, 0, len(chunk.header)+len(hunk.lines)+1)
	lines = append(lines, chunk.header...)
	lines = append(lines, hunk.header)
	lines = append(lines, hunk.lines...)
	return strings.Join(lines, "\n") + "\n", true
}
