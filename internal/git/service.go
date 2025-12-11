package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
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
		if !ref.Name().IsBranch() {
			return nil
		}
		hash := ref.Hash().String()
		labels[hash] = append(labels[hash], ref.Name().Short())
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
	var sections []FileSection
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	lineNo := strings.Count(header+"\n", "\n")
	for _, fp := range patch.FilePatches() {
		path := filePatchPath(fp)
		fileHeader := fmt.Sprintf("diff --git a/%s b/%s\n", path, path)
		headerLine := lineNo + 1
		b.WriteString(fileHeader)
		lineNo += strings.Count(fileHeader, "\n")
		if fp.IsBinary() {
			binaryInfo := "(binary files differ)\n"
			b.WriteString(binaryInfo)
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

func (s *Service) populateGraphStrings(entries []*Entry, skip int) error {
	if len(entries) == 0 {
		return nil
	}
	total := skip + len(entries)
	if total <= 0 {
		return nil
	}
	args := []string{
		"-C", s.repoPath,
		"log",
		"--graph",
		"--no-color",
		"--topo-order",
		fmt.Sprintf("--max-count=%d", total),
		"--format=%H",
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git log --graph: %s", msg)
	}
	rows, err := parseGraphRows(stdout.Bytes())
	if err != nil {
		return err
	}
	if len(rows) < total {
		return fmt.Errorf("git graph returned %d rows, need %d", len(rows), total)
	}
	rows = rows[skip:]
	if len(rows) > len(entries) {
		rows = rows[:len(entries)]
	}
	graphByHash := make(map[string]string, len(rows))
	for _, row := range rows {
		graphByHash[row.hash] = row.graph
	}
	for _, entry := range entries {
		entry.Graph = graphByHash[entry.Commit.Hash.String()]
	}
	return nil
}

type graphRow struct {
	hash  string
	graph string
}

func parseGraphRows(data []byte) ([]graphRow, error) {
	var rows []graphRow
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if row, ok := parseGraphLine(line); ok {
			rows = append(rows, row)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func parseGraphLine(line string) (graphRow, bool) {
	trimmed := strings.TrimRight(line, " ")
	if len(trimmed) < 40 {
		return graphRow{}, false
	}
	hash := trimmed[len(trimmed)-40:]
	if !isHexHash(hash) {
		return graphRow{}, false
	}
	graph := strings.TrimRight(trimmed[:len(trimmed)-40], " ")
	return graphRow{hash: hash, graph: graph}, true
}

func isHexHash(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
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

func refName(ref *plumbing.Reference) string {
	name := ref.Name().Short()
	if name == "" {
		name = ref.Name().String()
	}
	return name
}
