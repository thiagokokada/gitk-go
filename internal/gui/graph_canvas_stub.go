//go:build !graphcanvas

package gui

func (a *Controller) scheduleGraphCanvasRedraw() {
	// No-op when the graph canvas is not compiled in.
}
