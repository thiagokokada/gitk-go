package gui

import "github.com/thiagokokada/gitk-go/internal/git"

type localChangeActions struct {
	showUnstaged  bool
	showStaged    bool
	resetUnstaged bool
	resetStaged   bool
	loadUnstaged  bool
	loadStaged    bool
}

func (t treeState) localChangePlan(repoReady bool, prefetch bool, status git.LocalChanges) localChangeActions {
	if !repoReady {
		return localChangeActions{
			showUnstaged:  false,
			showStaged:    false,
			resetUnstaged: true,
			resetStaged:   true,
		}
	}
	prevUnstaged := t.showLocalUnstaged
	prevStaged := t.showLocalStaged
	actions := localChangeActions{
		showUnstaged:  status.HasWorktree,
		showStaged:    status.HasStaged,
		resetUnstaged: !status.HasWorktree,
		resetStaged:   !status.HasStaged,
	}
	if prefetch {
		actions.loadUnstaged = status.HasWorktree
		actions.loadStaged = status.HasStaged
		return actions
	}
	actions.loadUnstaged = status.HasWorktree && !prevUnstaged
	actions.loadStaged = status.HasStaged && !prevStaged
	return actions
}
