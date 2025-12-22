package git

import (
	"errors"

	gitbackend "github.com/thiagokokada/gitk-go/internal/git/backend"
)

type fakeBackend struct {
	repoPath string

	headStateFunc          func() (hash string, headName string, ok bool, err error)
	listRefsFunc           func() ([]gitbackend.Ref, error)
	switchBranchFunc       func(branch string) error
	commitDiffTextFunc     func(commitHash string, parentHash string) (string, error)
	worktreeDiffTextFunc   func(staged bool) (string, error)
	localChangesStatusFunc func() (gitbackend.LocalChanges, error)
	startLogStreamFunc     func(fromHash string) (gitbackend.LogStream, error)

	lastCommitHash   string
	lastParentHash   string
	lastStagedParam  *bool
	lastSwitchBranch string
}

func (f *fakeBackend) RepoPath() string { return f.repoPath }

func (f *fakeBackend) StartLogStream(fromHash string) (gitbackend.LogStream, error) {
	if f.startLogStreamFunc != nil {
		return f.startLogStreamFunc(fromHash)
	}
	return nil, errors.New("unexpected StartLogStream call")
}

func (f *fakeBackend) HeadState() (hash string, headName string, ok bool, err error) {
	if f.headStateFunc != nil {
		return f.headStateFunc()
	}
	return "", "", false, errors.New("unexpected HeadState call")
}

func (f *fakeBackend) ListRefs() ([]gitbackend.Ref, error) {
	if f.listRefsFunc != nil {
		return f.listRefsFunc()
	}
	return nil, errors.New("unexpected ListRefs call")
}

func (f *fakeBackend) SwitchBranch(branch string) error {
	f.lastSwitchBranch = branch
	if f.switchBranchFunc != nil {
		return f.switchBranchFunc(branch)
	}
	return errors.New("unexpected SwitchBranch call")
}

func (f *fakeBackend) CommitDiffText(commitHash string, parentHash string) (string, error) {
	f.lastCommitHash = commitHash
	f.lastParentHash = parentHash
	if f.commitDiffTextFunc != nil {
		return f.commitDiffTextFunc(commitHash, parentHash)
	}
	return "", errors.New("unexpected CommitDiffText call")
}

func (f *fakeBackend) WorktreeDiffText(staged bool) (string, error) {
	f.lastStagedParam = &staged
	if f.worktreeDiffTextFunc != nil {
		return f.worktreeDiffTextFunc(staged)
	}
	return "", errors.New("unexpected WorktreeDiffText call")
}

func (f *fakeBackend) LocalChangesStatus() (gitbackend.LocalChanges, error) {
	if f.localChangesStatusFunc != nil {
		return f.localChangesStatusFunc()
	}
	return gitbackend.LocalChanges{}, errors.New("unexpected LocalChangesStatus call")
}
