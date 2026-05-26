package view

import (
	"runtime"

	tk "modernc.org/tk9.0"
)

type MenuHandlers struct {
	OpenRepository func()
	SwitchBranch   func()
	UIFont         func()
	FixedFont      func()
	Shortcuts      func()
	About          func()
}

func (a *App) InitMenubar(handlers MenuHandlers) {
	menubar := tk.Menu(tk.Tearoff(false))
	a.Menubar = menubar

	openAccel := "Ctrl+O"
	branchAccel := "Ctrl+B"
	if runtime.GOOS == "darwin" {
		openAccel = "Cmd+O"
		branchAccel = "Cmd+B"
	}

	fileMenu := menubar.Menu(tk.Tearoff(false))
	a.FileMenu = fileMenu
	fileMenu.AddCommand(tk.Lbl("Open Repository..."), tk.Accelerator(openAccel), tk.Command(handlers.OpenRepository))
	fileMenu.AddCommand(tk.Lbl("Switch Branch..."), tk.Accelerator(branchAccel), tk.Command(handlers.SwitchBranch))
	fileMenu.AddSeparator()
	fileMenu.AddCommand(tk.Lbl("Quit"), tk.Command(func() { tk.Destroy(tk.App) }))
	menubar.AddCascade(tk.Lbl("File"), tk.Mnu(fileMenu))

	viewMenu := menubar.Menu(tk.Tearoff(false))
	a.ViewMenu = viewMenu
	viewMenu.AddCommand(tk.Lbl("UI Font..."), tk.Command(handlers.UIFont))
	viewMenu.AddCommand(tk.Lbl("Fixed Font..."), tk.Command(handlers.FixedFont))
	menubar.AddCascade(tk.Lbl("View"), tk.Mnu(viewMenu))

	helpMenu := menubar.Menu(tk.Tearoff(false))
	a.HelpMenu = helpMenu
	helpMenu.AddCommand(tk.Lbl("Keyboard Shortcuts"), tk.Command(handlers.Shortcuts))
	helpMenu.AddCommand(tk.Lbl("About gitk-go"), tk.Command(handlers.About))
	menubar.AddCascade(tk.Lbl("Help"), tk.Mnu(helpMenu))

	tk.App.Configure(tk.Mnu(menubar))
}
