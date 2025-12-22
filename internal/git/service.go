package git

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"
)

const DefaultBatch = 1000

const DefaultGraphMaxColumns = 200

type Service struct {
	// mu serializes access to repo operations that share iterators/state (scan session).
	mu sync.Mutex

	backend gitbackend.Backend
	scan    *scanSession

	graphMaxColumns int
}

type Entry struct {
	Commit     *Commit
	Summary    string
	SearchText string
	Graph      string
}

type FileSection struct {
	Path string
	Line int
}

func Open(repoPath string) (*Service, error) {
	backend, err := gitbackend.OpenCLI(repoPath)
	if err != nil {
		return nil, err
	}
	return &Service{
		backend:         backend,
		graphMaxColumns: DefaultGraphMaxColumns,
	}, nil
}

func NewWithBackend(backend gitbackend.Backend) *Service {
	return &Service{
		backend:         backend,
		graphMaxColumns: DefaultGraphMaxColumns,
	}
}

func (s *Service) RepoPath() string {
	if s.backend == nil {
		return ""
	}
	return s.backend.RepoPath()
}

func (s *Service) SetGraphMaxColumns(maxColumns int) {
	if maxColumns <= 0 {
		maxColumns = DefaultGraphMaxColumns
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.graphMaxColumns = maxColumns
	if s.scan != nil && s.scan.graphBuilder != nil {
		s.scan.graphBuilder.maxColumns = maxColumns
		s.scan.graphBuilder.trim()
	}
}

func (s *Service) ScanCommits(skip, batch uint) ([]*Entry, string, bool, error) {
	slog.Debug("ScanCommits start", slog.Uint64("skip", uint64(skip)), slog.Uint64("batch", uint64(batch)))
	startTotal := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	startHead := time.Now()
	headHash, headName, ok, err := s.headStateLocked()
	if err != nil {
		return nil, "", false, fmt.Errorf("resolve HEAD: %w", err)
	}
	if !ok {
		if s.scan != nil {
			s.scan.close()
			s.scan = nil
		}
		return nil, "", false, nil
	}
	headDur := time.Since(startHead)

	startSession := time.Now()
	if err := s.ensureScanSessionLocked(headHash, headName); err != nil {
		return nil, "", false, err
	}
	sessionDur := time.Since(startSession)
	// If the caller requests a different position than the current session, reset and advance to skip.
	if skip != s.scan.returned {
		if err := s.alignSessionLocked(skip, headHash, headName); err != nil {
			if err == io.EOF {
				return nil, s.scan.headName, false, nil
			}
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
	}

	startIter := time.Now()
	entries, err := s.collectEntries(batch)
	if err != nil {
		return nil, "", false, fmt.Errorf("iterate commits: %w", err)
	}
	iterDur := time.Since(startIter)

	graphTarget := skip + uint(len(entries))
	startGraph := time.Now()
	s.scan.assignGraphStrings(entries)
	graphDur := time.Since(startGraph)

	startMore := time.Now()
	hasMore, err := s.scan.hasMore()
	if err != nil {
		return nil, "", false, err
	}
	hasMoreDur := time.Since(startMore)
	totalDur := time.Since(startTotal)

	// dur_* fields represent the wall time spent in each ScanCommits stage.
	slog.Debug("ScanCommits done",
		slog.Int("returned", len(entries)),
		slog.Uint64("session_returned", uint64(s.scan.returned)),
		slog.Bool("has_more", hasMore),
		slog.String("head", s.scan.headName),
		slog.Uint64("graph_target", uint64(graphTarget)),
		slog.Int("graph_processed", s.scan.graphProcessed),
		slog.Int("graph_cache_len", len(s.scan.graphCache)),
		slog.Int("graph_cols", len(s.scan.graphBuilder.columns)),
		slog.Int("graph_cols_max", s.scan.graphColsMax),
		slog.Duration("dur_total", totalDur),
		slog.Duration("dur_head", headDur),
		slog.Duration("dur_session", sessionDur),
		slog.Duration("dur_iter", iterDur),
		slog.Duration("dur_graph", graphDur),
		slog.Duration("dur_has_more", hasMoreDur),
	)
	return entries, s.scan.headName, hasMore, nil
}
func (s *Service) alignSessionLocked(skip uint, headHash, headName string) error {
	start := time.Now()
	slog.Debug("ScanCommits reset session",
		slog.Uint64("requested_skip", uint64(skip)),
		slog.Uint64("session_returned", uint64(s.scan.returned)),
		slog.String("head", s.scan.headName),
	)
	if err := s.resetScanLocked(headHash, headName); err != nil {
		return err
	}
	if err := s.scan.discard(skip); err != nil {
		return err
	}
	slog.Debug("ScanCommits reset session done", slog.Duration("dur", time.Since(start)))
	return nil
}

func (s *Service) collectEntries(batch uint) ([]*Entry, error) {
	entries := make([]*Entry, 0, max(batch, DefaultBatch))
	for uint(len(entries)) < batch {
		commit, err := s.scan.next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		entries = append(entries, newEntry(commit))
	}
	return entries, nil
}

func (s *Service) headStateLocked() (hash string, headName string, ok bool, err error) {
	if s.backend == nil {
		return "", "", false, fmt.Errorf("repository root not set")
	}
	return s.backend.HeadState()
}

func FormatCommitHeader(c *Commit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "commit %s\n", c.Hash)
	appendSignatureLine(&b, "Author", c.Author)
	committer := c.Committer
	if committer.Name == "" && committer.Email == "" && committer.When.IsZero() {
		committer = c.Author
	}
	appendSignatureLine(&b, "Committer", committer)
	b.WriteString("\n")
	message := strings.TrimRight(c.Message, "\n")
	if message == "" {
		b.WriteString("    (no commit message)\n")
		return b.String()
	}
	for line := range strings.SplitSeq(message, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(&b, "    %s\n", line)
	}
	return b.String()
}

func appendSignatureLine(b *strings.Builder, label string, sig Signature) {
	fmt.Fprintf(b, "%s: %s <%s>", label, sig.Name, sig.Email)
	if !sig.When.IsZero() {
		fmt.Fprintf(b, "  %s", sig.When.Format("2006-01-02 15:04:05 -0700"))
	}
	b.WriteByte('\n')
}

func newEntry(c *Commit) *Entry {
	summary := formatSummary(c)
	var b strings.Builder
	b.WriteString(strings.ToLower(c.Hash))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Name))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Email))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Message))
	return &Entry{Commit: c, Summary: summary, SearchText: b.String()}
}

