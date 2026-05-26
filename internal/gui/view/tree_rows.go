package view

import (
	"github.com/thiagokokada/gitk-go/internal/gui/model"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
	tk "modernc.org/tk9.0"
)

func (a *App) InsertLocalRow(staged bool, index int, label string) {
	tag := "localUnstaged"
	if staged {
		tag = "localStaged"
	}
	values := []string{"", label, "", ""}
	a.TreeView.Insert("", index, tk.Id(model.LocalRowID(staged)), tk.Values(values), tk.Tags(tag))
}

func (a *App) InsertMoreIndicatorRow() {
	values := []string{"", "There are more commits...", "", ""}
	a.TreeView.Insert("", "end", tk.Id(model.MoreIndicatorID), tk.Values(values))
}

func (a *App) InsertLoadingIndicatorRow() {
	values := []string{"", "Loading commits...", "", ""}
	a.TreeView.Insert("", "end", tk.Id(model.LoadingIndicatorID), tk.Values(values))
}

func (a *App) InsertCommitRow(id string, row model.TreeRow) {
	if a.TreeView == nil {
		return
	}
	a.TreeView.Insert("", "end", tk.Id(id), tk.Values(row.Values()))
}

func (a *App) UpdateCommitRow(id string, row model.TreeRow) bool {
	if a.TreeView == nil || a.TreeView.String() == "" {
		return false
	}
	a.TreeView.Item(id, tk.Values(row.Values()))
	return true
}

func (a *App) ClearTreeRows(trackedIDs []string) {
	if a.TreeView == nil {
		return
	}
	children := a.TreeView.Children("")
	attached := make(map[string]struct{}, len(children))
	if len(children) > 0 {
		args := make([]any, len(children))
		for i, child := range children {
			args[i] = child
			attached[child] = struct{}{}
		}
		a.TreeView.Delete(args...)
	}
	for _, id := range trackedIDs {
		if _, ok := attached[id]; ok {
			continue
		}
		a.TreeView.Delete(id)
	}
}

func (a *App) DeleteTreeRows(ids []string) {
	if a.TreeView == nil {
		return
	}
	for _, id := range ids {
		a.TreeView.Delete(id)
	}
}

func (a *App) SetTreeChildren(ids []string) error {
	if a.TreeView == nil {
		return nil
	}
	treePath := a.TreeView.String()
	if treePath == "" {
		return nil
	}
	if len(ids) == 0 {
		_, err := tkutil.Evalf("%s children {} {}", treePath)
		return err
	}
	// XXX: Workaround a bug in Tk-go, ideally we would use a.TreeView.Children("", ids...) instead.
	children := tkutil.TclSafeStrings(ids...)
	_, err := tkutil.Evalf("%s children {} {%s}", treePath, children)
	return err
}
