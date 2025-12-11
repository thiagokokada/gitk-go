//go:build !gitcli

package git

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitlib "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	gitindex "github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pmezard/go-difflib/difflib"
)

type localChange struct {
	path string
	from *object.File
	to   *object.File
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
