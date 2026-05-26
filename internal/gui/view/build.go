package view

import (
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
	tk "modernc.org/tk9.0"
)

type Config struct {
	GraphCanvas        bool
	DiffDetailFontSpec []any
}

func (a *App) Build(cfg Config, handlers Handlers) {
	tk.GridColumnConfigure(tk.App, 0, tk.Weight(1))
	tk.GridRowConfigure(tk.App, 1, tk.Weight(1))

	controls := a.buildControls(handlers)
	tk.Grid(controls, tk.Row(0), tk.Column(0), tk.Sticky(tk.WE))

	mainPane := a.buildMainPane(cfg, handlers)
	tk.Grid(mainPane, tk.Row(1), tk.Column(0), tk.Sticky(tk.NEWS), tk.Padx("4p"), tk.Pady("4p"))

	a.Status = tk.App.TLabel(tk.Anchor(tk.W), tk.Relief(tk.SUNKEN), tk.Padding("4p"), tk.Font(tk.DefaultFont))
	tk.Grid(a.Status, tk.Row(2), tk.Column(0), tk.Sticky(tk.WE))
}

func (a *App) buildControls(handlers Handlers) *tk.TFrameWidget {
	controls := tk.App.TFrame(tk.Padding("4p"))
	tk.GridColumnConfigure(controls.Window, 1, tk.Weight(1))

	a.RepoLabel = controls.TLabel(tk.Anchor(tk.W), tk.Font(tk.DefaultFont))
	tk.Grid(a.RepoLabel, tk.Row(0), tk.Column(0), tk.Columnspan(4), tk.Sticky(tk.W))

	filterLabel := controls.TLabel(tk.Txt("Filter:"), tk.Anchor(tk.E), tk.Font(tk.DefaultFont))
	tk.Grid(filterLabel, tk.Row(1), tk.Column(0), tk.Sticky(tk.E))
	a.FilterEntry = controls.TEntry(tk.Width(40), tk.Textvariable(""))
	tk.Grid(a.FilterEntry, tk.Row(1), tk.Column(1), tk.Sticky(tk.WE), tk.Padx("4p"))

	tk.Bind(a.FilterEntry, "<KeyRelease>", tk.Command(func() {
		if handlers.ApplyFilter != nil {
			handlers.ApplyFilter(a.FilterEntry.Textvariable())
		}
	}))

	clearBtn := controls.TButton(tk.Txt("Clear"), tk.Command(func() {
		a.FilterEntry.Configure(tk.Textvariable(""))
		if handlers.ClearFilter != nil {
			handlers.ClearFilter()
		}
	}))
	tk.Grid(clearBtn, tk.Row(1), tk.Column(2), tk.Sticky(tk.E), tk.Padx("4p"))
	a.ReloadButton = controls.TButton(tk.Txt("Reload"), tk.Command(func() {
		if handlers.Reload != nil {
			handlers.Reload()
		}
	}))
	tk.Grid(a.ReloadButton, tk.Row(1), tk.Column(3), tk.Sticky(tk.E))
	return controls
}

func (a *App) buildMainPane(cfg Config, handlers Handlers) *tk.TPanedwindowWidget {
	pane := tk.App.TPanedwindow(tk.Orient(tk.VERTICAL))
	listArea := pane.TFrame()
	diffArea := pane.TFrame()
	pane.Add(listArea.Window)
	pane.Add(diffArea.Window)

	a.buildCommitPane(listArea, cfg, handlers)
	a.buildDiffPane(diffArea, cfg, handlers)

	var (
		lastHeight  int
		stableCount int
		pending     bool
	)
	setSashOnce := func() {
		if pending {
			return
		}
		pending = true
		tk.TclAfterIdle(func() {
			pending = false
			height := tkutil.Atoi(tk.WinfoHeight(pane.Window))
			if height <= 1 {
				return
			}
			if height == lastHeight {
				stableCount++
			} else {
				stableCount = 0
				lastHeight = height
			}
			target := (height + 2) / 4
			tkutil.MustEvalf("%s sashpos 0 %d", pane, target)
			if stableCount >= 1 {
				tk.Bind(pane, "<Configure>", "")
			}
		})
	}
	tk.Bind(pane, "<Configure>", tk.Command(func(e *tk.Event) {
		setSashOnce()
	}))

	return pane
}

