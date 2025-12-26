package gui

import "github.com/thiagokokada/gitk-go/internal/gui/widgets"

func (a *Controller) scheduleGraphCanvasDraw() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.state.tree.graphCanvas == nil {
		return
	}
	a.state.tree.graphCanvas.ScheduleDraw(func() {
		a.drawGraphCanvas()
	})
}

func (a *Controller) drawGraphCanvas() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.state.tree.graphCanvas == nil {
		return
	}
	a.state.tree.graphCanvas.Draw(widgets.GraphCanvasDrawInput{
		Visible: a.data.visible,
		Labels:  a.state.tree.branchLabels,
		Dark:    a.theme.palette.isDark(),
	})
}
