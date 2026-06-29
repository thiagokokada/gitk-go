package view

import (
	"fmt"
	"strconv"
	"strings"

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
	a.TreeView.Insert("", "end", tk.Id(id), tk.Values(row.Values()))
}

func (a *App) UpdateCommitRow(id string, row model.TreeRow) bool {
	if a.TreeView.String() == "" {
		return false
	}
	a.TreeView.Item(id, tk.Values(row.Values()))
	return true
}

func (a *App) ClearTreeRows(trackedIDs []string) {
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
	for _, id := range ids {
		a.TreeView.Delete(id)
	}
}

func (a *App) SetTreeChildren(ids []string) error {
	treePath := a.TreeView.String()
	if treePath == "" {
		return nil
	}
	if len(ids) == 0 {
		_, err := tkutil.Evalf("%s children {} {}", treePath)
		return err
	}
	// XXX: Tk-go v1.75.4 flattens new children but passes the result to tclSafeList
	// without variadic expansion, so []string becomes one escaped "[a b]" item.
	children := tkutil.TclSafeStrings(ids...)
	_, err := tkutil.Evalf("%s children {} {%s}", treePath, children)
	return err
}

func (a *App) TreeExists() bool {
	path := a.TreeView.String()
	if path == "" {
		return false
	}
	return tkutil.WidgetExists(path)
}

func (a *App) SelectedTreeRow() string {
	sel := a.TreeView.Selection("")
	if len(sel) == 0 {
		return ""
	}
	return sel[0]
}

func (a *App) SelectTreeRow(id string) {
	if id == "" {
		return
	}
	a.TreeView.Selection("set", id)
	a.TreeView.Focus(id)
	a.TreeView.See(id)
}

func (a *App) FocusTree() {
	if a.TreeView.String() == "" {
		tk.Focus(tk.App)
		return
	}
	tk.Focus(a.TreeView)
}

func (a *App) FocusTreeRowAt(x int, y int) string {
	id := a.TreeRowAt(x, y)
	if id == "" {
		return ""
	}
	tk.Focus(a.TreeView)
	a.TreeView.Selection("set", id)
	a.TreeView.Focus(id)
	return id
}

func (a *App) TreeRowAt(x int, y int) string {
	return strings.TrimSpace(a.TreeView.IdentifyItem(x, y))
}

func (a *App) TreeChildCount() int {
	if a.TreeView.String() == "" {
		return 0
	}
	return len(a.TreeView.Children(""))
}

func (a *App) MoveTreeYview(target float64) error {
	if a.TreeView.String() == "" {
		return nil
	}
	_, err := tkutil.Evalf("%s yview moveto %f", a.TreeView, target)
	return err
}

func (a *App) ScrollTreeYview(delta int, unit ScrollUnit) error {
	if a.TreeView.String() == "" || delta == 0 {
		return nil
	}
	switch unit {
	case ScrollPages, ScrollUnits:
	default:
		return fmt.Errorf("unsupported tree scroll unit %q", unit)
	}
	_, err := tkutil.Evalf("%s yview scroll %d %s", a.TreeView, delta, unit)
	return err
}

func (a *App) TreeYviewRange() (start float64, end float64, err error) {
	path := a.TreeView.String()
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