func (a *App) buildCommitPane(listArea *tk.TFrameWidget, cfg Config, handlers Handlers) {
	tk.GridRowConfigure(listArea.Window, 0, tk.Weight(1))
	tk.GridRowConfigure(listArea.Window, 1, tk.Weight(0))
	tk.GridColumnConfigure(listArea.Window, 0, tk.Weight(1))
	tk.GridColumnConfigure(listArea.Window, 1, tk.Weight(0))

	treeScroll := listArea.TScrollbar()
	if cfg.GraphCanvas {
		// Avoid setting Background(""): Tk treats it as an invalid color name.
		a.GraphCanvas = listArea.Canvas(tk.Width(120), tk.Highlightthickness(0), tk.Borderwidth(0))
	} else {
		a.GraphCanvas = nil
	}
	a.TreeView = listArea.TTreeview(
		tk.Show("headings"),
		tk.Columns("graph commit author date"),
		tk.Selectmode("browse"),
		tk.Height(18),
		tk.Yscrollcommand(func(e *tk.Event) {
			e.ScrollSet(treeScroll)
			if handlers.TreeScrolled != nil {
				handlers.TreeScrolled()
			}
			if handlers.ScheduleGraphDraw != nil {
				handlers.ScheduleGraphDraw()
			}
		}),
	)
	if cfg.GraphCanvas {
		a.TreeView.Column("graph", tk.Anchor(tk.W), tk.Width(260), tk.Stretch(false))
	} else {
		a.TreeView.Column("graph", tk.Anchor(tk.W), tk.Width(120), tk.Stretch(false))
	}
	a.TreeView.Column("commit", tk.Anchor(tk.W), tk.Width(380))
	a.TreeView.Column("author", tk.Anchor(tk.W), tk.Width(280))
	a.TreeView.Column("date", tk.Anchor(tk.W), tk.Width(180))
	a.TreeView.Heading("graph", tk.Txt("Graph"))
	a.TreeView.Heading("commit", tk.Txt("Commit"))
	a.TreeView.Heading("author", tk.Txt("Author"))
	a.TreeView.Heading("date", tk.Txt("Date"))
	tk.Grid(a.TreeView, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.Grid(treeScroll, tk.Row(0), tk.Column(1), tk.Sticky(tk.NS))
	treeScroll.Configure(tk.Command(func(e *tk.Event) {
		e.Yview(a.TreeView)
		if handlers.ScheduleGraphDraw != nil {
			handlers.ScheduleGraphDraw()
		}
	}))

	tk.Bind(a.TreeView, "<<TreeviewSelect>>", tk.Command(func() {
		if handlers.TreeSelection != nil {
			handlers.TreeSelection()
		}
	}))
	if cfg.GraphCanvas {
		tk.Bind(a.TreeView, "<Configure>", tk.Command(func() {
			if handlers.ScheduleGraphDraw != nil {
				handlers.ScheduleGraphDraw()
			}
		}))
		tk.Bind(a.TreeView, "<B1-Motion>", tk.Command(func() {
			if handlers.ScheduleGraphDraw != nil {
				handlers.ScheduleGraphDraw()
			}
		}))
		tk.Bind(a.TreeView, "<ButtonRelease-1>", tk.Command(func() {
			if handlers.ScheduleGraphDraw != nil {
				handlers.ScheduleGraphDraw()
			}
		}))
	}
}

func (a *App) buildDiffPane(diffArea *tk.TFrameWidget, cfg Config, handlers Handlers) {
	tk.GridRowConfigure(diffArea.Window, 0, tk.Weight(1))
	tk.GridColumnConfigure(diffArea.Window, 0, tk.Weight(1))

	diffPane := diffArea.TPanedwindow(tk.Orient(tk.HORIZONTAL))
	tk.Grid(diffPane, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))

	textFrame := diffPane.TFrame()
	fileFrame := diffPane.TFrame()
	diffPane.Add(textFrame.Window, tk.Weight(5))
	diffPane.Add(fileFrame.Window, tk.Weight(1))

	tk.GridRowConfigure(fileFrame.Window, 0, tk.Weight(1))
	tk.GridColumnConfigure(fileFrame.Window, 0, tk.Weight(1))
	tk.GridRowConfigure(textFrame.Window, 0, tk.Weight(1))
	tk.GridColumnConfigure(textFrame.Window, 0, tk.Weight(1))

	detailYScroll := textFrame.TScrollbar(tk.Command(func(e *tk.Event) { e.Yview(a.DiffDetail) }))
	detailXScroll := textFrame.TScrollbar(
		tk.Orient(tk.HORIZONTAL),
		tk.Command(func(e *tk.Event) { e.Xview(a.DiffDetail) }),
	)
	a.DiffDetail = textFrame.Text(
		tk.Wrap(tk.NONE),
		tk.Font(cfg.DiffDetailFontSpec...),
		tk.Exportselection(false),
		tk.Tabs("1c"),
	)
	a.DiffDetail.Configure(tk.Yscrollcommand(func(e *tk.Event) {
		e.ScrollSet(detailYScroll)
		if handlers.DiffScrolled != nil {
			handlers.DiffScrolled()
		}
	}))
	a.DiffDetail.Configure(tk.Xscrollcommand(func(e *tk.Event) { e.ScrollSet(detailXScroll) }))
	tk.Grid(a.DiffDetail, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.Grid(detailYScroll, tk.Row(0), tk.Column(1), tk.Sticky(tk.NS))
	tk.Grid(detailXScroll, tk.Row(1), tk.Column(0), tk.Sticky(tk.WE))
	a.DiffDetail.Configure(tk.State("disabled"))

	fileScroll := fileFrame.TScrollbar()
	a.DiffFileList = fileFrame.Text(tk.Wrap(tk.NONE), tk.Width(40), tk.Exportselection(false))
	a.DiffFileList.Configure(tk.Yscrollcommand(func(e *tk.Event) { e.ScrollSet(fileScroll) }))
	a.DiffFileList.Configure(tk.State("disabled"))
	tk.Grid(a.DiffFileList, tk.Row(0), tk.Column(0), tk.Sticky(tk.NEWS))
	tk.Grid(fileScroll, tk.Row(0), tk.Column(1), tk.Sticky(tk.NS))
	fileScroll.Configure(tk.Command(func(e *tk.Event) { e.Yview(a.DiffFileList) }))
	tk.Bind(a.DiffFileList, "<Button-1>", tk.Command(func(e *tk.Event) {
		if handlers.FileSelection != nil {
			handlers.FileSelection(e)
		}
	}))
}
