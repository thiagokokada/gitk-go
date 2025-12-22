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

	// Treeview "identify item" takes x/y coordinates; using a small x offset avoids
	// the left border and hits the first visible cell reliably.
	defaultTreeIdentifyX = 5

	// Fallback width (pixels) for the "graph" column/overlay canvas when Tk hasn't
	// reported geometry yet. It only affects the initial draw and any rare cases
	// where `winfo`/`column -width` returns 0.
	defaultGraphColumnWidth = 120

	// Bound the amount of probing/advancing we do during redraw. These guards prevent
	// rare-but-expensive scans when the Treeview is very large or its items behave
	// unexpectedly.
	maxTreeIdentifyProbeRows = 200
	maxNonCommitRowSkips     = 8
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

type graphDrawContext struct {
	canvas      *CanvasWidget
	dark        bool
	canvasWidth int
	maxCols     int
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

func (g *GraphCanvas) Redraw(
	canvas *CanvasWidget,
	treeView *TTreeviewWidget,
	visible []*git.Entry,
	labels map[string][]string,
	dark bool,
) {
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
	first := firstVisibleTreeItemForRedraw(treePath, max(1, g.overlay.x+1), yOffset, treeHeight)
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
		canvasWidth = defaultGraphColumnWidth
	}
	maxCols := maxGraphCanvasCols(canvasWidth)
	if maxCols <= 0 {
		return
	}
	ctx := graphDrawContext{
		canvas:      canvas,
		dark:        dark,
		canvasWidth: canvasWidth,
		maxCols:     maxCols,
	}

	selectedIdx := -1
	if sel := treeView.Selection(""); len(sel) > 0 {
		if idx, err := strconv.Atoi(sel[0]); err == nil && idx >= 0 {
			selectedIdx = idx
		}
	}

	// Treeview items can include non-commit rows (local changes, "more...", loading); resolve the
	// first visible commit row and account for any leading non-numeric rows.
	bbox := strings.Fields(tkutil.EvalOrEmpty("%s bbox {%s} #1", treePath, first))
	if len(bbox) < 4 {
		return
	}
	firstRowY := tkutil.Atoi(bbox[1]) - yOffset
	rowHeight := tkutil.Atoi(bbox[3])
	if rowHeight <= 0 {
		return
	}
	firstIdx, skippedRows, ok := resolveFirstCommitIndex(first, func(item string) string {
		return strings.TrimSpace(tkutil.EvalOrEmpty("%s next {%s}", treePath, item))
	})
	if !ok || firstIdx >= len(visible) {
		return
	}
	y := firstRowY + skippedRows*rowHeight
	for idx := firstIdx; idx < len(visible); idx++ {
		if contentHeight > 0 && y > contentHeight {
			break
		}
		entry := visible[idx]
		if entry != nil {
			rowLabels := []string(nil)
			if entry.Commit != nil && labels != nil {
				rowLabels = labels[entry.Commit.Hash]
			}
			ctx.drawRow(entry.Graph, rowLabels, y, rowHeight, idx == selectedIdx)
		}
		y += rowHeight
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

	colWidth := tkutil.Atoi(tkutil.EvalOrEmpty("%s column graph -width", treePath))
	xOffset := g.overlay.x
	yOffset := g.overlay.y
	if xOffset <= 0 || yOffset <= 0 {
		xOffset, yOffset, colWidth = graphContentCellGeometry(treePath, treeHeight)
	} else if colWidth <= 0 {
		// Fall back to a cached width if the Treeview hasn't been configured yet.
		colWidth = g.overlay.width
	}
	if colWidth <= 0 {
		colWidth = defaultGraphColumnWidth
	}
	if xOffset <= 0 {
		xOffset = 1
	}
	if yOffset <= 0 {
		// No items yet; avoid covering the header until we can measure the content area.
		return
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
	tkutil.EvalOrEmpty(
		"place %s -in %s -x %d -y %d -width %d -height %d",
		canvasPath,
		treePath,
		xOffset,
		yOffset,
		colWidth,
		canvasHeight,
	)
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
	probeLimit := min(treeHeight-1, maxTreeIdentifyProbeRows)
	x := defaultTreeIdentifyX
	for y := 1; y <= probeLimit; y++ {
		item := strings.TrimSpace(tkutil.EvalOrEmpty("%s identify item %d %d", treePath, x, y))
		if item != "" {
			return item
		}
	}
	return ""
}

func firstVisibleTreeItemForRedraw(treePath string, xProbe int, yOffset int, treeHeight int) string {
	if treePath == "" || treeHeight <= 1 {
		return ""
	}
	if xProbe <= 0 {
		xProbe = defaultTreeIdentifyX
	}
	y := yOffset + 1
	if y < 1 {
		y = 1
	}
	if y >= treeHeight {
		y = treeHeight - 1
	}
	item := strings.TrimSpace(tkutil.EvalOrEmpty("%s identify item %d %d", treePath, xProbe, y))
	if item != "" {
		return item
	}
	return firstVisibleTreeItem(treePath, treeHeight)
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

func resolveFirstCommitIndex(firstItem string, next func(string) string) (idx int, skipped int, ok bool) {
	item := strings.TrimSpace(firstItem)
	for item != "" && skipped <= maxNonCommitRowSkips {
		parsed, err := strconv.Atoi(item)
		if err == nil && parsed >= 0 {
			return parsed, skipped, true
		}
		if next == nil {
			break
		}
		item = strings.TrimSpace(next(item))
		skipped++
	}
	return 0, skipped, false
}

type graphLabelStyle struct {
	fill string
	out  string
	text string
}

func (ctx graphDrawContext) drawRow(
	raw string,
	labels []string,
	yTop int,
	height int,
	selected bool,
) {
	if ctx.canvas == nil || ctx.maxCols <= 0 || height <= 0 {
		return
	}
	tokens := parseGraphTokens(raw, ctx.maxCols)
	if len(tokens) == 0 {
		return
	}
	if selected && ctx.canvasWidth > 0 {
		fill := "#cfe7ff"
		if ctx.dark {
			fill = "#253446"
		}
		ctx.canvas.CreateRectangle(
			0, yTop,
			ctx.canvasWidth, yTop+height,
			Fill(fill),
			Width(0),
		)
	}
	yMid := graphRowMidY(yTop, height)
	radius := min(graphCanvasLaneSpacing/2, max(2, height/3))

	colors := graphCanvasLaneColors(ctx.dark)
	head := containsPrefix(labels, "HEAD")
	nodeX := graphCanvasLaneMargin + graphCanvasLaneSpacing/2
	nodeColor := colors[0]
	for col, token := range tokens {
		x := graphCanvasLaneMargin + col*graphCanvasLaneSpacing + graphCanvasLaneSpacing/2
		color := colors[col%len(colors)]
		switch token {
		case "|":
			ctx.canvas.CreateLine(x, yTop, x, yTop+height, Width(graphCanvasLineWidth), Fill(color))
		case "*":
			nodeX = x
			nodeColor = color
			ctx.canvas.CreateLine(x, yTop, x, yMid-radius, Width(graphCanvasLineWidth), Fill(color))
			ctx.canvas.CreateLine(x, yMid+radius, x, yTop+height, Width(graphCanvasLineWidth), Fill(color))
			fill := "white"
			if ctx.dark {
				fill = "#1e1e1e"
			}
			if head {
				fill = "#ffd75e"
				if ctx.dark {
					fill = "#b58900"
				}
			}
			ctx.canvas.CreateOval(
				x-radius, yMid-radius,
				x+radius, yMid+radius,
				Fill(fill),
				Outline(color),
				Width(1),
			)
		default:
		}
	}
	ctx.drawLabels(labels, nodeX, yMid, radius, nodeColor)
}

func (ctx graphDrawContext) drawLabels(
	labels []string,
	nodeX int,
	yMid int,
	radius int,
	nodeColor string,
) {
	if ctx.canvas == nil || len(labels) == 0 || ctx.canvasWidth <= 0 {
		return
	}
	canvasPath := ctx.canvas.String()
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
		if x >= ctx.canvasWidth-graphCanvasLabelGap {
			break
		}
		style := graphLabelStyleFor(ctx.dark, label, nodeColor)
		textID := ctx.canvas.CreateText(
			x+graphCanvasLabelPadX, yMid,
			Anchor(W),
			Txt(label),
			Font(graphCanvasLabelFont),
			Fill(style.text),
		)
		bbox := ctx.canvas.Bbox(textID)
		if len(bbox) < 4 {
			continue
		}
		x1 := tkutil.Atoi(bbox[0]) - graphCanvasLabelPadX
		y1 := tkutil.Atoi(bbox[1]) - graphCanvasLabelPadY
		x2 := tkutil.Atoi(bbox[2]) + graphCanvasLabelPadX
		y2 := tkutil.Atoi(bbox[3]) + graphCanvasLabelPadY
		if x1 >= ctx.canvasWidth {
			continue
		}
		rectID := ctx.canvas.CreateRectangle(
			x1, y1,
			min(x2, ctx.canvasWidth), y2,
			Fill(style.fill),
			Outline(style.out),
			Width(1),
		)
		tkutil.EvalOrEmpty("%s lower %s %s", canvasPath, rectID, textID)
		if !connected && x1 > nodeX+radius {
			connected = true
			ctx.canvas.CreateLine(nodeX+radius, yMid, x1, yMid, Width(graphCanvasConnectorW), Fill(style.out))
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
