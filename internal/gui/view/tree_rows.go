package view

import (
	"github.com/thiagokokada/gitk-go/internal/gui/model"
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
