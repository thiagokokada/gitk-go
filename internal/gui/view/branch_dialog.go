package view

import (
	"strings"

	tk "modernc.org/tk9.0"
)

type BranchSwitchRow struct {
	Name      string
	Display   string
	IsCurrent bool
}

type BranchSwitchDialogHandlers struct {
	Filter func(string) []BranchSwitchRow
	Submit func(string)
}

func (a *App) ShowBranchSwitchDialog(current string, rows []BranchSwitchRow, handlers BranchSwitchDialogHandlers) {
	if a.BranchWindow != nil {
		tk.Destroy(a.BranchWindow.Window)
		a.BranchWindow = nil
	}

	visible := rows
	dialog := tk.App.Toplevel()
	a.BranchWindow = dialog
	dialog.WmTitle("Switch Branch")
	tk.WmTransient(dialog.Window, tk.App)
	tk.WmAttributes(dialog.Window, "-topmost", 1)

	frame := dialog.TFrame(tk.Padding("12p"))
	tk.Grid(frame, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.GridColumnConfigure(frame.Window, 0, tk.Weight(1))
	tk.GridRowConfigure(frame.Window, 2, tk.Weight(1))

	currentLabel := strings.TrimSpace(current)
	if currentLabel == "" || currentLabel == "HEAD" {
		currentLabel = "detached HEAD"
	}
	header := frame.TLabel(tk.Txt("Current: "+currentLabel), tk.Anchor(tk.W))
	tk.Grid(header, tk.Row(0), tk.Column(0), tk.Sticky(tk.WE), tk.Pady("0 8p"))

	filter := frame.TEntry(tk.Width(48), tk.Textvariable(""))
	tk.Grid(filter, tk.Row(1), tk.Column(0), tk.Sticky(tk.WE), tk.Pady("0 8p"))

	listFrame := frame.TFrame()
	tk.Grid(listFrame, tk.Row(2), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.GridColumnConfigure(listFrame.Window, 0, tk.Weight(1))
	tk.GridRowConfigure(listFrame.Window, 0, tk.Weight(1))

	scroll := listFrame.TScrollbar()
	list := listFrame.Listbox(tk.Exportselection(false), tk.Height(12))
	list.Configure(tk.Yscrollcommand(func(e *tk.Event) { e.ScrollSet(scroll) }))
	tk.Grid(list, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.Grid(scroll, tk.Row(0), tk.Column(1), tk.Sticky(tk.NS))
	scroll.Configure(tk.Command(func(e *tk.Event) { e.Yview(list) }))

	submit := func() {
		name, ok := selectedBranchName(list, visible)
		if !ok {
			return
		}
		tk.Destroy(dialog.Window)
		if handlers.Submit != nil {
			handlers.Submit(name)
		}
	}

	buttons := frame.TFrame()
	tk.Grid(buttons, tk.Row(3), tk.Column(0), tk.Sticky(tk.E), tk.Pady("8p 0"))
	cancelBtn := buttons.TButton(tk.Txt("Cancel"), tk.Command(func() { tk.Destroy(dialog.Window) }))
	switchBtn := buttons.TButton(tk.Txt("Switch"), tk.Command(submit))
	tk.Grid(cancelBtn, tk.Row(0), tk.Column(0), tk.Sticky(tk.E), tk.Padx("0 8p"))
	tk.Grid(switchBtn, tk.Row(0), tk.Column(1), tk.Sticky(tk.E))

	renderBranchRows(list, visible)
	tk.Bind(filter, "<KeyRelease>", tk.Command(func() {
		if handlers.Filter != nil {
			visible = handlers.Filter(filter.Textvariable())
		}
		renderBranchRows(list, visible)
	}))
	BindEmacsEntryShortcuts(filter)
	tk.Bind(list, "<Double-Button-1>", tk.Command(submit))
	tk.Bind(dialog.Window, "<KeyPress-Escape>", tk.Command(func() { tk.Destroy(dialog.Window) }))
	tk.Bind(dialog.Window, "<KeyPress-Return>", tk.Command(submit))
	tk.Bind(dialog.Window, "<Destroy>", tk.Command(func() {
		if a.BranchWindow == dialog {
			a.BranchWindow = nil
		}
	}))

	tk.Focus(filter)
	dialog.Center()
}

func renderBranchRows(list *tk.ListboxWidget, rows []BranchSwitchRow) {
	list.Delete(0, tk.END)
	for _, row := range rows {
		list.Insert(tk.END, row.Display)
	}
	if len(rows) == 0 {
		return
	}
	target := 0
	for i, row := range rows {
		if row.IsCurrent {
			target = i
			break
		}
	}
	list.SelectionClear(0, tk.END)
	list.SelectionSet(target)
	list.Activate(target)
	list.See(target)
}

func selectedBranchName(list *tk.ListboxWidget, rows []BranchSwitchRow) (name string, ok bool) {
	if list == nil || len(rows) == 0 {
		return "", false
	}
	selected := list.Curselection()
	if len(selected) == 0 {
		return "", false
	}
	idx := selected[0]
	if idx < 0 || idx >= len(rows) {
		return "", false
	}
	name = strings.TrimSpace(rows[idx].Name)
	return name, name != ""
}
