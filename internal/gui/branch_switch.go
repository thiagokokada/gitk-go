package gui

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/thiagokokada/gitk-go/internal/gui/selection"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
	. "modernc.org/tk9.0"
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
		MessageBox(
			Parent(App),
			Title("Switch Branch"),
			Icon("error"),
			Msg("No repository is currently open."),
			Type("ok"),
		)
		return
	}
	branches, head, err := a.svc.LocalBranchNames()
	if err != nil {
		MessageBox(
			Parent(App),
			Title("Switch Branch"),
			Icon("error"),
			Msg(fmt.Sprintf("Unable to list branches:\n\n%v", err)),
			Type("ok"),
		)
		return
	}
	if len(branches) == 0 {
		MessageBox(
			Parent(App),
			Title("Switch Branch"),
			Icon("info"),
			Msg("This repository has no local branches."),
			Type("ok"),
		)
		return
	}
	a.showBranchSwitchDialog(branches, head)
}

func (a *Controller) showBranchSwitchDialog(branches []string, current string) {
	if a.ui.branchWindow != nil {
		Destroy(a.ui.branchWindow.Window)
		a.ui.branchWindow = nil
	}

	all := buildBranchChoices(branches, current)
	visible := all

	dialog := App.Toplevel()
	a.ui.branchWindow = dialog
	dialog.WmTitle("Switch Branch")
	WmTransient(dialog.Window, App)
	WmAttributes(dialog.Window, "-topmost", 1)

	frame := dialog.TFrame(Padding("12p"))
	Grid(frame, Row(0), Column(0), Sticky(NEWS))
	GridColumnConfigure(frame.Window, 0, Weight(1))
	GridRowConfigure(frame.Window, 2, Weight(1))

	currentLabel := strings.TrimSpace(current)
	if currentLabel == "" || currentLabel == "HEAD" {
		currentLabel = "detached HEAD"
	}
	header := frame.TLabel(Txt(fmt.Sprintf("Current: %s", currentLabel)), Anchor(W))
	Grid(header, Row(0), Column(0), Sticky(WE), Pady("0 8p"))

	filter := frame.TEntry(Width(48), Textvariable(""))
	Grid(filter, Row(1), Column(0), Sticky(WE), Pady("0 8p"))

	listFrame := frame.TFrame()
	Grid(listFrame, Row(2), Column(0), Sticky(NEWS))
	GridColumnConfigure(listFrame.Window, 0, Weight(1))
	GridRowConfigure(listFrame.Window, 0, Weight(1))

	scroll := listFrame.TScrollbar()
	list := listFrame.Listbox(Exportselection(false), Height(12))
	list.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(scroll) }))
	Grid(list, Row(0), Column(0), Sticky(NEWS))
	Grid(scroll, Row(0), Column(1), Sticky(NS))
	scroll.Configure(Command(func(e *Event) { e.Yview(list) }))

	buttons := frame.TFrame()
	Grid(buttons, Row(3), Column(0), Sticky(E), Pady("8p 0"))
	cancelBtn := buttons.TButton(Txt("Cancel"), Command(func() { Destroy(dialog.Window) }))
	switchBtn := buttons.TButton(Txt("Switch"), Command(func() {
		a.applyBranchChoiceSelection(dialog, list, visible)
	}))
	Grid(cancelBtn, Row(0), Column(0), Sticky(E), Padx("0 8p"))
	Grid(switchBtn, Row(0), Column(1), Sticky(E))

	render := func() {
		list.Delete(0, END)
		for _, c := range visible {
			list.Insert(END, c.display)
		}
		if len(visible) == 0 {
			return
		}
		target := 0
		for i, c := range visible {
			if c.isCurrent {
				target = i
				break
			}
		}
		list.SelectionClear(0, END)
		list.SelectionSet(target)
		list.Activate(target)
		list.See(target)
	}
	render()

	Bind(filter, "<KeyRelease>", Command(func() {
		visible = filterBranchChoices(all, filter.Textvariable())
		render()
	}))
	Bind(list, "<Double-Button-1>", Command(func() { a.applyBranchChoiceSelection(dialog, list, visible) }))
	Bind(dialog.Window, "<KeyPress-Escape>", Command(func() { Destroy(dialog.Window) }))
	Bind(dialog.Window, "<KeyPress-Return>", Command(func() { a.applyBranchChoiceSelection(dialog, list, visible) }))
	Bind(dialog.Window, "<Destroy>", Command(func() {
		if a.ui.branchWindow == dialog {
			a.ui.branchWindow = nil
		}
	}))

	if _, err := tkutil.Eval("focus %s", filter); err != nil {
		slog.Debug("focus branch filter", slog.Any("error", err))
	}
	dialog.Center()
}

func (a *Controller) applyBranchChoiceSelection(dialog *ToplevelWidget, list *ListboxWidget, visible []branchChoice) {
	if list == nil || len(visible) == 0 {
		return
	}
	selected := list.Curselection()
	if len(selected) == 0 {
		return
	}
	idx := selected[0]
	if idx < 0 || idx >= len(visible) {
		return
	}
	branch := visible[idx].name
	if strings.TrimSpace(branch) == "" {
		return
	}
	if dialog != nil {
		Destroy(dialog.Window)
	}
	a.switchBranchAsync(branch)
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
		PostEvent(func() {
			if err != nil {
				MessageBox(
					Parent(App),
					Title("Switch Branch"),
					Icon("error"),
					Msg(fmt.Sprintf("Unable to switch branches:\n\n%v", err)),
					Type("ok"),
				)
				a.setStatus(fmt.Sprintf("Failed to switch branches: %v", err))
				return
			}

			a.cancelPendingDiffLoad()
			a.repo.headRef = ""
			a.data.commits = nil
			a.data.visible = nil
			a.state.tree = treeState{}
			a.state.localDiff = localDiffCache{}
			a.state.selection = selection.State{}

			if a.ui.diffFileList != nil {
				a.ui.diffFileList.Delete(0, END)
			}
			a.setFileSections(nil)
			a.setLocalRowVisibility(false, false)
			a.setLocalRowVisibility(true, false)

			a.clearTreeRows()
			a.clearDetailText("Select a commit to view its details.")
			a.showInitialLoadingRow()
			a.setStatus("Loading commits...")
			a.refreshLocalChangesAsync(true)
			a.reloadCommitsAsync()
		}, false)
	}()
}
