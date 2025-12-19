package widgets

import (
	"strconv"
	"strings"

	. "modernc.org/tk9.0"

	"github.com/thiagokokada/gitk-go/internal/git"
	"github.com/thiagokokada/gitk-go/internal/gui/tkutil"
)

const (
	graphCanvasLaneSpacing = 8
	graphCanvasLaneMargin  = 6
	graphCanvasLineWidth   = 2

	graphCanvasLabelPadX  = 4
	graphCanvasLabelPadY  = 2
	graphCanvasLabelGap   = 6
	graphCanvasLabelMinX  = 2
	graphCanvasConnectorW = 1

	graphCanvasLabelFont = "TkDefaultFont 9"
)

type GraphCanvas struct {
	redrawPending bool
	overlay       graphOverlayState
}

type graphOverlayState struct {
	ready bool
	width int
	x     int
	y     int
	h     int
	bg    string
}

func (g *GraphCanvas) ScheduleRedraw(redraw func()) {
	if g.redrawPending {
		return
	}
	g.redrawPending = true
	PostEvent(func() {
		g.redrawPending = false
		if redraw != nil {
			redraw()
		}
	}, false)
}

func (g *GraphCanvas) Redraw(canvas *CanvasWidget, treeView *TTreeviewWidget, visible []*git.Entry, labels map[string][]string, dark bool) {
	if canvas == nil || treeView == nil {
		return
	}
	g.ensureOverlay(canvas, treeView)
	canvas.Delete("all")

	treePath := treeView.String()
	if treePath == "" {
		return
	}
	treeHeight := tkutil.Atoi(tkutil.EvalOrEmpty("winfo height %s", treePath))
	yOffset := g.overlay.y
	contentHeight := g.overlay.h
	first := firstVisibleTreeItem(treePath, treeHeight)
	if first == "" || treeHeight <= 1 {
		return
	}

	canvasPath := canvas.String()
	if canvasPath == "" {
		return
	}
	// Prefer the Treeview column width since the overlay canvas size may lag behind `place`.
	canvasWidth := tkutil.Atoi(tkutil.EvalOrEmpty("%s column graph -width", treePath))
	if canvasWidth <= 0 {
		canvasWidth = tkutil.Atoi(tkutil.EvalOrEmpty("winfo width %s", canvasPath))
	}
	if canvasWidth <= 0 {
		canvasWidth = 120
	}
	maxCols := maxGraphCanvasCols(canvasWidth)
	if maxCols <= 0 {
		return
	}

	selected := map[string]struct{}{}
	for _, id := range treeView.Selection("") {
		selected[id] = struct{}{}
	}

	item := first
	for item != "" {
		// Use the first data column (#1). The tree column (#0) may be hidden when using `show=headings`.
		bbox := strings.Fields(tkutil.EvalOrEmpty("%s bbox {%s} #1", treePath, item))
		if len(bbox) < 4 {
			break
		}
		y := tkutil.Atoi(bbox[1]) - yOffset
		h := tkutil.Atoi(bbox[3])
		if contentHeight > 0 && y > contentHeight {
			break
		}
		_, isSelected := selected[item]
		if idx, err := strconv.Atoi(item); err == nil && idx >= 0 && idx < len(visible) {
			entry := visible[idx]
			if entry != nil {
				rowLabels := []string(nil)
				if entry.Commit != nil && labels != nil {
					rowLabels = labels[entry.Commit.Hash.String()]
				}
				drawGraphRow(canvas, dark, entry.Graph, rowLabels, y, h, maxCols, canvasWidth, isSelected)
			}
		}
		item = strings.TrimSpace(tkutil.EvalOrEmpty("%s next {%s}", treePath, item))
	}
}

