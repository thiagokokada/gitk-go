package gui

import "github.com/thiagokokada/gitk-go/internal/gui/widgets"

func (a *Controller) scheduleGraphCanvasDraw() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.runtime.graphCanvas == nil {
		return
	}
	a.runtime.graphCanvas.ScheduleDraw(func() {
		a.drawGraphCanvas()
	})
}

func (a *Controller) drawGraphCanvas() {
	if !a.cfg.graphCanvas {
		return
	}
	if a.runtime.graphCanvas == nil {
		return
	}
	a.runtime.graphCanvas.Draw(widgets.GraphCanvasDrawInput{
		Visible:   a.model.Data.Visible,
		Labels:    a.model.State.Tree.BranchLabels,
		Theme:     graphCanvasThemeForPalette(a.theme.palette),
		IndexByID: a.model.State.Tree.Rows.VisibleByID,
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
