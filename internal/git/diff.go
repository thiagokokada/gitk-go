package git

import (
	"fmt"
	"strings"
)

func (s *Service) Diff(commit *Commit) (string, []FileSection, error) {
	if commit == nil {
		return "", nil, fmt.Errorf("commit not specified")
	}
	header := FormatCommitHeader(commit)
	diffText, err := s.commitDiffText(commit)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(diffText) == "" {
		return header + "\nNo file level changes.", nil, nil
	}
	if !strings.HasSuffix(header, "\n") {
		header += "\n"
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(diffText)
	if !strings.HasSuffix(diffText, "\n") {
		b.WriteByte('\n')
	}
	lineOffset := strings.Count(header, "\n")
	sections := parseGitDiffSections(diffText, lineOffset)
	return b.String(), sections, nil
}

func (s *Service) commitDiffText(commit *Commit) (string, error) {
	if len(commit.ParentHashes) > 0 {
		parent := commit.ParentHashes[0]
		return s.backend.CommitDiffText(commit.Hash, parent)
	}
	return s.backend.CommitDiffText(commit.Hash, "")
}

func parseGitDiffSections(diffText string, lineOffset int) []FileSection {
	lines := strings.Split(diffText, "\n")
	var sections []FileSection
	for i, line := range lines {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		if path := parseGitDiffPath(line); path != "" {
			sections = append(sections, FileSection{Path: path, Line: lineOffset + i + 1})
		}
	}
	return sections
}

func parseGitDiffPath(line string) string {
	const prefix = "diff --git "
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	tokens := diffLineTokens(strings.TrimSpace(line[len(prefix):]))
	if len(tokens) < 2 {
		return ""
	}
	return normalizeDiffPath(tokens[1])
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
