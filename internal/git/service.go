package git

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const DefaultBatch = 1000

type Service struct {
	// mu serializes access to repo operations that share iterators/state (scan session).
	mu sync.Mutex

	repo repoState
	scan *scanSession
}

type repoState struct {
	*gitlib.Repository
	path string
}

type Entry struct {
	Commit     *object.Commit
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
	repo, err := gitlib.PlainOpenWithOptions(abs, &gitlib.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	return &Service{repo: repoState{path: abs, Repository: repo}}, nil
}

func (s *Service) RepoPath() string {
	return s.repo.path
}

func (s *Service) ScanCommits(skip, batch int) ([]*Entry, string, bool, error) {
	if batch <= 0 {
		batch = DefaultBatch
	}
	slog.Debug("ScanCommits start", slog.Int("skip", skip), slog.Int("batch", batch))
	s.mu.Lock()
	defer s.mu.Unlock()

	ref, err := s.repo.Head()
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			if s.scan != nil {
				s.scan.close()
				s.scan = nil
			}
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("resolve HEAD: %w", err)
	}
	if err := s.ensureScanSessionLocked(ref); err != nil {
		return nil, "", false, err
	}
	if skip < 0 {
		skip = 0
	}
	// If the caller requests a different position than the current session, reset and advance to skip.
	if skip != s.scan.returned {
		slog.Debug("ScanCommits reset session",
			slog.Int("requested_skip", skip),
			slog.Int("session_returned", s.scan.returned),
			slog.String("head", s.scan.headName),
		)
		if err := s.resetScanLocked(ref); err != nil {
			return nil, "", false, err
		}
		if err := s.scan.discard(skip); err != nil {
			if err == io.EOF {
				return nil, s.scan.headName, false, nil
			}
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
	}

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
	if err := s.populateGraphStrings(entries, skip); err != nil {
		return nil, "", false, err
	}
	hasMore, err := s.scan.hasMore()
	if err != nil {
		return nil, "", false, err
	}
	slog.Debug("ScanCommits done",
		slog.Int("returned", len(entries)),
		slog.Int("session_returned", s.scan.returned),
		slog.Bool("has_more", hasMore),
		slog.String("head", s.scan.headName),
	)
	return entries, s.scan.headName, hasMore, nil
}

type scanSession struct {
	head     plumbing.Hash
	headName string

	displayIter object.CommitIter

	// buffered holds the next commit returned by hasMore() so ScanCommits can keep consuming in-order.
	buffered  *object.Commit
	exhausted bool
	returned  int
}

func (s *Service) ensureScanSessionLocked(ref *plumbing.Reference) error {
	if s.scan != nil && s.scan.head == ref.Hash() {
		return nil
	}
	return s.resetScanLocked(ref)
}

func (s *Service) resetScanLocked(ref *plumbing.Reference) error {
	if s.scan != nil {
		s.scan.close()
		s.scan = nil
	}
	displayIter, err := s.repo.Log(&gitlib.LogOptions{From: ref.Hash(), Order: gitlib.LogOrderCommitterTime})
	if err != nil {
		return fmt.Errorf("read commits: %w", err)
	}
	s.scan = &scanSession{
		head:        ref.Hash(),
		headName:    refName(ref),
		displayIter: displayIter,
	}
	slog.Debug("ScanCommits session initialized", slog.String("head", s.scan.headName))
	return nil
}

func (s *scanSession) close() {
	if s == nil {
		return
	}
	if s.displayIter != nil {
		s.displayIter.Close()
	}
	s.displayIter = nil
	s.buffered = nil
	s.exhausted = true
}

func (s *scanSession) hasMore() (bool, error) {
	if s.exhausted {
		return false, nil
	}
	if s.buffered != nil {
		return true, nil
	}
	// Read-ahead a single commit into buffered so hasMore doesn't consume an extra commit.
	commit, err := s.displayIter.Next()
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

func (s *scanSession) next() (*object.Commit, error) {
	if s.exhausted {
		return nil, io.EOF
	}
	if s.buffered != nil {
		commit := s.buffered
		s.buffered = nil
		s.returned++
		return commit, nil
	}
	commit, err := s.displayIter.Next()
	if err != nil {
		if err == io.EOF {
			s.exhausted = true
		}
		return nil, err
	}
	s.returned++
	return commit, nil
}

func (s *scanSession) discard(count int) error {
	// Consume and drop commits to align the session position with the requested skip.
	for range count {
		if _, err := s.next(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) BranchLabels() (map[string][]string, error) {
	labels := map[string][]string{}
	if s.repo.Repository == nil {
		return labels, nil
	}
	refs, err := s.repo.References()
	if err != nil {
		return nil, err
	}
	defer refs.Close()
	headRef, err := s.repo.Head()
	var headHash plumbing.Hash
	var headBranch string
	if err == nil && headRef != nil {
		headHash = headRef.Hash()
		if headRef.Name().IsBranch() {
			headBranch = headRef.Name().Short()
		}
	}
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		name := ref.Name()
		isBranch := name.IsBranch()
		isRemote := name.IsRemote()
		if !isBranch && !isRemote {
			return nil
		}
		short := name.Short()
		if isRemote && strings.HasSuffix(short, "/HEAD") {
			return nil
		}
		hash := ref.Hash().String()
		labels[hash] = append(labels[hash], short)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if headHash != plumbing.ZeroHash {
		key := headHash.String()
		label := "HEAD"
		if headBranch != "" {
			label = fmt.Sprintf("HEAD -> %s", headBranch)
		}
		labels[key] = append([]string{label}, labels[key]...)
	}
	return labels, nil
}

func FormatCommitHeader(c *object.Commit) string {
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

// populateGraphStrings expects the caller to serialize with Service.mu.
func (s *Service) populateGraphStrings(entries []*Entry, skip int) error {
	if len(entries) == 0 {
		return nil
	}
	total := skip + len(entries)
	if total <= 0 {
		return nil
	}
	ref, err := s.repo.Head()
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return nil
		}
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	iter, err := s.repo.Log(&gitlib.LogOptions{From: ref.Hash(), Order: gitlib.LogOrderDFS})
	if err != nil {
		return fmt.Errorf("read commits for graph: %w", err)
	}
	defer iter.Close()

	builder := newGraphBuilder()
	graphByHash := make(map[string]string, len(entries))
	processed := 0
	for processed < total {
		commit, err := iter.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("iterate commits for graph: %w", err)
		}
		line := builder.Line(commit)
		processed++
		if processed <= skip {
			continue
		}
		graphByHash[commit.Hash.String()] = line
		if len(graphByHash) == len(entries) {
			break
		}
	}
	for _, entry := range entries {
		entry.Graph = graphByHash[entry.Commit.Hash.String()]
	}
	return nil
}

func appendSignatureLine(b *strings.Builder, label string, sig object.Signature) {
	fmt.Fprintf(b, "%s: %s <%s>", label, sig.Name, sig.Email)
	if !sig.When.IsZero() {
		fmt.Fprintf(b, "  %s", sig.When.Format("2006-01-02 15:04:05 -0700"))
	}
	b.WriteByte('\n')
}

func newEntry(c *object.Commit) *Entry {
	summary := formatSummary(c)
	var b strings.Builder
	b.WriteString(strings.ToLower(c.Hash.String()))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Name))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Author.Email))
	b.WriteByte(' ')
	b.WriteString(strings.ToLower(c.Message))
	return &Entry{Commit: c, Summary: summary, SearchText: b.String()}
}

