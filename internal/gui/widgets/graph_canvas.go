package widgets

import (
	"fmt"
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
	canvas        *CanvasWidget
	treeView      *TTreeviewWidget
	canvasPath    string
	treePath      string
	input         GraphCanvasDrawInput
	draw          graphCanvasDrawState
}

type GraphCanvasDrawInput struct {
	Visible []*git.Entry
	Labels  map[string][]string
	Dark    bool
}

type graphOverlayState struct {
	ready bool
	width int
	x     int
	y     int
	h     int
	bg    string
}

type graphCanvasDrawState struct {
	canvas      *CanvasWidget
	dark        bool
	canvasWidth int
	maxCols     int
}

type graphCanvasDrawPlan struct {
	contentHeight int
	rowHeight     int
	startY        int
	firstIdx      int
	selectedIdx   int
	visible       []*git.Entry
	labels        map[string][]string
}

func NewGraphCanvas(canvas *CanvasWidget, treeView *TTreeviewWidget) (*GraphCanvas, error) {
	if canvas == nil || treeView == nil {
		return nil, fmt.Errorf("graph canvas: canvas and treeview must be set")
	}
	canvasPath := canvas.String()
	if canvasPath == "" {
		return nil, fmt.Errorf("graph canvas: canvas path is empty")
	}
	treePath := treeView.String()
	if treePath == "" {
		return nil, fmt.Errorf("graph canvas: treeview path is empty")
	}
	return &GraphCanvas{
		canvas:     canvas,
		treeView:   treeView,
		canvasPath: canvasPath,
		treePath:   treePath,
	}, nil
}

func (g *GraphCanvas) ScheduleDraw(redraw func()) {
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

func (g *GraphCanvas) Draw(input GraphCanvasDrawInput) {
	g.input = input
	plan, ok := g.planGraphCanvasDraw()
	if !ok {
		return
	}
	y := plan.startY
	for idx := plan.firstIdx; idx < len(plan.visible); idx++ {
		if plan.contentHeight > 0 && y > plan.contentHeight {
			break
		}
		entry := plan.visible[idx]
		if entry != nil {
			rowLabels := []string(nil)
			if entry.Commit != nil && plan.labels != nil {
				rowLabels = plan.labels[entry.Commit.Hash]
			}
			g.drawGraphRow(entry.Graph, rowLabels, y, plan.rowHeight, idx == plan.selectedIdx)
		}
		y += plan.rowHeight
	}
}

func (g *GraphCanvas) planGraphCanvasDraw() (graphCanvasDrawPlan, bool) {
	input := g.input
	g.ensureOverlay()
	g.canvas.Delete("all")

	treePath := g.treePath
	treeHeight := tkutil.Atoi(tkutil.EvalOrEmpty("winfo height %s", treePath))
	yOffset := g.overlay.y
	contentHeight := g.overlay.h
	first := firstVisibleTreeItemForRedraw(treePath, max(1, g.overlay.x+1), yOffset, treeHeight)
	if first == "" {
		return graphCanvasDrawPlan{}, false
	}

	canvasPath := g.canvasPath
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
		return graphCanvasDrawPlan{}, false
	}
	g.draw = graphCanvasDrawState{
		canvas:      g.canvas,
		dark:        input.Dark,
		canvasWidth: canvasWidth,
		maxCols:     maxCols,
	}

	selectedIdx := -1
	if sel := g.treeView.Selection(""); len(sel) > 0 {
		if idx, err := strconv.Atoi(sel[0]); err == nil && idx >= 0 {
			selectedIdx = idx
		}
	}

	// Treeview items can include non-commit rows (local changes, "more...", loading); resolve the
	// first visible commit row and account for any leading non-numeric rows.
	bbox := strings.Fields(tkutil.EvalOrEmpty("%s bbox {%s} #1", treePath, first))
	if len(bbox) < 4 {
		return graphCanvasDrawPlan{}, false
	}
	firstRowY := tkutil.Atoi(bbox[1]) - yOffset
	rowHeight := tkutil.Atoi(bbox[3])
	if rowHeight <= 0 {
		return graphCanvasDrawPlan{}, false
	}
	firstIdx, skippedRows, ok := resolveFirstCommitIndex(first, func(item string) string {
		return strings.TrimSpace(tkutil.EvalOrEmpty("%s next {%s}", treePath, item))
	})
	if !ok || firstIdx >= len(input.Visible) {
		return graphCanvasDrawPlan{}, false
	}

	return graphCanvasDrawPlan{
		contentHeight: contentHeight,
		rowHeight:     rowHeight,
		startY:        firstRowY + skippedRows*rowHeight,
		firstIdx:      firstIdx,
		selectedIdx:   selectedIdx,
		visible:       input.Visible,
		labels:        input.Labels,
	}, true
}

