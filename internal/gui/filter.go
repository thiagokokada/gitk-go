package gui

import (
	"log/slog"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

func (a *Controller) applyFilter(raw string) {
	if a.ui.filterEntry.Textvariable() != raw {
		return
	}
	a.applyFilterContent(raw)
}

func (a *Controller) applyFilterState(raw string) {
	a.model.applyFilter(raw)
}

func (a *Controller) applyFilterImmediate(raw string) {
	a.runtime.actions.filterDebounce.Stop()
	a.applyFilter(raw)
}

func (a *Controller) applyFilterContent(raw string) {
	a.applyFilterState(raw)
	if a.ui.treeView == nil || !tkutil.WidgetExists(a.ui.treeView.String()) {
		return
	}
	autoLoad := shouldAutoLoadForFilter(
		a.model.state.filter.value,
		len(a.model.data.visible),
		a.model.state.tree.hasMore,
		a.model.state.tree.loadingBatch,
	)
	a.storeScrollState()
	if autoLoad {
		a.loadMoreCommitsAsync(false)
	}
	a.syncTreeRows()

	if staged, ok := a.model.state.selection.LocalSelection(); ok {
		id := localRowID(staged)
		if a.model.state.tree.rows.hasSpecialItem(id) {
			a.ui.treeView.Selection("set", id)
			a.ui.treeView.Focus(id)
			a.ui.treeView.See(id)
			a.setStatus(a.statusSummary())
			a.scheduleAutoLoadCheck()
			a.restoreScrollState()
			a.scheduleGraphCanvasDraw()
			return
		}
	}

	if len(a.model.data.visible) == 0 {
		if len(a.model.data.commits) == 0 {
			a.clearDetailText("Repository has no commits yet.")
		} else {
			a.clearDetailText("No commits match the current filter.")
		}
		a.setStatus(a.statusSummary())
		return
	}

	index := a.visibleSelectionIndex()
	if index < 0 && len(a.model.data.visible) > 0 {
		index = 0
	}
	if index >= 0 {
		if entry, ok := a.commitEntryAt(index); ok {
			id := commitRowID(entry)
			if id != "" {
				a.ui.treeView.Selection("set", id)
				a.ui.treeView.Focus(id)
				a.ui.treeView.See(id)
			}
			if entry.Commit != nil && entry.Commit.Hash != a.model.state.selection.CommitHash() {
				a.showCommitDetails(entry, index)
			}
		}
	}
	a.setStatus(a.statusSummary())
	a.scheduleAutoLoadCheck()
	a.restoreScrollState()
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) storeScrollState() {
	a.model.state.scroll.total = a.treeChildCount()
	if a.model.state.scroll.total > 0 {
		if start, _, err := a.treeYviewRange(); err == nil {
			a.model.state.scroll.start = start
		}
	}
}

func (a *Controller) restoreScrollState() {
	newTotal := a.treeChildCount()
	target, ok := a.model.state.scroll.restoreTarget(newTotal)
	if !ok {
		return
	}
	tkutil.MustEvalf("%s yview moveto %f", a.ui.treeView, target)
}

func (a *Controller) treeChildCount() int {
	path := a.ui.treeView.String()
	if path == "" {
		return 0
	}
	return len(a.ui.treeView.Children(""))
}

func (a *Controller) visibleSelectionIndex() int {
	return a.model.state.selection.CommitIndex(a.model.data.visible)
}

func (a *Controller) scheduleFilterApply(raw string) {
	if raw == "" {
		a.applyFilterImmediate("")
		return
	}
	slog.Debug("scheduleFilterApply", slog.String("value", raw))
	a.runtime.actions.filterDebounce.Trigger(raw)
}

func (a *Controller) scheduleFilterApplyState(raw string) {
	if raw == "" {
		a.runtime.actions.filterDebounce.Stop()
		a.applyFilterState("")
		return
	}
	a.runtime.actions.filterDebounce.SetPending(raw)
}

func shouldAutoLoadForFilter(filterValue string, visibleLen int, hasMore bool, loadingBatch bool) bool {
	if loadingBatch || !hasMore || visibleLen > 0 {
		return false
	}
	return strings.TrimSpace(filterValue) != ""
}

func (s scrollState) restoreTarget(newTotal int) (float64, bool) {
	if s.start < 0 || s.total <= 0 || newTotal <= 0 {
		return 0, false
	}
	target := s.start * float64(s.total) / float64(newTotal)
	target = max(0.0, min(target, 1.0))
	return target, true
}
