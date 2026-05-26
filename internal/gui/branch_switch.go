package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/view"
	tk "modernc.org/tk9.0"
)

type branchChoice struct {
	name      string
	display   string
	isCurrent bool
}

func buildBranchChoices(branches []string, current string) []branchChoice {
	current = strings.TrimSpace(current)
	unique := make(map[string]struct{}, len(branches))
	var names []string
	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		if _, ok := unique[b]; ok {
			continue
		}
		unique[b] = struct{}{}
		names = append(names, b)
	}
	slices.Sort(names)

	choices := make([]branchChoice, 0, len(names))
	for _, name := range names {
		isCurrent := current != "" && name == current
		display := name
		if isCurrent {
			display = fmt.Sprintf("%s (current)", name)
		}
		choices = append(choices, branchChoice{name: name, display: display, isCurrent: isCurrent})
	}

	if current == "" {
		return choices
	}
	for i, c := range choices {
		if c.isCurrent {
			choices[0], choices[i] = choices[i], choices[0]
			break
		}
	}
	return choices
}

func filterBranchChoices(choices []branchChoice, query string) []branchChoice {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return choices
	}
	out := make([]branchChoice, 0, len(choices))
	for _, c := range choices {
		if strings.Contains(strings.ToLower(c.name), q) {
			out = append(out, c)
		}
	}
	return out
}

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
	all := buildBranchChoices(branches, current)
	a.ui.ShowBranchSwitchDialog(current, branchChoiceRows(all), view.BranchSwitchDialogHandlers{
		Filter: func(query string) []view.BranchSwitchRow {
			return branchChoiceRows(filterBranchChoices(all, query))
		},
		Submit: a.switchBranchAsync,
	})
}

func branchChoiceRows(choices []branchChoice) []view.BranchSwitchRow {
	rows := make([]view.BranchSwitchRow, 0, len(choices))
	for _, choice := range choices {
		rows = append(rows, view.BranchSwitchRow{
			Name:      choice.name,
			Display:   choice.display,
			IsCurrent: choice.isCurrent,
		})
	}
	return rows
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
