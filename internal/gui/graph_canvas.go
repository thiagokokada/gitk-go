package gui

import "github.com/thiagokokada/gitk-go/internal/gui/widgets"

func (a *Controller) scheduleGraphCanvasDraw() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.model.state.tree.graphCanvas == nil {
		return
	}
	a.model.state.tree.graphCanvas.ScheduleDraw(func() {
		a.drawGraphCanvas()
	})
}

func (a *Controller) drawGraphCanvas() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.model.state.tree.graphCanvas == nil {
		return
	}
	a.model.state.tree.graphCanvas.Draw(widgets.GraphCanvasDrawInput{
		Visible:   a.model.data.visible,
		Labels:    a.model.state.tree.branchLabels,
		Theme:     graphCanvasThemeForPalette(a.theme.palette),
		IndexByID: a.model.state.tree.rows.visibleByID,
	})
}

func graphCanvasThemeForPalette(palette colorPalette) widgets.GraphCanvasTheme {
	canvas := palette.GraphCanvas
	return widgets.GraphCanvasTheme{
		LaneColors:      canvas.LaneColors[:],
		SelectedRowFill: canvas.SelectedRowFill,
		NodeFill:        canvas.NodeFill,
		HeadNodeFill:    canvas.HeadNodeFill,
		HeadLabel:       graphCanvasLabelStyle(canvas.HeadLabel),
		TagLabel:        graphCanvasLabelStyle(canvas.TagLabel),
		BranchLabel:     graphCanvasLabelStyle(canvas.BranchLabel),
		DefaultLabel:    graphCanvasLabelStyle(canvas.DefaultLabel),
	}
}

func graphCanvasLabelStyle(palette graphLabelPalette) widgets.GraphCanvasLabelStyle {
	return widgets.GraphCanvasLabelStyle{
		Fill:    palette.Fill,
		Outline: palette.Outline,
		Text:    palette.Text,
	}
}
