package gui

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	. "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

func (a *Controller) onTreeSelectionChanged() {
	a.scheduleGraphCanvasDraw()
	sel := a.ui.TreeView.Selection("")
	if len(sel) == 0 {
		a.model.TreeSelectionPlan("")
		return
	}
	plan := a.model.TreeSelectionPlan(sel[0])
	switch plan.Kind {
	case model.TreeSelectionLocal:
		a.showLocalChanges(plan.Staged)
		return
	case model.TreeSelectionCommit:
		a.showCommitDetails(plan.Entry, plan.Index)
	default:
		return
	}
}

func (a *Controller) setLocalRowVisibility(staged bool, show bool) {
	if !a.model.State.Tree.SetLocalRowVisible(staged, show) {
		return
	}
	id := model.LocalRowID(staged)
	if show {
		if !a.model.State.Tree.Rows.HasSpecialItem(id) {
			a.insertSingleLocalRow(staged)
		}
		return
	}
	if a.model.State.Tree.Rows.HasSpecialItem(id) {
		a.ui.TreeView.Delete(id)
		a.model.State.Tree.Rows.RemoveSpecialItem(id)
	}
	if stagedSelection, ok := a.model.State.Selection.LocalSelection(); ok && stagedSelection == staged {
		a.selectFallbackCommit()
	}
}

func (a *Controller) insertSingleLocalRow(staged bool) {
	label := localRowLabel(staged)
	index := a.model.State.Tree.LocalRowInsertIndex(staged)
	a.ui.InsertLocalRow(staged, index, label)
	a.model.State.Tree.Rows.AddSpecialItem(model.LocalRowID(staged))
}

func localRowLabel(staged bool) string {
	if staged {
		return localStagedLabel
	}
	return localUnstagedLabel
}

func (a *Controller) clearTreeRows() {
	children := a.ui.TreeView.Children("")
	attached := make(map[string]struct{}, len(children))
	if len(children) == 0 {
		children = nil
	}
	if len(children) > 0 {
		args := make([]any, len(children))
		for i, child := range children {
			args[i] = child
			attached[child] = struct{}{}
		}
		a.ui.TreeView.Delete(args...)
	}
	for _, id := range a.model.State.Tree.Rows.TrackedItemIDs() {
		if _, ok := attached[id]; ok {
			continue
		}
		a.ui.TreeView.Delete(id)
	}
	a.model.State.Tree.Rows.ResetTracking()
}

func (a *Controller) syncTreeRows() {
	if a.ui.TreeView == nil {
		return
	}
	a.ensureLocalRows()
	a.pruneCommitRows()

	refresh := a.model.State.Tree.Rows.RefreshValues
	ordered := make([]string, 0, len(a.model.Data.Visible)+3)
	ordered = append(ordered, a.model.State.Tree.LocalRowIDs()...)
	for _, entry := range a.model.Data.Visible {
		id := model.CommitRowID(entry)
		if id == "" {
			continue
		}
		if !a.model.State.Tree.Rows.HasItem(id) {
			a.insertCommitRow(id, entry)
		} else if refresh {
			a.updateCommitRow(id, entry)
		}
		ordered = append(ordered, id)
	}

	if a.model.State.Tree.HasMore && len(a.model.Data.Visible) > 0 {
		a.ensureMoreIndicatorRow()
		ordered = append(ordered, model.MoreIndicatorID)
	}

	if a.model.State.Tree.LoadingBatch && len(a.model.Data.Visible) == 0 {
		a.ensureLoadingIndicatorRow()
		ordered = append(ordered, model.LoadingIndicatorID)
	}

	a.setTreeChildren(ordered)
	a.model.State.Tree.Rows.RefreshValues = false
}

func (a *Controller) ensureLocalRows() {
	for _, id := range a.model.State.Tree.LocalRowIDs() {
		if a.model.State.Tree.Rows.HasSpecialItem(id) {
			continue
		}
		a.insertSingleLocalRow(id == model.LocalStagedRowID)
	}
}

