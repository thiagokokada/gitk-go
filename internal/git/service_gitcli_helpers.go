//go:build gitcli

package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func (s *Service) runGitCommand(args []string, allowExit1 bool, context string) (string, error) {
	if s.repoPath == "" {
		return "", fmt.Errorf("repository root not set")
	}
	cmdArgs := append([]string{"-C", s.repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if allowExit1 && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && stderr.Len() == 0 {
			// treat as success when git diff signals changes via exit code 1
		} else {
			if stderr.Len() > 0 {
				return "", fmt.Errorf("%s: %v: %s", context, err, strings.TrimSpace(stderr.String()))
			}
			return "", fmt.Errorf("%s: %w", context, err)
		}
	}
	return stdout.String(), nil
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