func (g *GraphCanvas) ensureOverlay(canvas *CanvasWidget, treeView *TTreeviewWidget) {
	canvasPath := canvas.String()
	treePath := treeView.String()
	if canvasPath == "" || treePath == "" {
		return
	}

	bg := strings.TrimSpace(tkutil.EvalOrEmpty("ttk::style lookup Treeview -background"))
	if bg == "" {
		bg = strings.TrimSpace(tkutil.EvalOrEmpty("ttk::style lookup Treeview -fieldbackground"))
	}
	treeHeight := tkutil.Atoi(tkutil.EvalOrEmpty("winfo height %s", treePath))
	treeWidth := tkutil.Atoi(tkutil.EvalOrEmpty("winfo width %s", treePath))
	xOffset, yOffset, colWidth := graphContentCellGeometry(treePath, treeHeight)
	if colWidth <= 0 {
		colWidth = tkutil.Atoi(tkutil.EvalOrEmpty("%s column graph -width", treePath))
	}
	if colWidth <= 0 {
		colWidth = 120
	}
	if xOffset <= 0 {
		xOffset = 1
	}
	if treeWidth > 0 {
		// Leave the left and right borders visible.
		colWidth = min(colWidth, max(0, treeWidth-xOffset-1))
	}
	// Leave the bottom border visible.
	canvasHeight := max(0, treeHeight-yOffset-1)

	st := &g.overlay
	if st.ready && st.width == colWidth && st.x == xOffset && st.y == yOffset && st.h == canvasHeight && st.bg == bg {
		return
	}
	st.width = colWidth
	st.x = xOffset
	st.y = yOffset
	st.h = canvasHeight
	st.bg = bg
	if bg != "" {
		canvas.Configure(Background(bg))
	}
	// Place the overlay only over the content area, not over the header.
	tkutil.EvalOrEmpty("place %s -in %s -x %d -y %d -width %d -height %d", canvasPath, treePath, xOffset, yOffset, colWidth, canvasHeight)
	tkutil.EvalOrEmpty("raise %s", canvasPath)

	if st.ready {
		return
	}
	st.ready = true
	// Forward basic interactions from the overlay to the treeview.
	//
	// Canvas event coordinates are relative to the canvas; convert to treeview
	// coordinates using the widgets' root positions.
	tkutil.EvalOrEmpty(`
		bind %[1]s <Button-1> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <Button-1> -x $x -y $y
		}
		bind %[1]s <Double-Button-1> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <Double-Button-1> -x $x -y $y
		}
		bind %[1]s <Button-2> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <Button-2> -x $x -y $y
		}
		bind %[1]s <Button-3> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <Button-3> -x $x -y $y
		}
		bind %[1]s <MouseWheel> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <MouseWheel> -x $x -y $y -delta %%D
		}
		bind %[1]s <Button-4> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <Button-4> -x $x -y $y
		}
		bind %[1]s <Button-5> {
			set rx [winfo rootx %%W]
			set ry [winfo rooty %%W]
			set trx [winfo rootx %[2]s]
			set try [winfo rooty %[2]s]
			set x [expr {%%x + $rx - $trx}]
			set y [expr {%%y + $ry - $try}]
			focus %[2]s
			event generate %[2]s <Button-5> -x $x -y $y
		}
	`, canvasPath, treePath)
}

func firstVisibleTreeItem(treePath string, treeHeight int) string {
	if treePath == "" || treeHeight <= 1 {
		return ""
	}
	probeLimit := min(treeHeight-1, 200)
	x := 5
	for y := 1; y <= probeLimit; y++ {
		region := strings.TrimSpace(tkutil.EvalOrEmpty("%s identify region %d %d", treePath, x, y))
		switch region {
		case "cell", "tree":
		default:
			continue
		}
		item := strings.TrimSpace(tkutil.EvalOrEmpty("%s identify item %d %d", treePath, x, y))
		if item != "" {
			return item
		}
	}
	return ""
}

func graphContentCellGeometry(treePath string, treeHeight int) (xOffset int, yOffset int, width int) {
	if treePath == "" || treeHeight <= 1 {
		return 0, 0, 0
	}
	first := firstVisibleTreeItem(treePath, treeHeight)
	if first == "" {
		return 0, 0, 0
	}
	bbox := strings.Fields(tkutil.EvalOrEmpty("%s bbox {%s} #1", treePath, first))
	if len(bbox) < 4 {
		return 0, 0, 0
	}
	return tkutil.Atoi(bbox[0]), tkutil.Atoi(bbox[1]), tkutil.Atoi(bbox[2])
}

func drawGraphRow(canvas *CanvasWidget, dark bool, raw string, labels []string, yTop int, height int, maxCols int, canvasWidth int, selected bool) {
	if canvas == nil || maxCols <= 0 || height <= 0 {
		return
	}
	tokens := parseGraphTokens(raw, maxCols)
	if len(tokens) == 0 {
		return
	}
	if selected && canvasWidth > 0 {
		fill := "#cfe7ff"
		if dark {
			fill = "#253446"
		}
		canvas.CreateRectangle(
			0, yTop,
			canvasWidth, yTop+height,
			Fill(fill),
			Width(0),
		)
	}
	yMid := graphRowMidY(yTop, height)
	radius := min(graphCanvasLaneSpacing/2, max(2, height/3))

	colors := graphCanvasLaneColors(dark)
	head := containsPrefix(labels, "HEAD")
	nodeX := graphCanvasLaneMargin + graphCanvasLaneSpacing/2
	nodeColor := colors[0]
	for col, token := range tokens {
		x := graphCanvasLaneMargin + col*graphCanvasLaneSpacing + graphCanvasLaneSpacing/2
		color := colors[col%len(colors)]
		switch token {
		case "|":
			canvas.CreateLine(x, yTop, x, yTop+height, Width(graphCanvasLineWidth), Fill(color))
		case "*":
			nodeX = x
			nodeColor = color
			canvas.CreateLine(x, yTop, x, yMid-radius, Width(graphCanvasLineWidth), Fill(color))
			canvas.CreateLine(x, yMid+radius, x, yTop+height, Width(graphCanvasLineWidth), Fill(color))
			fill := "white"
			if dark {
				fill = "#1e1e1e"
			}
			if head {
				fill = "#ffd75e"
				if dark {
					fill = "#b58900"
				}
			}
			canvas.CreateOval(
				x-radius, yMid-radius,
				x+radius, yMid+radius,
				Fill(fill),
				Outline(color),
				Width(1),
			)
		default:
		}
	}
	drawGraphLabels(canvas, dark, labels, nodeX, yMid, radius, nodeColor, canvasWidth)
}

