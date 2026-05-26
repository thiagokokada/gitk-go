package view

import tk "modernc.org/tk9.0"

func (a *App) ShowShortcutsDialog(content string) {
	if a.ShortcutsWindow != nil {
		tk.Destroy(a.ShortcutsWindow.Window)
		a.ShortcutsWindow = nil
	}
	dialog := tk.App.Toplevel()
	a.ShortcutsWindow = dialog
	dialog.WmTitle("Keyboard Shortcuts")
	tk.WmTransient(dialog.Window, tk.App)
	tk.WmAttributes(dialog.Window, "-topmost", 1)

	frame := dialog.TFrame(tk.Padding("12p"))
	tk.Grid(frame, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.GridColumnConfigure(frame.Window, 0, tk.Weight(1))
	tk.GridRowConfigure(frame.Window, 1, tk.Weight(1))

	header := frame.TLabel(tk.Txt("Keyboard Shortcuts"), tk.Anchor(tk.W))
	tk.Grid(header, tk.Row(0), tk.Column(0), tk.Sticky(tk.W), tk.Pady("0 8p"))

	text := frame.Text(tk.Width(62), tk.Height(18), tk.Wrap(tk.WORD), tk.Exportselection(false))
	text.Insert("1.0", content)
	text.Configure(tk.State("disabled"))
	tk.Grid(text, tk.Row(1), tk.Column(0), tk.Sticky(tk.NEWS))

	closeBtn := frame.TButton(tk.Txt("Close"), tk.Command(func() { tk.Destroy(dialog.Window) }))
	tk.Grid(closeBtn, tk.Row(2), tk.Column(0), tk.Sticky(tk.E), tk.Pady("8p 0"))

	tk.Bind(dialog.Window, "<KeyPress-Escape>", tk.Command(func() { tk.Destroy(dialog.Window) }))
	tk.Bind(dialog.Window, "<Destroy>", tk.Command(func() {
		if a.ShortcutsWindow == dialog {
			a.ShortcutsWindow = nil
		}
	}))
	dialog.Center()
}

func (a *App) ShowMessage(title string, icon string, message string) {
	tk.MessageBox(
		tk.Parent(tk.App),
		tk.Title(title),
		tk.Icon(icon),
		tk.Msg(message),
		tk.Type("ok"),
	)
}

func (a *App) ChooseRepositoryDirectory(title string, initialDir string) string {
	return tk.ChooseDirectory(
		tk.Parent(tk.App),
		tk.Title(title),
		tk.Initialdir(initialDir),
		tk.Mustexist(true),
	)
}
