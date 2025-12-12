//go:build gitcli

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
)

func (s *Service) LocalChanges() (LocalChanges, error) {
	var res LocalChanges
	if s.repo.path == "" {
		return res, fmt.Errorf("repository root not set")
	}
	args := []string{"-C", s.repo.path, "status", "--porcelain=v2"}
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return res, fmt.Errorf("git status: %v: %s", err, stderr.String())
		}
		return res, fmt.Errorf("git status: %w", err)
	}
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case '1', '2', 'u':
			if len(line) < 4 {
				continue
			}
			stagedState := line[2]
			worktreeState := line[3]
			if stagedState != '.' {
				res.HasStaged = true
			}
			if worktreeState != '.' && worktreeState != '?' {
				res.HasWorktree = true
			}
		default:
			// '?' untracked, '!' ignored, etc.
		}
		if res.HasWorktree && res.HasStaged {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return res, fmt.Errorf("parse git status: %w", err)
	}
	return res, nil
}
