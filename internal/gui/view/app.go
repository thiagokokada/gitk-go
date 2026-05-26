package view

import tk "modernc.org/tk9.0"

type Handlers struct {
	ApplyFilter       func(string)
	ClearFilter       func()
	Reload            func()
	TreeSelection     func()
	TreeScrolled      func()
	DiffScrolled      func()
	FileSelection     func(*tk.Event)
	ScheduleGraphDraw func()
}

type App struct {
	Menubar             *tk.MenuWidget
	FileMenu            *tk.MenuWidget
	ViewMenu            *tk.MenuWidget
	HelpMenu            *tk.MenuWidget
	Status              *tk.TLabelWidget
	RepoLabel           *tk.TLabelWidget
	FilterEntry         *tk.TEntryWidget
	ReloadButton        *tk.TButtonWidget
	GraphCanvas         *tk.CanvasWidget
	TreeView            *tk.TTreeviewWidget
	TreeContextMenu     *tk.MenuWidget
	DiffDetail          *tk.TextWidget
	DiffFileList        *tk.TextWidget
	DiffContextMenu     *tk.MenuWidget
	DiffFileContextMenu *tk.MenuWidget
	ShortcutsWindow     *tk.ToplevelWidget
	BranchWindow        *tk.ToplevelWidget
}
