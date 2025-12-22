package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type repo struct {
	path string
}

func (r repo) RepoPath() string {
	return r.path
}

func openRepo(repoPath string) (repo, error) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return repo{}, err
	}
	tmp := repo{path: abs}
	root, err := tmp.runGitCommand([]string{"rev-parse", "--show-toplevel"}, false, "git rev-parse")
	if err != nil {
		return repo{}, fmt.Errorf("open repository: %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return repo{}, fmt.Errorf("open repository: git rev-parse returned empty root")
	}
	return repo{path: root}, nil
}

func (r repo) runGitCommand(args []string, allowExit1 bool, context string) (string, error) {
	if r.path == "" {
		return "", fmt.Errorf("repository root not set")
	}
	cmdArgs := append([]string{"-C", r.path}, args...)
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
