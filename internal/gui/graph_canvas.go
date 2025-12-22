package gui

func (a *Controller) scheduleGraphCanvasRedraw() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.ui.graphCanvas == nil || a.ui.treeView == nil {
		return
	}
	a.state.tree.graphCanvas.ScheduleRedraw(func() {
		a.redrawGraphCanvas()
	})
}

func (a *Controller) redrawGraphCanvas() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.ui.graphCanvas == nil || a.ui.treeView == nil {
		return
	}
	a.state.tree.graphCanvas.Redraw(
		a.ui.graphCanvas,
		a.ui.treeView,
		a.data.visible,
		a.state.tree.branchLabels,
		a.theme.palette.isDark(),
	)
}