type graphLabelStyle struct {
	fill string
	out  string
	text string
}

func drawGraphLabels(canvas *CanvasWidget, dark bool, labels []string, nodeX int, yMid int, radius int, nodeColor string, canvasWidth int) {
	if canvas == nil || len(labels) == 0 || canvasWidth <= 0 {
		return
	}
	canvasPath := canvas.String()
	if canvasPath == "" {
		return
	}
	x := max(graphCanvasLabelMinX, nodeX+radius+graphCanvasLabelGap)
	connected := false
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if x >= canvasWidth-graphCanvasLabelGap {
			break
		}
		style := graphLabelStyleFor(dark, label, nodeColor)
		textID := canvas.CreateText(
			x+graphCanvasLabelPadX, yMid,
			Anchor(W),
			Txt(label),
			Font(graphCanvasLabelFont),
			Fill(style.text),
		)
		bbox := canvas.Bbox(textID)
		if len(bbox) < 4 {
			continue
		}
		x1 := tkutil.Atoi(bbox[0]) - graphCanvasLabelPadX
		y1 := tkutil.Atoi(bbox[1]) - graphCanvasLabelPadY
		x2 := tkutil.Atoi(bbox[2]) + graphCanvasLabelPadX
		y2 := tkutil.Atoi(bbox[3]) + graphCanvasLabelPadY
		if x1 >= canvasWidth {
			continue
		}
		rectID := canvas.CreateRectangle(
			x1, y1,
			min(x2, canvasWidth), y2,
			Fill(style.fill),
			Outline(style.out),
			Width(1),
		)
		tkutil.EvalOrEmpty("%s lower %s %s", canvasPath, rectID, textID)
		if !connected && x1 > nodeX+radius {
			connected = true
			canvas.CreateLine(nodeX+radius, yMid, x1, yMid, Width(graphCanvasConnectorW), Fill(style.out))
		}
		x = x2 + graphCanvasLabelGap
	}
}

func graphLabelStyleFor(dark bool, label string, nodeColor string) graphLabelStyle {
	labelLower := strings.ToLower(label)
	if strings.HasPrefix(label, "HEAD") {
		if dark {
			return graphLabelStyle{fill: "#b58900", out: "#8a6a00", text: "#111111"}
		}
		return graphLabelStyle{fill: "#ffd75e", out: "#c9a300", text: "#111111"}
	}
	if strings.HasPrefix(labelLower, "tag:") {
		if dark {
			return graphLabelStyle{fill: "#3a3a3a", out: "#6b6b6b", text: "#eaeaea"}
		}
		return graphLabelStyle{fill: "#e6e6e6", out: "#8a8a8a", text: "#111111"}
	}
	if strings.Contains(label, "/") {
		if dark {
			return graphLabelStyle{fill: "#253446", out: "#4fa3ff", text: "#eaeaea"}
		}
		return graphLabelStyle{fill: "#dbeafe", out: "#2563eb", text: "#111111"}
	}
	text := "#111111"
	fill := "#dff5de"
	if dark {
		text = "#eaeaea"
		fill = "#1f3b2a"
	}
	return graphLabelStyle{fill: fill, out: nodeColor, text: text}
}

func containsPrefix(values []string, prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func graphRowMidY(yTop int, height int) int {
	if height <= 0 {
		return yTop
	}
	return yTop + (height-1)/2
}

func maxGraphCanvasCols(canvasWidth int) int {
	avail := canvasWidth - 2*graphCanvasLaneMargin
	if avail <= 0 {
		return 0
	}
	return max(1, avail/graphCanvasLaneSpacing)
}

func graphCanvasLaneColors(dark bool) []string {
	// Based on gitk's default colors; keep a small, high-contrast palette.
	if dark {
		return []string{"#00ff00", "#ff5c5c", "#4fa3ff", "#d56bff", "#a0a0a0", "#d09a6b", "#ffb347"}
	}
	return []string{"#00cc00", "#cc0000", "#0055cc", "#aa00aa", "#555555", "#8b4513", "#ff8c00"}
}
