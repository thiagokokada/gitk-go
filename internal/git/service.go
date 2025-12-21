package git

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const DefaultBatch = 1000

const DefaultGraphMaxColumns = 200

type Service struct {
	// mu serializes access to repo operations that share iterators/state (scan session).
	mu sync.Mutex

	repo repoState
	scan *scanSession

	graphMaxColumns int
}

type repoState struct {
	path string
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
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	tmp := &Service{repo: repoState{path: abs}}
	root, err := tmp.runGitCommand([]string{"rev-parse", "--show-toplevel"}, false, "git rev-parse")
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("open repository: git rev-parse returned empty root")
	}
	return &Service{
		repo:            repoState{path: root},
		graphMaxColumns: DefaultGraphMaxColumns,
	}, nil
}

func (s *Service) RepoPath() string {
	return s.repo.path
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

func (s *Service) ScanCommits(skip, batch int) ([]*Entry, string, bool, error) {
	if batch <= 0 {
		batch = DefaultBatch
	}
	slog.Debug("ScanCommits start", slog.Int("skip", skip), slog.Int("batch", batch))
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
	if skip < 0 {
		skip = 0
	}
	// If the caller requests a different position than the current session, reset and advance to skip.
	if skip != s.scan.returned {
		startReset := time.Now()
		slog.Debug("ScanCommits reset session",
			slog.Int("requested_skip", skip),
			slog.Int("session_returned", s.scan.returned),
			slog.String("head", s.scan.headName),
		)
		if err := s.resetScanLocked(headHash, headName); err != nil {
			return nil, "", false, err
		}
		if err := s.scan.discard(skip); err != nil {
			if err == io.EOF {
				return nil, s.scan.headName, false, nil
			}
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
		slog.Debug("ScanCommits reset session done", slog.Duration("dur", time.Since(startReset)))
	}

	startIter := time.Now()
	entries := make([]*Entry, 0, batch)
	for len(entries) < batch {
		commit, err := s.scan.next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
		entries = append(entries, newEntry(commit))
	}
	iterDur := time.Since(startIter)

	startGraph := time.Now()
	graphTarget := skip + len(entries)
	if err := s.scan.ensureGraphProcessed(graphTarget); err != nil {
		return nil, "", false, err
	}
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
		slog.Int("session_returned", s.scan.returned),
		slog.Bool("has_more", hasMore),
		slog.String("head", s.scan.headName),
		slog.Int("graph_target", graphTarget),
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

func (s *Service) headStateLocked() (hash string, headName string, ok bool, err error) {
	if s.repo.path == "" {
		return "", "", false, fmt.Errorf("repository root not set")
	}
	out, err := s.runGitCommand([]string{"rev-parse", "-q", "--verify", "HEAD"}, true, "git rev-parse")
	if err != nil {
		return "", "", false, err
	}
	hash = strings.TrimSpace(out)
	if hash == "" {
		return "", "", false, nil
	}
	ref, err := s.runGitCommand([]string{"symbolic-ref", "-q", "--short", "HEAD"}, true, "git symbolic-ref")
	if err != nil {
		return "", "", false, err
	}
	headName = strings.TrimSpace(ref)
	if headName == "" {
		headName = "HEAD"
	}
	return hash, headName, true, nil
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

type LocalChanges struct {
	HasWorktree bool
	HasStaged   bool

	UnstagedDiff string
	StagedDiff   string

	UnstagedSections []FileSection
	StagedSections   []FileSection
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
	if g == nil || g.maxColumns <= 0 {
		return
	}
	if len(g.columns) > g.maxColumns {
		g.columns = g.columns[:g.maxColumns]
	}
}

func (g *graphBuilder) Line(c *Commit) string {
	if c == nil {
		return ""
	}
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
	if g == nil {
		return
	}
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
