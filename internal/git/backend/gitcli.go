package backend

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type gitCLI struct {
	path string
}

func OpenCLI(repoPath string) (Backend, error) {
	if err := ensureMinGitVersion(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	tmp := &gitCLI{path: abs}
	root, err := tmp.runGitCommand([]string{"rev-parse", "--show-toplevel"}, false, "git rev-parse")
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("open repository: git rev-parse returned empty root")
	}
	return &gitCLI{path: root}, nil
}

func (g *gitCLI) RepoPath() string {
	if g == nil {
		return ""
	}
	return g.path
}

func (g *gitCLI) runGitCommand(args []string, allowExit1 bool, context string) (string, error) {
	if g == nil || g.path == "" {
		return "", fmt.Errorf("repository root not set")
	}
	cmdArgs := append([]string{"-C", g.path}, args...)
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