func formatSummary(c *Commit) string {
	firstLine := strings.SplitN(strings.TrimSpace(c.Message), "\n", 2)[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:77] + "..."
	}
	timestamp := c.Committer.When.Format("2006-01-02 15:04")
	hash := c.Hash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return fmt.Sprintf("%s  %s  %s", hash, timestamp, firstLine)
}

func localDiffHeader(staged bool) string {
	if staged {
		return "Local changes checked into index but not committed"
	}
	return "Local uncommitted changes, not checked in to index"
}

type graphBuilder struct {
	columns    []string
	maxColumns int
}

func newGraphBuilder(maxColumns int) *graphBuilder {
	if maxColumns <= 0 {
		maxColumns = DefaultGraphMaxColumns
	}
	return &graphBuilder{maxColumns: maxColumns}
}

func (g *graphBuilder) trim() {
	if g.maxColumns <= 0 {
		return
	}
	if len(g.columns) > g.maxColumns {
		g.columns = g.columns[:g.maxColumns]
	}
}

func (g *graphBuilder) Line(c *Commit) string {
	idx := g.columnIndex(c.Hash)
	if idx == -1 {
		g.columns = append([]string{c.Hash}, g.columns...)
		idx = 0
	}
	g.trim()
	var b strings.Builder
	for i := range g.columns {
		if i == idx {
			b.WriteString("*")
		} else {
			b.WriteString("|")
		}
		if i != len(g.columns)-1 {
			b.WriteString(" ")
		}
	}
	g.advance(idx, c.ParentHashes)
	return b.String()
}

func (g *graphBuilder) columnIndex(hash string) int {
	for i, h := range g.columns {
		if h == hash {
			return i
		}
	}
	return -1
}

func (g *graphBuilder) advance(idx int, parents []string) {
	if len(parents) == 0 {
		g.columns = append(g.columns[:idx], g.columns[idx+1:]...)
		return
	}
	primary := parents[0]
	g.columns[idx] = primary
	for i := 1; i < len(parents); i++ {
		parent := parents[i]
		g.removeColumn(parent)
		pos := idx + i
		if pos > len(g.columns) {
			pos = len(g.columns)
		}
		g.columns = append(g.columns[:pos], append([]string{parent}, g.columns[pos:]...)...)
	}
	g.trim()
}

func (g *graphBuilder) removeColumn(hash string) {
	for i, h := range g.columns {
		if h == hash {
			g.columns = append(g.columns[:i], g.columns[i+1:]...)
			return
		}
	}
}