func (a *Controller) ensureMoreIndicatorRow() {
	if a.model.State.Tree.Rows.HasSpecialItem(model.MoreIndicatorID) {
		return
	}
	a.ui.InsertMoreIndicatorRow()
	a.model.State.Tree.Rows.AddSpecialItem(model.MoreIndicatorID)
}

func (a *Controller) ensureLoadingIndicatorRow() {
	if a.model.State.Tree.Rows.HasSpecialItem(model.LoadingIndicatorID) {
		return
	}
	a.ui.InsertLoadingIndicatorRow()
	a.model.State.Tree.Rows.AddSpecialItem(model.LoadingIndicatorID)
}

func (a *Controller) insertCommitRow(id string, entry *git.Entry) {
	row, ok := treeRowData(entry, a.model.State.Tree.BranchLabels, a.cfg.graphCanvas)
	if !ok {
		return
	}
	a.ui.InsertCommitRow(id, row)
	a.model.State.Tree.Rows.AddItem(id)
	a.model.State.Tree.Rows.SetItemValue(id, row)
}

func (a *Controller) updateCommitRow(id string, entry *git.Entry) {
	row, ok := treeRowData(entry, a.model.State.Tree.BranchLabels, a.cfg.graphCanvas)
	if !ok {
		return
	}
	if !a.model.State.Tree.Rows.ItemValueChanged(id, row) {
		return
	}
	if !a.ui.UpdateCommitRow(id, row) {
		return
	}
	a.model.State.Tree.Rows.SetItemValue(id, row)
}

func (a *Controller) setTreeChildren(ids []string) {
	treePath := a.ui.TreeView.String()
	if treePath == "" {
		return
	}
	if len(ids) == 0 {
		if _, err := tkutil.Evalf("%s children {} {}", treePath); err != nil {
			slog.Debug("tree children clear", slog.Any("error", err))
		}
		return
	}
	// XXX: Workaround a bug in Tk-go, ideally we would use a.ui.TreeView.Children("", ids...) instead
	children := tkutil.TclSafeStrings(ids...)
	if _, err := tkutil.Evalf("%s children {} {%s}", treePath, children); err != nil {
		slog.Debug("tree children set", slog.Any("error", err))
	}
}

func (a *Controller) pruneCommitRows() {
	for _, id := range a.model.State.Tree.Rows.PruneStaleCommitRows() {
		a.ui.TreeView.Delete(id)
	}
}

func (a *Controller) scheduleAutoLoadCheck() {
	if a.model.State.Filter.Value == "" || !a.model.State.Tree.HasMore {
		return
	}
	slog.Debug("scheduleAutoLoadCheck",
		slog.String("filter", a.model.State.Filter.Value),
		slog.Int("visible", len(a.model.Data.Visible)),
		slog.Bool("has_more", a.model.State.Tree.HasMore),
	)
	PostEvent(func() {
		a.maybeLoadMoreOnScroll()
	}, false)
}

func (a *Controller) maybeLoadMoreOnScroll() {
	if a.model.State.Tree.LoadingBatch || !a.model.State.Tree.HasMore {
		return
	}
	start, end, err := a.treeYviewRange()
	if err != nil {
		slog.Error("tree yview", slog.Any("error", err))
		return
	}
	if a.model.State.Tree.ShouldLoadMoreOnScroll(
		a.model.State.Filter.Value,
		len(a.model.Data.Visible),
		int(a.cfg.batch),
		start,
		end,
	) {
		a.loadMoreCommitsAsync(false)
	}
}

func (a *Controller) treeYviewRange() (start float64, end float64, err error) {
	path := a.ui.TreeView.String()
	if path == "" {
		return 0, 0, fmt.Errorf("tree widget has empty path")
	}
	out, err := tkutil.Evalf("%s yview", path)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected treeview yview output %q", out)
	}
	start, err = strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, err
	}
	end, err = strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}