func (g *GraphCanvas) ensureOverlay() {
	canvas := g.canvas
	canvasPath := g.canvasPath
	treePath := g.treePath

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

func (g *GraphCanvas) drawGraphRow(
	raw string,
	labels []string,
	yTop int,
	height int,
	selected bool,
) {
	tokens := parseGraphTokens(raw, g.draw.maxCols)
	if len(tokens) == 0 {
		return
	}
	if selected {
		fill := "#cfe7ff"
		if g.draw.dark {
			fill = "#253446"
		}
		g.draw.canvas.CreateRectangle(
			0, yTop,
			g.draw.canvasWidth, yTop+height,
			Fill(fill),
			Width(0),
		)
	}
	yMid := graphRowMidY(yTop, height)
	radius := min(graphCanvasLaneSpacing/2, max(2, height/3))

	colors := graphCanvasLaneColors(g.draw.dark)
	head := containsPrefix(labels, "HEAD")
	nodeX := graphCanvasLaneMargin + graphCanvasLaneSpacing/2
	nodeColor := colors[0]
	for col, token := range tokens {
		x := graphCanvasLaneMargin + col*graphCanvasLaneSpacing + graphCanvasLaneSpacing/2
		color := colors[col%len(colors)]
		switch token {
		case "|":
			g.draw.canvas.CreateLine(x, yTop, x, yTop+height, Width(graphCanvasLineWidth), Fill(color))
		case "*":
			nodeX = x
			nodeColor = color
			g.draw.canvas.CreateLine(x, yTop, x, yMid-radius, Width(graphCanvasLineWidth), Fill(color))
			g.draw.canvas.CreateLine(x, yMid+radius, x, yTop+height, Width(graphCanvasLineWidth), Fill(color))
			fill := "white"
			if g.draw.dark {
				fill = "#1e1e1e"
			}
			if head {
				fill = "#ffd75e"
				if g.draw.dark {
					fill = "#b58900"
				}
			}
			g.draw.canvas.CreateOval(
				x-radius, yMid-radius,
				x+radius, yMid+radius,
				Fill(fill),
				Outline(color),
				Width(1),
			)
		default:
		}
	}
	g.drawGraphLabels(labels, nodeX, yMid, radius, nodeColor)
}

func (g *GraphCanvas) drawGraphLabels(
	labels []string,
	nodeX int,
	yMid int,
	radius int,
	nodeColor string,
) {
	if len(labels) == 0 {
		return
	}
	canvasPath := g.draw.canvas.String()
	x := max(graphCanvasLabelMinX, nodeX+radius+graphCanvasLabelGap)
	connected := false
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if x >= g.draw.canvasWidth-graphCanvasLabelGap {
			break
		}
		style := graphLabelStyleFor(g.draw.dark, label, nodeColor)
		textID := g.draw.canvas.CreateText(
			x+graphCanvasLabelPadX, yMid,
			Anchor(W),
			Txt(label),
			Font(graphCanvasLabelFont),
			Fill(style.text),
		)
		bbox := g.draw.canvas.Bbox(textID)
		if len(bbox) < 4 {
			continue
		}
		x1 := tkutil.Atoi(bbox[0]) - graphCanvasLabelPadX
		y1 := tkutil.Atoi(bbox[1]) - graphCanvasLabelPadY
		x2 := tkutil.Atoi(bbox[2]) + graphCanvasLabelPadX
		y2 := tkutil.Atoi(bbox[3]) + graphCanvasLabelPadY
		if x1 >= g.draw.canvasWidth {
			continue
		}
		rectID := g.draw.canvas.CreateRectangle(
			x1, y1,
			min(x2, g.draw.canvasWidth), y2,
			Fill(style.fill),
			Outline(style.out),
			Width(1),
		)
		tkutil.EvalOrEmpty("%s lower %s %s", canvasPath, rectID, textID)
		if !connected && x1 > nodeX+radius {
			connected = true
			g.draw.canvas.CreateLine(nodeX+radius, yMid, x1, yMid, Width(graphCanvasConnectorW), Fill(style.out))
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