func formatSummary(c *object.Commit) string {
	firstLine := strings.SplitN(strings.TrimSpace(c.Message), "\n", 2)[0]
	if len(firstLine) > 80 {
		firstLine = firstLine[:77] + "..."
	}
	timestamp := c.Committer.When.Format("2006-01-02 15:04")
	return fmt.Sprintf("%s  %s  %s", c.Hash.String()[:7], timestamp, firstLine)
}

func filePatchPath(fp diff.FilePatch) string {
	from, to := fp.Files()
	if to != nil && to.Path() != "" {
		return to.Path()
	}
	if from != nil && from.Path() != "" {
		return from.Path()
	}
	return "(unknown)"
}

func localDiffHeader(staged bool) string {
	if staged {
		return "Local changes checked into index but not committed"
	}
	return "Local uncommitted changes, not checked in to index"
}

func renderPatch(header string, patch interface{ FilePatches() []diff.FilePatch }) (string, []FileSection, error) {
	var b strings.Builder
	lineOffset := 0
	if header != "" {
		if !strings.HasSuffix(header, "\n") {
			header += "\n"
		}
		b.WriteString(header)
		lineOffset = strings.Count(header, "\n")
	}
	if patch == nil {
		if b.Len() == 0 {
			return "No changes.", nil, nil
		}
		return b.String(), nil, nil
	}
	filePatches := patch.FilePatches()
	if len(filePatches) == 0 {
		if b.Len() == 0 {
			return "No changes.", nil, nil
		}
		b.WriteString("No changes.\n")
		return b.String(), nil, nil
	}
	body, err := encodeUnifiedPatch(filePatches)
	if err != nil {
		return "", nil, err
	}
	b.WriteString(body)
	sections := parseGitDiffSections(body, lineOffset)
	return b.String(), sections, nil
}

func encodeUnifiedPatch(filePatches []diff.FilePatch) (string, error) {
	var buf bytes.Buffer
	enc := diff.NewUnifiedEncoder(&buf, diff.DefaultContextLines)
	if err := enc.Encode(filePatchSet{patches: filePatches}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type filePatchSet struct {
	patches []diff.FilePatch
}

func (f filePatchSet) FilePatches() []diff.FilePatch { return f.patches }
func (filePatchSet) Message() string                 { return "" }

type graphBuilder struct {
	columns []plumbing.Hash
}

func newGraphBuilder() *graphBuilder {
	return &graphBuilder{}
}

func (g *graphBuilder) Line(c *object.Commit) string {
	if c == nil {
		return ""
	}
	idx := g.columnIndex(c.Hash)
	if idx == -1 {
		g.columns = append([]plumbing.Hash{c.Hash}, g.columns...)
		idx = 0
	}
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

func (g *graphBuilder) columnIndex(hash plumbing.Hash) int {
	for i, h := range g.columns {
		if h == hash {
			return i
		}
	}
	return -1
}

func (g *graphBuilder) advance(idx int, parents []plumbing.Hash) {
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
		g.columns = append(g.columns[:pos], append([]plumbing.Hash{parent}, g.columns[pos:]...)...)
	}
}

func (g *graphBuilder) removeColumn(hash plumbing.Hash) {
	for i, h := range g.columns {
		if h == hash {
			g.columns = append(g.columns[:i], g.columns[i+1:]...)
			return
		}
	}
}

func refName(ref *plumbing.Reference) string {
	name := ref.Name().Short()
	if name == "" {
		name = ref.Name().String()
	}
	return name
}
