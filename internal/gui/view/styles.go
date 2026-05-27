package view

import tk "modernc.org/tk9.0"

type TreeRowColors struct {
	LocalUnstaged string
	LocalStaged   string
}

type DiffColors struct {
	Add    string
	Delete string
	Header string
}

type DiffFileListColors struct {
	AddText    string
	DeleteText string
}

func (a *App) ApplyTreeRowStyles(colors TreeRowColors) {
	if a.TreeView == nil {
		return
	}
	a.TreeView.TagConfigure("localUnstaged", tk.Background(colors.LocalUnstaged))
	a.TreeView.TagConfigure("localStaged", tk.Background(colors.LocalStaged))
}

func (a *App) ApplyDiffTagStyles(colors DiffColors) {
	if a.DiffDetail == nil {
		return
	}
	selBg := a.DiffDetail.Selectbackground()
	selFg := a.DiffDetail.Selectforeground()
	tagOpts := func(bg string) []tk.Opt {
		opts := []tk.Opt{tk.Background(bg)}
		if selBg != "" {
			opts = append(opts, tk.Selectbackground(selBg))
		}
		if selFg != "" {
			opts = append(opts, tk.Selectforeground(selFg))
		}
		return opts
	}
	a.DiffDetail.TagConfigure(DiffTagAdd, tagOpts(colors.Add)...)
	a.DiffDetail.TagConfigure(DiffTagDelete, tagOpts(colors.Delete)...)
	a.DiffDetail.TagConfigure(DiffTagHeader, tagOpts(colors.Header)...)
}

func (a *App) ApplyDiffFileListStyles(colors DiffFileListColors) {
	if a.DiffFileList == nil {
		return
	}
	a.DiffFileList.TagConfigure("diffFileAddCount", tk.Foreground(colors.AddText))
	a.DiffFileList.TagConfigure("diffFileDelCount", tk.Foreground(colors.DeleteText))
	selBg := a.DiffFileList.Selectbackground()
	if selBg == "" {
		return
	}
	a.DiffFileList.TagConfigure("diffFileSelected", tk.Background(selBg))
}

func (a *App) CurrentDiffText() string {
	if a.DiffDetail == nil {
		return ""
	}
	return a.DiffDetail.Text()
}
