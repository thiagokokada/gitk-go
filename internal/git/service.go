package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	gitindex "github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pmezard/go-difflib/difflib"
)

const DefaultBatch = 1000

type Service struct {
	repoPath string
	repo     *gitlib.Repository
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
	return &Service{repoPath: abs, repo: repo}, nil
}

func (s *Service) RepoPath() string {
	return s.repoPath
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
	opts := &gitlib.LogOptions{From: ref.Hash(), Order: gitlib.LogOrderDFS}
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
	if s.repo == nil {
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

func (s *Service) Diff(commit *object.Commit) (string, []FileSection, error) {
	currentTree, err := commit.Tree()
	if err != nil {
		return "", nil, err
	}
	var parentTree *object.Tree
	if commit.NumParents() > 0 {
		parent, err := commit.Parent(0)
		if err != nil {
			return "", nil, err
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return "", nil, err
		}
	}
	changes, err := object.DiffTree(parentTree, currentTree)
	if err != nil {
		return "", nil, err
	}
	if len(changes) == 0 {
		header := FormatCommitHeader(commit)
		return header + "\nNo file level changes.", nil, nil
	}
	patch, err := changes.Patch()
	if err != nil {
		return "", nil, err
	}
	header := FormatCommitHeader(commit)
	return renderPatch(header, patch)
}

func FormatCommitHeader(c *object.Commit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "commit %s\n", c.Hash)
	fmt.Fprintf(&b, "Author: %s <%s>\n", c.Author.Name, c.Author.Email)
	fmt.Fprintf(&b, "Date:   %s\n\n", c.Author.When.Format(time.RFC1123))
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

type localChange struct {
	path string
	from *object.File
	to   *object.File
}

func (s *Service) LocalChanges() (LocalChanges, error) {
	var res LocalChanges
	if s.repo == nil {
		return res, nil
	}
	wt, err := s.repo.Worktree()
	if err != nil {
		return res, err
	}
	status, err := wt.Status()
	if err != nil {
		return res, err
	}
	for _, st := range status {
		if st.Worktree != gitlib.Unmodified && st.Worktree != gitlib.Untracked {
			res.HasWorktree = true
		}
		if st.Staging != gitlib.Unmodified {
			res.HasStaged = true
		}
		if res.HasWorktree && res.HasStaged {
			break
		}
	}
	return res, nil
}

func (s *Service) WorktreeDiff(staged bool) (string, []FileSection, error) {
	if s.repo == nil {
		return "", nil, fmt.Errorf("repository not initialized")
	}
	wt, err := s.repo.Worktree()
	if err != nil {
		return "", nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return "", nil, err
	}
	headTree, err := s.headTree()
	if err != nil && err != plumbing.ErrReferenceNotFound {
		return "", nil, err
	}
	var idx *gitindex.Index
	if staged {
		idx, err = s.repo.Storer.Index()
		if err != nil {
			return "", nil, err
		}
	}
	var paths []string
	for path, st := range status {
		include := false
		if staged {
			include = st.Staging != gitlib.Unmodified
		} else {
			include = st.Worktree != gitlib.Unmodified && st.Worktree != gitlib.Untracked
		}
		if include {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	var diffs []localChange
	for _, path := range paths {
		fromFile, err := fileFromTree(headTree, path)
		if err != nil {
			return "", nil, err
		}
		var toFile *object.File
		if staged {
			toFile, err = fileFromIndex(idx, s.repo, path)
		} else {
			toFile, err = fileFromDisk(s.repoPath, path)
		}
		if err != nil {
			return "", nil, err
		}
		if fromFile == nil && toFile == nil {
			continue
		}
		diffs = append(diffs, localChange{path: path, from: fromFile, to: toFile})
	}
	header := localDiffHeader(staged) + "\n"
	if len(diffs) == 0 {
		return "", nil, nil
	}
	return renderLocalDiff(header, diffs)
}

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
	timestamp := c.Author.When.Format("2006-01-02 15:04")
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

func (s *Service) headTree() (*object.Tree, error) {
	if s.repo == nil {
		return nil, nil
	}
	ref, err := s.repo.Head()
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return nil, nil
		}
		return nil, err
	}
	commit, err := s.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}
	return commit.Tree()
}

func fileFromTree(tree *object.Tree, path string) (*object.File, error) {
	if tree == nil {
		return nil, nil
	}
	f, err := tree.File(path)
	if err == object.ErrFileNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func fileFromIndex(idx *gitindex.Index, repo *gitlib.Repository, path string) (*object.File, error) {
	if idx == nil || repo == nil {
		return nil, nil
	}
	entry, err := idx.Entry(path)
	if err == gitindex.ErrEntryNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	blob, err := object.GetBlob(repo.Storer, entry.Hash)
	if err != nil {
		return nil, err
	}
	return object.NewFile(entry.Name, entry.Mode, blob), nil
}

func fileFromDisk(root, path string) (*object.File, error) {
	if root == "" {
		return nil, fmt.Errorf("repository root not set")
	}
	fullPath := filepath.Join(root, path)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	mem := &plumbing.MemoryObject{}
	mem.SetType(plumbing.BlobObject)
	if _, err := mem.Write(data); err != nil {
		return nil, err
	}
	blob, err := object.DecodeBlob(mem)
	if err != nil {
		return nil, err
	}
	mode := filemode.Regular
	if info, err := file.Stat(); err == nil {
		if m, err := filemode.NewFromOSFileMode(info.Mode()); err == nil {
			mode = m
		}
	}
	return object.NewFile(path, mode, blob), nil
}

func renderLocalDiff(header string, diffs []localChange) (string, []FileSection, error) {
	var b strings.Builder
	lineNo := 0
	if header != "" {
		if !strings.HasSuffix(header, "\n") {
			header += "\n"
		}
		b.WriteString(header)
		lineNo = strings.Count(header, "\n")
	}
	var sections []FileSection
	for _, diffItem := range diffs {
		if diffItem.path == "" {
			continue
		}
		fileHeader := fmt.Sprintf("diff --git a/%s b/%s\n", diffItem.path, diffItem.path)
		sections = append(sections, FileSection{Path: diffItem.path, Line: lineNo + 1})
		b.WriteString(fileHeader)
		lineNo += strings.Count(fileHeader, "\n")

		isBinary, err := binaryChange(diffItem)
		if err != nil {
			return "", nil, err
		}
		if isBinary {
			b.WriteString("(binary files differ)\n")
			lineNo++
			continue
		}

		fromLines, err := fileLines(diffItem.from)
		if err != nil {
			return "", nil, err
		}
		toLines, err := fileLines(diffItem.to)
		if err != nil {
			return "", nil, err
		}

		ud := difflib.UnifiedDiff{
			A:        fromLines,
			B:        toLines,
			FromFile: fmt.Sprintf("a/%s", diffItem.path),
			ToFile:   fmt.Sprintf("b/%s", diffItem.path),
			Context:  3,
		}
		diffText, err := difflib.GetUnifiedDiffString(ud)
		if err != nil {
			return "", nil, err
		}
		if diffText == "" {
			b.WriteString("(no textual changes)\n")
			lineNo++
			continue
		}
		b.WriteString(diffText)
		lineNo += strings.Count(diffText, "\n")
		if !strings.HasSuffix(diffText, "\n") {
			b.WriteString("\n")
			lineNo++
		}
	}
	return b.String(), sections, nil
}

func binaryChange(ch localChange) (bool, error) {
	if ch.from != nil {
		bin, err := ch.from.IsBinary()
		if err != nil {
			return false, err
		}
		if bin {
			return true, nil
		}
	}
	if ch.to != nil {
		bin, err := ch.to.IsBinary()
		if err != nil {
			return false, err
		}
		if bin {
			return true, nil
		}
	}
	return false, nil
}

func fileLines(f *object.File) ([]string, error) {
	if f == nil {
		return []string{}, nil
	}
	content, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return difflib.SplitLines(content), nil
}
