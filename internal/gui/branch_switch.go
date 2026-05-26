package gui

import (
	"fmt"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/view"
	tk "modernc.org/tk9.0"
)

func (a *Controller) promptBranchSwitch() {
	if a.svc == nil || a.svc.RepoPath() == "" {
		view.ShowMessage("Switch Branch", "error", "No repository is currently open.")
		return
	}
	branches, head, err := a.svc.LocalBranchNames()
	if err != nil {
		view.ShowMessage("Switch Branch", "error", fmt.Sprintf("Unable to list branches:\n\n%v", err))
		return
	}
	if len(branches) == 0 {
		view.ShowMessage("Switch Branch", "info", "This repository has no local branches.")
		return
	}
	a.showBranchSwitchDialog(branches, head)
}

func (a *Controller) showBranchSwitchDialog(branches []string, current string) {
	all := view.BuildBranchSwitchRows(branches, current)
	a.ui.ShowBranchSwitchDialog(current, all, view.BranchSwitchDialogHandlers{
		Filter: func(query string) []view.BranchSwitchRow {
			return view.FilterBranchSwitchRows(all, query)
		},
		Submit: a.switchBranchAsync,
	})
}

func (a *Controller) switchBranchAsync(branch string) {
	if a.svc == nil {
		return
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return
	}
	a.setStatus(fmt.Sprintf("Switching to %s...", branch))
	go func() {
		err := a.svc.SwitchBranch(branch)
		tk.PostEvent(func() {
			if err != nil {
				view.ShowMessage("Switch Branch", "error", fmt.Sprintf("Unable to switch branches:\n\n%v", err))
				a.setStatus(fmt.Sprintf("Failed to switch branches: %v", err))
				return
			}

			a.cancelPendingDiffLoad()
			a.clearTreeRows()
			a.model.ResetBranch()

			a.setFileSections(nil)
			a.setLocalRowVisibility(false, false)
			a.setLocalRowVisibility(true, false)
			a.clearDetailText("Select a commit to view its details.")
			a.showInitialLoadingRow()
			a.setStatus("Loading commits...")
			a.refreshLocalChangesAsync(true)
			a.reloadCommitsAsync()
		}, false)
	}()
}
