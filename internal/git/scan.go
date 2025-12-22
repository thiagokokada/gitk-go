package git

import (
	"fmt"
	"io"
	"log/slog"

	gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"
)

type scanSession struct {
	head     string
	headName string

	logStream gitbackend.LogStream

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
	if s.backend == nil || s.backend.RepoPath() == "" {
		return fmt.Errorf("repository root not set")
	}
	stream, err := s.backend.StartLogStream(headHash)
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
