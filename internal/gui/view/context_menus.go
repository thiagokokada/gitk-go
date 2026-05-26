package view

import tk "modernc.org/tk9.0"

type ContextMenuHandlers struct {
	Tree                            func(*tk.Event)
	CopyCommitReference             func()
	Diff                            func(*tk.Event)
	CopySelection                   func()
	CopySelectionWithoutLineMarkers func()
	DiffFile                        func(*tk.Event)
	CopyFilePath                    func()
}

func (a *App) InitContextMenus(handlers ContextMenuHandlers) {
	a.initTreeContextMenu(handlers)
	a.initDiffContextMenu(handlers)
	a.initDiffFileContextMenu(handlers)
}

func (a *App) initTreeContextMenu(handlers ContextMenuHandlers) {
	menu := tk.App.Menu(tk.Tearoff(false))
	item := menu.AddCommand(tk.Command(handlers.CopyCommitReference))
	menu.EntryConfigure(item, tk.Lbl("Copy commit reference"))
	a.TreeContextMenu = menu
	if a.TreeView == nil || handlers.Tree == nil {
		return
	}
	tk.Bind(a.TreeView, "<Button-2>", tk.Command(handlers.Tree))
	tk.Bind(a.TreeView, "<Button-3>", tk.Command(handlers.Tree))
}

func (a *App) initDiffContextMenu(handlers ContextMenuHandlers) {
	menu := tk.App.Menu(tk.Tearoff(false))
	menu.AddCommand(tk.Lbl("Copy selection"), tk.Command(handlers.CopySelection))
	menu.AddCommand(
		tk.Lbl("Copy selection without +/- markers"),
		tk.Command(handlers.CopySelectionWithoutLineMarkers),
	)
	a.DiffContextMenu = menu
	if a.DiffDetail == nil || handlers.Diff == nil {
		return
	}
	tk.Bind(a.DiffDetail, "<Button-2>", tk.Command(handlers.Diff))
	tk.Bind(a.DiffDetail, "<Button-3>", tk.Command(handlers.Diff))
}

func (a *App) initDiffFileContextMenu(handlers ContextMenuHandlers) {
	menu := tk.App.Menu(tk.Tearoff(false))
	menu.AddCommand(tk.Lbl("Copy file path"), tk.Command(handlers.CopyFilePath))
	a.DiffFileContextMenu = menu
	if a.DiffFileList == nil || handlers.DiffFile == nil {
		return
	}
	tk.Bind(a.DiffFileList, "<Button-2>", tk.Command(handlers.DiffFile))
	tk.Bind(a.DiffFileList, "<Button-3>", tk.Command(handlers.DiffFile))
}

func (a *App) PopupTreeContextMenu(xRoot int, yRoot int) {
	if a.TreeContextMenu == nil {
		return
	}
	tk.Popup(a.TreeContextMenu.Window, xRoot, yRoot, nil)
}

func (a *App) PopupDiffContextMenu(xRoot int, yRoot int) {
	if a.DiffContextMenu == nil {
		return
	}
	tk.Popup(a.DiffContextMenu.Window, xRoot, yRoot, nil)
}

func (a *App) PopupDiffFileContextMenu(xRoot int, yRoot int) {
	if a.DiffFileContextMenu == nil {
		return
	}
	tk.Popup(a.DiffFileContextMenu.Window, xRoot, yRoot, nil)
}
