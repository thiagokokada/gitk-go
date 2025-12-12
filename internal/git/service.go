package git

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const DefaultBatch = 1000

type Service struct {
	repo repoState
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
	ref, err := s.repo.Head()
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("resolve HEAD: %w", err)
	}
	opts := &gitlib.LogOptions{From: ref.Hash(), Order: gitlib.LogOrderCommitterTime}
	iter, err := s.repo.Log(opts)
	if err != nil {
		return nil, "", false, fmt.Errorf("read commits: %w", err)
	}
	defer iter.Close()
	for range skip {
		if _, err := iter.Next(); err != nil {
			if err == io.EOF {
				return nil, refName(ref), false, nil
			}
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
	}
	var entries []*Entry
	for len(entries) < batch {
		commit, err := iter.Next()
		if err == io.EOF {
			return entries, refName(ref), false, nil
		}
		if err != nil {
			return nil, "", false, fmt.Errorf("iterate commits: %w", err)
		}
		entries = append(entries, newEntry(commit))
	}
	hasMore := false
	if _, err := iter.Next(); err == nil {
		hasMore = true
	} else if err != io.EOF {
		return nil, "", false, fmt.Errorf("iterate commits: %w", err)
	}
	if err := s.populateGraphStrings(entries, skip); err != nil {
		return nil, "", false, err
	}
	return entries, refName(ref), hasMore, nil
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

// populateGraphStrings requires the caller to hold s.repo.mu.
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
	if patch == nil {
		return header, nil, nil
	}
	var b strings.Builder
	lineNo := 0
	if header != "" {
		if !strings.HasSuffix(header, "\n") {
			header += "\n"
		}
		b.WriteString(header)
		lineNo = strings.Count(header, "\n")
	}
	filePatches := patch.FilePatches()
	if len(filePatches) == 0 {
		if header == "" {
			return "No changes.", nil, nil
		}
		b.WriteString("No changes.\n")
		return b.String(), nil, nil
	}
	var sections []FileSection
	for _, fp := range filePatches {
		path := filePatchPath(fp)
		fileHeader := fmt.Sprintf("diff --git a/%s b/%s\n", path, path)
		headerLine := lineNo + 1
		b.WriteString(fileHeader)
		lineNo += strings.Count(fileHeader, "\n")
		if fp.IsBinary() {
			b.WriteString("(binary files differ)\n")
			lineNo++
		} else {
			for _, chunk := range fp.Chunks() {
				if chunk == nil {
					continue
				}
				lines := strings.Split(chunk.Content(), "\n")
				for i, line := range lines {
					if i == len(lines)-1 && line == "" {
						continue
					}
					var prefix string
					switch chunk.Type() {
					case diff.Add:
						prefix = "+"
					case diff.Delete:
						prefix = "-"
					default:
						prefix = " "
					}
					b.WriteString(prefix + line + "\n")
					lineNo++
				}
			}
		}
		sections = append(sections, FileSection{Path: path, Line: headerLine})
	}
	return b.String(), sections, nil
}

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
