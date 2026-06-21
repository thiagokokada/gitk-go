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
	Commit      Commit
	Summary     string
	SearchText  string
	Graph       string
	ListMessage string
	ListAuthor  string
	ListDate    string
}

type FileSection struct {
	Path    string
	Line    int
	Added   int
	Removed int
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

func (s *Service) ScanCommits(skip, batch uint) ([]Entry, string, bool, error) {
	slog.Debug("ScanCommits start", slog.Uint64("skip", uint64(skip)), slog.Uint64("batch", uint64(batch)))
	startTotal := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		headHash string
		headName string
		headDur  time.Duration
	)
	if s.scan == nil || skip == 0 {
		startHead := time.Now()
		var (
			ok  bool
			err error
		)
		headHash, headName, ok, err = s.headStateLocked()
		headDur = time.Since(startHead)
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
	} else {
		headHash = s.scan.head
		headName = s.scan.headName
	}

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

func (s *Service) collectEntries(batch uint) ([]Entry, error) {
	entries := make([]Entry, 0, max(batch, DefaultBatch))
	for uint(len(entries)) < batch {
		commit, err := s.scan.next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if commit == nil {
			return nil, fmt.Errorf("backend returned nil commit")
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

func FormatCommitHeader(c Commit) string {
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

func newEntry(c *Commit) Entry {
	summary := formatSummary(c)
	listMsg, listAuthor, listDate := formatListColumns(c)
	var b strings.Builder
	b.WriteString(strings.ToLower(c.Hash))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Name))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Email))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Message))
	return Entry{
		Commit:      *c,
		Summary:     summary,
		SearchText:  b.String(),
		ListMessage: listMsg,
		ListAuthor:  listAuthor,
		ListDate:    listDate,
	}
}

func (e *Entry) ListColumns() (msg, author, when string) {
	if e == nil {
		return "", "", ""
	}
	if e.ListMessage == "" && e.ListAuthor == "" && e.ListDate == "" {
		return formatListColumns(&e.Commit)
	}
	return e.ListMessage, e.ListAuthor, e.ListDate
}

func formatListColumns(c *Commit) (msg, author, when string) {
	firstLine := firstCommitLine(c.Message)
	hash := c.Hash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	msg = hash + "  " + firstLine
	author = c.Author.Name + " <" + c.Author.Email + ">"
	when = c.Committer.When.Format("2006-01-02 15:04")
	return msg, author, when
}

func formatSummary(c *Commit) string {
	firstLine := firstCommitLine(c.Message)
	timestamp := c.Committer.When.Format("2006-01-02 15:04")
	hash := c.Hash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return hash + "  " + timestamp + "  " + firstLine
}

func firstCommitLine(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if idx := strings.IndexByte(message, '\n'); idx >= 0 {
		message = message[:idx]
	}
	if len(message) > 80 {
		message = message[:77] + "..."
	}
	return message
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
	return &graphBuilder{
		columns:    make([]string, 0, maxColumns),
		maxColumns: maxColumns,
	}
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
		g.insertAt(0, c.Hash)
		idx = 0
	}
	g.trim()
	cols := len(g.columns)
	var b strings.Builder
	if cols > 0 {
		b.Grow(cols*2 - 1)
	}
	for i := range cols {
		if i == idx {
			b.WriteString("*")
		} else {
			b.WriteString("|")
		}
		if i != cols-1 {
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
		g.removeAt(idx)
		return
	}
	primary := parents[0]
	g.columns[idx] = primary
	for i := 1; i < len(parents); i++ {
		parent := parents[i]
		g.removeColumn(parent)
		pos := min(idx+i, len(g.columns))
		g.insertAt(pos, parent)
	}
	g.trim()
}

func (g *graphBuilder) removeColumn(hash string) {
	for i, h := range g.columns {
		if h == hash {
			g.removeAt(i)
			return
		}
	}
}

func (g *graphBuilder) insertAt(pos int, hash string) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(g.columns) {
		pos = len(g.columns)
	}
	g.columns = append(g.columns, "")
	if pos < len(g.columns)-1 {
		copy(g.columns[pos+1:], g.columns[pos:len(g.columns)-1])
	}
	g.columns[pos] = hash
}

func (g *graphBuilder) removeAt(idx int) {
	if idx < 0 || idx >= len(g.columns) {
		return
	}
	copy(g.columns[idx:], g.columns[idx+1:])
	last := len(g.columns) - 1
	g.columns[last] = ""
	g.columns = g.columns[:last]
}
