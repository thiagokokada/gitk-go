package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func (s *Service) runGitCommand(args []string, allowExit1 bool, context string) (string, error) {
	if s.repo.path == "" {
		return "", fmt.Errorf("repository root not set")
	}
	cmdArgs := append([]string{"-C", s.repo.path}, args...)
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
