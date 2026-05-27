package view

import tk "modernc.org/tk9.0"

func (a *App) SetRepoLabel(text string) {
	if a.RepoLabel == nil {
		return
	}
	a.RepoLabel.Configure(tk.Txt(text))
}

func (a *App) SetStatus(text string) {
	tk.PostEvent(func() {
		if a.Status != nil {
			a.Status.Configure(tk.Txt(text))
		}
	}, false)
}

func (a *App) SetReloadButtonLabel(text string) {
	if a.ReloadButton == nil {
		return
	}
	a.ReloadButton.Configure(tk.Txt(text))
}

func (a *App) ClearFilterText() {
	if a.FilterEntry == nil {
		return
	}
	a.FilterEntry.Configure(tk.Textvariable(""))
}

func CopyToClipboard(text string) {
	tk.ClipboardClear()
	tk.ClipboardAppend(text)
}
