//go:build graphcanvas

package gui

type treeState struct {
	branchLabels      map[string][]string
	contextTargetID   string
	hasMore           bool
	loadingBatch      bool
	showLocalUnstaged bool
	showLocalStaged   bool

	graphRedrawPending bool
	graphOverlay       graphOverlayState
}

type graphOverlayState struct {
	ready bool
	width int
	x     int
	y     int
	h     int
	bg    string
}
