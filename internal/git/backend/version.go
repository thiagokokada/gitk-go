package backend

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Minimum supported git version for the CLI backend. Keep this aligned with the
// flags and subcommands we use across the project (e.g. "git switch" and
// "git status --porcelain=v2").
var minGitVersion = gitVersion{major: 2, minor: 23, patch: 0}

type gitVersion struct {
	major int
	minor int
	patch int
}

func MinGitVersion() string {
	return minGitVersion.String()
}

func (v gitVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}

func (v gitVersion) less(other gitVersion) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

func parseGitVersionOutput(out string) (gitVersion, bool) {
	s := strings.TrimSpace(out)
	if s == "" {
		return gitVersion{}, false
	}
	// Common formats:
	// - "git version 2.44.0"
	// - "git version 2.39.3 (Apple Git-146)"
	// - "git version 2.39.3.windows.1"
	if idx := strings.Index(s, "git version"); idx >= 0 {
		s = strings.TrimSpace(s[idx+len("git version"):])
	}
	// Find first digit to tolerate vendor suffixes/prefixes.
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return gitVersion{}, false
	}
	s = s[start:]
	// Keep only the leading numeric/dot portion (e.g. "2.39.3" from "2.39.3.windows.1").
	end := 0
	for end < len(s) {
		c := s[end]
		if (c >= '0' && c <= '9') || c == '.' {
			end++
			continue
		}
		break
	}
	s = strings.Trim(s[:end], ".")
	if s == "" {
		return gitVersion{}, false
	}

	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return gitVersion{}, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return gitVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return gitVersion{}, false
	}
	patch := 0
	if len(parts) >= 3 {
		if p, err := strconv.Atoi(parts[2]); err == nil {
			patch = p
		}
	}
	return gitVersion{major: major, minor: minor, patch: patch}, true
}

type gitVersionInfo struct {
	out    string
	parsed gitVersion
	ok     bool
	err    error
}

func validateGitVersionOutput(out string) error {
	got, ok := parseGitVersionOutput(out)
	if !ok {
		return fmt.Errorf("unable to parse git version output: %q", strings.TrimSpace(out))
	}
	if got.less(minGitVersion) {
		return fmt.Errorf("git %s is too old; gitk-go requires git >= %s", got, minGitVersion)
	}
	return nil
}

var (
	gitVersionOnce      sync.Once
	gitVersionInfoCache gitVersionInfo
)

func gitVersionInfoCached() gitVersionInfo {
	gitVersionOnce.Do(func() {
		outBytes, err := exec.Command("git", "--version").CombinedOutput()
		out := strings.TrimSpace(string(outBytes))
		gitVersionInfoCache.out = out
		if err != nil {
			if out != "" {
				gitVersionInfoCache.err = fmt.Errorf("git --version: %v: %s", err, out)
				return
			}
			gitVersionInfoCache.err = fmt.Errorf("git --version: %w", err)
			return
		}
		parsed, ok := parseGitVersionOutput(out)
		gitVersionInfoCache.parsed = parsed
		gitVersionInfoCache.ok = ok
		if !ok {
			gitVersionInfoCache.err = fmt.Errorf("unable to parse git version output: %q", out)
			return
		}
	})
	return gitVersionInfoCache
}

func GitVersion() (string, error) {
	info := gitVersionInfoCached()
	return info.out, info.err
}

var (
	minGitVersionOnce sync.Once
	minGitVersionErr  error
)

func ensureMinGitVersion() error {
	minGitVersionOnce.Do(func() {
		info := gitVersionInfoCached()
		if info.err != nil {
			minGitVersionErr = info.err
			return
		}
		if !info.ok {
			minGitVersionErr = fmt.Errorf("unable to parse git version output: %q", info.out)
			return
		}
		if info.parsed.less(minGitVersion) {
			minGitVersionErr = fmt.Errorf("git %s is too old; gitk-go requires git >= %s", info.parsed, minGitVersion)
		}
	})
	return minGitVersionErr
}
