package view

import tk "modernc.org/tk9.0"

func (a *App) SetRepoLabel(text string) {
	a.RepoLabel.Configure(tk.Txt(text))
}

func (a *App) SetStatus(text string) {
	tk.PostEvent(func() {
		a.Status.Configure(tk.Txt(text))
	}, false)
}

func (a *App) SetReloadButtonLabel(text string) {
	a.ReloadButton.Configure(tk.Txt(text))
}

func (a *App) ClearFilterText() {
	a.FilterEntry.Configure(tk.Textvariable(""))
}

func CopyToClipboard(text string) {
	tk.ClipboardClear()
	tk.ClipboardAppend(text)
}
