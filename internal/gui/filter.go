package gui

import (
	"log/slog"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/model"
)

func (a *Controller) applyFilter(raw string) {
	if a.ui.FilterText() != raw {
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
	if !a.ui.TreeExists() {
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
		a.ui.SelectTreeRow(model.LocalRowID(plan.Staged))
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
	id := plan.Entry.Commit.Hash
	a.ui.SelectTreeRow(id)
	if plan.LoadDetail {
		a.showCommitDetails(plan.Entry, plan.Index)
	}
}

func (a *Controller) storeScrollState() {
	a.model.State.Scroll.Total = a.ui.TreeChildCount()
	if a.model.State.Scroll.Total > 0 {
		if start, _, err := a.ui.TreeYviewRange(); err == nil {
			a.model.State.Scroll.Start = start
		}
	}
}

func (a *Controller) restoreScrollState() {
	newTotal := a.ui.TreeChildCount()
	target, ok := a.model.State.Scroll.RestoreTarget(newTotal)
	if !ok {
		return
	}
	if err := a.ui.MoveTreeYview(target); err != nil {
		slog.Error("tree yview restore", slog.Any("error", err))
	}
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
