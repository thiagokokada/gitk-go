package view

import (
	"log/slog"

	tk "modernc.org/tk9.0"
)

func ShowFontDialog(title string, seed tk.Opt, apply func([]string, bool) bool) {
	tk.Fontchooser(
		tk.Parent(tk.App),
		tk.Title(title),
		seed,
		tk.Command(func() {
			if apply == nil || !apply(tk.FontchooserFont(), true) {
				slog.Debug("font selection missing or invalid", slog.String("title", title))
			}
		}),
	)
	tk.FontchooserShow()
}

func (a *App) ApplyNamedFontOptions(names []string, options []any) {
	for _, name := range names {
		tk.FontConfigure(name, options...)
	}
}

func (a *App) ApplyUIFontToStyles() {
	styles := []string{
		".",
		"TLabel",
		"TEntry",
		"TButton",
		"Treeview",
		"Treeview.Heading",
	}
	for _, style := range styles {
		tk.StyleConfigure(style, tk.Font(tk.DefaultFont))
	}
}

func (a *App) ApplyUIFontToWidgets() {
	if a.DiffFileList != nil {
		a.DiffFileList.Configure(tk.Font(tk.DefaultFont))
	}
	if a.TreeContextMenu != nil {
		a.TreeContextMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.DiffContextMenu != nil {
		a.DiffContextMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.Menubar != nil {
		a.Menubar.Configure(tk.Font(tk.DefaultFont))
	}
	if a.FileMenu != nil {
		a.FileMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.ViewMenu != nil {
		a.ViewMenu.Configure(tk.Font(tk.DefaultFont))
	}
	if a.HelpMenu != nil {
		a.HelpMenu.Configure(tk.Font(tk.DefaultFont))
	}
}

func (a *App) ApplyFixedFontToDiff() {
	if a.DiffDetail == nil {
		return
	}
	a.DiffDetail.Configure(tk.Font(tk.FixedFont))
}
