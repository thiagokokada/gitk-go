package selection

import (
	"sync/atomic"

	"github.com/thiagokokada/gitk-go/internal/git"
)

type selectionKind int

const (
	selectionNone selectionKind = iota
	selectionCommit
	selectionLocalUnstaged
	selectionLocalStaged
)

type selectionSnapshot struct {
	kind selectionKind
	hash string
	idx  int
}

type State struct {
	snapshot atomic.Pointer[selectionSnapshot]
}

func (s *State) snapshotValue() selectionSnapshot {
	if snap := s.snapshot.Load(); snap != nil {
		return *snap
	}
	return selectionSnapshot{}
}

func (s *State) storeSnapshot(snapshot selectionSnapshot) {
	s.snapshot.Store(&snapshot)
}

func (s *State) Clear() {
	s.snapshot.Store(nil)
}

func (s *State) SetCommit(entry *git.Entry, idx int) bool {
	if entry == nil || entry.Commit == nil || idx < 0 {
		s.Clear()
		return false
	}
	s.storeSnapshot(selectionSnapshot{
		kind: selectionCommit,
		hash: entry.Commit.Hash,
		idx:  idx,
	})
	return true
}

func (s *State) SetLocal(staged bool) {
	kind := selectionLocalUnstaged
	if staged {
		kind = selectionLocalStaged
	}
	s.storeSnapshot(selectionSnapshot{kind: kind})
}

func (s *State) CommitHash() string {
	snap := s.snapshotValue()
	if snap.kind != selectionCommit {
		return ""
	}
	return snap.hash
}

func (s *State) CommitIndex(visible []*git.Entry) int {
	snap := s.snapshotValue()
	if snap.kind != selectionCommit {
		return -1
	}
	if snap.idx >= 0 && snap.idx < len(visible) {
		entry := visible[snap.idx]
		if entry != nil && entry.Commit != nil && entry.Commit.Hash == snap.hash {
			return snap.idx
		}
	}
	if snap.hash == "" {
		return -1
	}
	for idx, entry := range visible {
		if entry == nil || entry.Commit == nil {
			continue
		}
		if entry.Commit.Hash == snap.hash {
			return idx
		}
	}
	return -1
}
