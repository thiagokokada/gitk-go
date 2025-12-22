package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type scanSession struct {
	head     string
	headName string

	logStream *gitLogStream

	// buffered holds the next commit returned by hasMore() so ScanCommits can keep consuming in-order.
	buffered  *Commit
	exhausted bool
	returned  uint

	graphBuilder   *graphBuilder
	graphCache     map[string]string
	graphProcessed int
	graphColsMax   int
	graphEOF       bool
}

func (s *Service) ensureScanSessionLocked(headHash, headName string) error {
	if s.scan != nil && s.scan.head == headHash {
		return nil
	}
	return s.resetScanLocked(headHash, headName)
}

func (s *Service) resetScanLocked(headHash, headName string) error {
	if s.scan != nil {
		s.scan.close()
		s.scan = nil
	}
	if s.repo.path == "" {
		return fmt.Errorf("repository root not set")
	}
	stream, err := startGitLogStream(s.repo.path, headHash)
	if err != nil {
		return err
	}
	s.scan = &scanSession{
		head:       headHash,
		headName:   headName,
		logStream:  stream,
		graphEOF:   false,
		exhausted:  false,
		graphCache: make(map[string]string, DefaultBatch),

		graphBuilder: newGraphBuilder(s.graphMaxColumns),
	}
	slog.Debug("ScanCommits session initialized", slog.String("head", s.scan.headName))
	return nil
}

func (s *scanSession) close() {
	if s.logStream != nil {
		if err := s.logStream.Close(); err != nil {
			slog.Debug("git log stream close", slog.Any("error", err))
		}
	}
	s.logStream = nil
	s.buffered = nil
	s.exhausted = true
	s.graphEOF = true
}

func (s *scanSession) hasMore() (bool, error) {
	if s.exhausted {
		return false, nil
	}
	if s.buffered != nil {
		return true, nil
	}
	commit, err := s.readNextCommit()
	if err != nil {
		if err == io.EOF {
			s.exhausted = true
			return false, nil
		}
		return false, fmt.Errorf("iterate commits: %w", err)
	}
	s.buffered = commit
	return true, nil
}

func (s *scanSession) next() (*Commit, error) {
	if s.exhausted {
		return nil, io.EOF
	}
	if s.buffered != nil {
		commit := s.buffered
		s.buffered = nil
		s.returned++
		return commit, nil
	}
	commit, err := s.readNextCommit()
	if err != nil {
		if err == io.EOF {
			s.exhausted = true
		}
		return nil, err
	}
	s.returned++
	return commit, nil
}

func (s *scanSession) discard(count uint) error {
	for range count {
		if _, err := s.next(); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanSession) ensureGraphProcessed(target uint) error {
	// Graph lines are computed while commits are streamed; there is no separate iterator to
	// advance without affecting displayed commits.
	return nil
}

func (s *scanSession) assignGraphStrings(entries []*Entry) {
	if len(entries) == 0 || len(s.graphCache) == 0 {
		return
	}
	for _, entry := range entries {
		entry.Graph = s.graphCache[entry.Commit.Hash]
	}
}

func (s *scanSession) readNextCommit() (*Commit, error) {
	if s.graphEOF || s.logStream == nil || s.graphBuilder == nil {
		return nil, io.EOF
	}
	commit, err := s.logStream.Next()
	if err != nil {
		if err == io.EOF {
			s.graphEOF = true
		}
		return nil, err
	}
	line := s.graphBuilder.Line(commit)
	s.graphProcessed++
	s.graphCache[commit.Hash] = line
	if cols := len(s.graphBuilder.columns); cols > s.graphColsMax {
		s.graphColsMax = cols
	}
	return commit, nil
}

type gitLogStream struct {
	cancel context.CancelFunc
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr bytes.Buffer
	r      *bufio.Reader

	waitOnce sync.Once
	waitErr  error
}

func startGitLogStream(repoPath string, fromHash string) (*gitLogStream, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository root not set")
	}
	fromHash = strings.TrimSpace(fromHash)
	if fromHash == "" {
		return nil, fmt.Errorf("starting commit not specified")
	}
	// NUL-delimited records; commit message cannot contain NUL.
	const format = "%H%n%P%n%an%n%ae%n%aI%n%cn%n%ce%n%cI%n%B%x00"

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(
		ctx,
		"git",
		"--no-pager",
		"-C",
		repoPath,
		"log",
		"--no-color",
		"--no-decorate",
		"--date-order",
		"--no-patch",
		// Use tformat to avoid git log adding an extra newline after each record.
		"--pretty=tformat:"+format,
		fromHash,
	)
	var stream gitLogStream
	stream.cancel = cancel
	stream.cmd = cmd
	cmd.Stderr = &stream.stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("git log stdout: %w", err)
	}
	stream.stdout = stdout
	stream.r = bufio.NewReader(stdout)
	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdout.Close()
		if stream.stderr.Len() > 0 {
			return nil, fmt.Errorf("git log start: %v: %s", err, strings.TrimSpace(stream.stderr.String()))
		}
		return nil, fmt.Errorf("git log start: %w", err)
	}
	return &stream, nil
}

