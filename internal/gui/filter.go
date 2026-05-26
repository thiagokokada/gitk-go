package gui

import (
	"log/slog"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

func (a *Controller) applyFilter(raw string) {
	if a.ui.FilterEntry.Textvariable() != raw {
		return
	}
	a.applyFilterContent(raw)
}

func (a *Controller) applyFilterState(raw string) {
	a.model.ApplyFilter(raw)
}

func (a *Controller) applyFilterImmediate(raw string) {
	a.runtime.actions.filterDebounce.Stop()
	a.applyFilter(raw)
}

func (a *Controller) applyFilterContent(raw string) {
	a.applyFilterState(raw)
	if a.ui.TreeView == nil || !tkutil.WidgetExists(a.ui.TreeView.String()) {
		return
	}
	autoLoad := shouldAutoLoadForFilter(
		a.model.State.Filter.Value,
		len(a.model.Data.Visible),
		a.model.State.Tree.HasMore,
		a.model.State.Tree.LoadingBatch,
	)
	a.storeScrollState()
	if autoLoad {
		a.loadMoreCommitsAsync(false)
	}
	a.syncTreeRows()

	plan := a.model.FilterSelectionPlan()
	if a.applyFilterSelectionPlan(plan) {
		return
	}

	a.setStatus(a.statusSummary())
	a.scheduleAutoLoadCheck()
	a.restoreScrollState()
	a.scheduleGraphCanvasDraw()
}

func (a *Controller) applyFilterSelectionPlan(plan model.SelectionDisplayPlan) bool {
	switch plan.Kind {
	case model.SelectionDisplayLocal:
		a.focusTreeRow(model.LocalRowID(plan.Staged))
		a.setStatus(a.statusSummary())
		a.scheduleAutoLoadCheck()
		a.restoreScrollState()
		a.scheduleGraphCanvasDraw()
		return true
	case model.SelectionDisplayMessage:
		a.clearDetailText(plan.Message)
		a.setStatus(a.statusSummary())
		return true
	case model.SelectionDisplayCommit:
		a.selectCommitPlan(plan)
		return false
	default:
		return false
	}
}

func (a *Controller) selectCommitPlan(plan model.SelectionDisplayPlan) {
	id := model.CommitRowID(plan.Entry)
	a.focusTreeRow(id)
	if plan.LoadDetail {
		a.showCommitDetails(plan.Entry, plan.Index)
	}
}

func (a *Controller) focusTreeRow(id string) {
	if id == "" {
		return
	}
	a.ui.TreeView.Selection("set", id)
	a.ui.TreeView.Focus(id)
	a.ui.TreeView.See(id)
}

func (a *Controller) storeScrollState() {
	a.model.State.Scroll.Total = a.treeChildCount()
	if a.model.State.Scroll.Total > 0 {
		if start, _, err := a.treeYviewRange(); err == nil {
			a.model.State.Scroll.Start = start
		}
	}
}

func (a *Controller) restoreScrollState() {
	newTotal := a.treeChildCount()
	target, ok := a.model.State.Scroll.RestoreTarget(newTotal)
	if !ok {
		return
	}
	tkutil.MustEvalf("%s yview moveto %f", a.ui.TreeView, target)
}

func (a *Controller) treeChildCount() int {
	path := a.ui.TreeView.String()
	if path == "" {
		return 0
	}
	return len(a.ui.TreeView.Children(""))
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
