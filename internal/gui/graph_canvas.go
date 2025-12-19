package gui

func (a *Controller) scheduleGraphCanvasRedraw() {
	if !a.graphCanvas {
		return
	}
	if a.ui.graphCanvas == nil || a.ui.treeView == nil {
		return
	}
	a.tree.graphCanvas.ScheduleRedraw(func() {
		a.redrawGraphCanvas()
	})
}

func (a *Controller) redrawGraphCanvas() {
	if !a.graphCanvas {
		return
	}
	if a.ui.graphCanvas == nil || a.ui.treeView == nil {
		return
	}
	a.tree.graphCanvas.Redraw(a.ui.graphCanvas, a.ui.treeView, a.visible, a.tree.branchLabels, a.palette.isDark())
}