func (s *gitLogStream) Next() (*Commit, error) {
	rec, err := s.r.ReadBytes(0)
	if err != nil {
		if err == io.EOF {
			if waitErr := s.wait(); waitErr != nil {
				return nil, waitErr
			}
			return nil, io.EOF
		}
		return nil, err
	}
	if len(rec) == 0 {
		return nil, io.EOF
	}
	// Strip trailing NUL.
	rec = rec[:len(rec)-1]
	// git log prints a newline between commits even when the format ends with NUL,
	// so subsequent records can start with '\n'.
	for len(rec) > 0 && (rec[0] == '\n' || rec[0] == '\r') {
		rec = rec[1:]
	}
	if len(rec) == 0 {
		return nil, fmt.Errorf("unexpected empty git log record")
	}
	commit, err := parseGitLogRecord(rec)
	if err != nil {
		return nil, err
	}
	return commit, nil
}

func (s *gitLogStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	return s.wait()
}

func (s *gitLogStream) wait() error {
	s.waitOnce.Do(func() {
		s.waitErr = s.cmd.Wait()
	})
	if s.waitErr == nil {
		return nil
	}
	if s.stderr.Len() > 0 {
		return fmt.Errorf("git log: %v: %s", s.waitErr, strings.TrimSpace(s.stderr.String()))
	}
	return fmt.Errorf("git log: %w", s.waitErr)
}

func parseGitLogRecord(rec []byte) (*Commit, error) {
	parts := strings.Split(string(rec), "\n")
	if len(parts) < 8 {
		return nil, fmt.Errorf("unexpected git log record: got %d lines", len(parts))
	}
	hashStr := strings.TrimSpace(parts[0])
	if hashStr == "" {
		return nil, fmt.Errorf("missing commit hash")
	}
	var parents []string
	parentLine := strings.TrimSpace(parts[1])
	if parentLine != "" {
		parents = append(parents, strings.Fields(parentLine)...)
	}
	authorName := parts[2]
	authorEmail := parts[3]
	authorWhen, _ := time.Parse(time.RFC3339, parts[4])
	committerName := parts[5]
	committerEmail := parts[6]
	committerWhen, _ := time.Parse(time.RFC3339, parts[7])
	message := ""
	if len(parts) > 8 {
		message = strings.Join(parts[8:], "\n")
	}
	return &Commit{
		Hash:         hashStr,
		ParentHashes: parents,
		Author:       Signature{Name: authorName, Email: authorEmail, When: authorWhen},
		Committer:    Signature{Name: committerName, Email: committerEmail, When: committerWhen},
		Message:      message,
	}, nil
}
