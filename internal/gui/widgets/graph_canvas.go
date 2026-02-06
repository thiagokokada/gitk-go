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

	graphCanvasLabelPadX  = 2
	graphCanvasLabelPadY  = 0
	graphCanvasLabelGap   = 2
	graphCanvasLabelMinX  = 2
	graphCanvasConnectorW = 1

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
	handlers      GraphCanvasHandlers
	handlersBound bool
}

type GraphCanvasLabelStyle struct {
	Fill    string
	Outline string
	Text    string
}

type GraphCanvasTheme struct {
	LaneColors      []string
	SelectedRowFill string
	NodeFill        string
	HeadNodeFill    string
	HeadLabel       GraphCanvasLabelStyle
	TagLabel        GraphCanvasLabelStyle
	BranchLabel     GraphCanvasLabelStyle
	DefaultLabel    GraphCanvasLabelStyle
}

type GraphCanvasDrawInput struct {
	Visible   []*git.Entry
	Labels    map[string][]string
	Theme     GraphCanvasTheme
	IndexByID map[string]int
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
	theme       GraphCanvasTheme
	canvasWidth int
	maxCols     int
}

type GraphCanvasHandlers struct {
	OnClick       func(x, y int)
	OnDoubleClick func(x, y int)
	OnContextMenu func(x, y int, xRoot, yRoot int)
	OnWheel       func(delta int)
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

func (g *GraphCanvas) SetHandlers(handlers GraphCanvasHandlers) {
	g.handlers = handlers
	if g.overlay.ready {
		g.bindOverlayHandlers()
	}
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
	if len(input.Theme.LaneColors) == 0 {
		return
	}
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
	treeHeight := tkutil.Atoi(WinfoHeight(g.treeView.Window))
	yOffset := g.overlay.y
	contentHeight := g.overlay.h
	first := firstVisibleTreeItemForRedraw(g.treeView, max(1, g.overlay.x+1), yOffset, treeHeight)
	if first == "" {
		return graphCanvasDrawPlan{}, false
	}

	// Prefer the Treeview column width since the overlay canvas size may lag behind `place`.
	canvasWidth := treeviewColumnWidth(g.treeView, "graph")
	if canvasWidth <= 0 {
		canvasWidth = tkutil.Atoi(WinfoWidth(g.canvas.Window))
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
		theme:       input.Theme,
		canvasWidth: canvasWidth,
		maxCols:     maxCols,
	}

	selectedIdx := -1
	if sel := g.treeView.Selection(""); len(sel) > 0 {
		if idx, ok := indexForTreeItem(sel[0], input.IndexByID); ok && idx >= 0 {
			selectedIdx = idx
		}
	}

	// Treeview items can include non-commit rows (local changes, "more...", loading); resolve the
	// first visible commit row and account for any leading non-commit rows.
	bbox := strings.Fields(tkutil.EvalfOrEmpty("%s bbox {%s} #1", treePath, first))
	if len(bbox) < 4 {
		return graphCanvasDrawPlan{}, false
	}
	firstRowY := tkutil.Atoi(bbox[1]) - yOffset
	rowHeight := tkutil.Atoi(bbox[3])
	if rowHeight <= 0 {
		return graphCanvasDrawPlan{}, false
	}
	indexForItem := func(item string) (int, bool) {
		return indexForTreeItem(item, input.IndexByID)
	}
	firstIdx, skippedRows, ok := resolveFirstCommitIndex(first, indexForItem, func(item string) string {
		return strings.TrimSpace(tkutil.EvalfOrEmpty("%s next {%s}", treePath, item))
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
	treePath := g.treePath

	bg := strings.TrimSpace(StyleLookup("Treeview", Background))
	if bg == "" {
		bg = strings.TrimSpace(StyleLookup("Treeview", Fieldbackground))
	}
	treeHeight := tkutil.Atoi(WinfoHeight(g.treeView.Window))
	treeWidth := tkutil.Atoi(WinfoWidth(g.treeView.Window))

	colWidth := treeviewColumnWidth(g.treeView, "graph")
	xOffset := g.overlay.x
	yOffset := g.overlay.y
	if xOffset <= 0 || yOffset <= 0 {
		xOffset, yOffset, colWidth = graphContentCellGeometry(treePath, g.treeView, treeHeight)
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
	Place(
		g.canvas,
		In(g.treeView),
		X(xOffset),
		Y(yOffset),
		Width(colWidth),
		Height(canvasHeight),
	)
	g.canvas.Raise(nil)

	if st.ready {
		return
	}
	st.ready = true
	g.bindOverlayHandlers()
}

func (g *GraphCanvas) bindOverlayHandlers() {
	if g.handlersBound {
		return
	}
	g.handlersBound = true
	Bind(g.canvas, "<Button-1>", Command(func(e *Event) {
		g.handleOverlayPointEvent(e, g.handlers.OnClick)
	}))
	Bind(g.canvas, "<Double-Button-1>", Command(func(e *Event) {
		g.handleOverlayPointEvent(e, g.handlers.OnDoubleClick)
	}))
	Bind(g.canvas, "<Button-2>", Command(func(e *Event) {
		g.handleOverlayContextEvent(e)
	}))
	Bind(g.canvas, "<Button-3>", Command(func(e *Event) {
		g.handleOverlayContextEvent(e)
	}))
	Bind(g.canvas, "<MouseWheel>", Command(func(e *Event) {
		if g.handlers.OnWheel != nil {
			g.handlers.OnWheel(e.Delta)
		}
	}))
	Bind(g.canvas, "<Button-4>", Command(func(e *Event) {
		if g.handlers.OnWheel != nil {
			g.handlers.OnWheel(120)
		}
	}))
	Bind(g.canvas, "<Button-5>", Command(func(e *Event) {
		if g.handlers.OnWheel != nil {
			g.handlers.OnWheel(-120)
		}
	}))
}

func (g *GraphCanvas) handleOverlayPointEvent(e *Event, handler func(x, y int)) {
	if handler == nil || e == nil {
		return
	}
	x, y, ok := treeCoordsForOverlay(g.overlay, e.X, e.Y)
	if !ok {
		return
	}
	handler(x, y)
}

func (g *GraphCanvas) handleOverlayContextEvent(e *Event) {
	if e == nil || g.handlers.OnContextMenu == nil {
		return
	}
	x, y, ok := treeCoordsForOverlay(g.overlay, e.X, e.Y)
	if !ok {
		return
	}
	g.handlers.OnContextMenu(x, y, e.XRoot, e.YRoot)
}

func treeCoordsForOverlay(overlay graphOverlayState, x, y int) (resX, resY int, ok bool) {
	if !overlay.ready {
		return 0, 0, false
	}
	return x + overlay.x, y + overlay.y, true
}

func firstVisibleTreeItem(treeView *TTreeviewWidget, treeHeight int) string {
	if treeView == nil || treeHeight <= 1 {
		return ""
	}
	if treeView.String() == "" {
		return ""
	}
	probeLimit := min(treeHeight-1, maxTreeIdentifyProbeRows)
	x := defaultTreeIdentifyX
	for y := 1; y <= probeLimit; y++ {
		item := strings.TrimSpace(treeView.IdentifyItem(x, y))
		if item != "" {
			return item
		}
	}
	return ""
}

func firstVisibleTreeItemForRedraw(treeView *TTreeviewWidget, xProbe int, yOffset int, treeHeight int) string {
	if treeView == nil || treeHeight <= 1 {
		return ""
	}
	if treeView.String() == "" {
		return ""
	}
	if xProbe <= 0 {
		xProbe = defaultTreeIdentifyX
	}
	y := max(yOffset+1, 1)
	if y >= treeHeight {
		y = treeHeight - 1
	}
	item := strings.TrimSpace(treeView.IdentifyItem(xProbe, y))
	if item != "" {
		return item
	}
	return firstVisibleTreeItem(treeView, treeHeight)
}

func graphContentCellGeometry(
	treePath string,
	treeView *TTreeviewWidget,
	treeHeight int,
) (xOffset int, yOffset int, width int) {
	if treePath == "" || treeView == nil || treeHeight <= 1 {
		return 0, 0, 0
	}
	first := firstVisibleTreeItem(treeView, treeHeight)
	if first == "" {
		return 0, 0, 0
	}
	bbox := strings.Fields(tkutil.EvalfOrEmpty("%s bbox {%s} #1", treePath, first))
	if len(bbox) < 4 {
		return 0, 0, 0
	}
	return tkutil.Atoi(bbox[0]), tkutil.Atoi(bbox[1]), tkutil.Atoi(bbox[2])
}

func treeviewColumnWidth(treeView *TTreeviewWidget, column string) int {
	if treeView == nil || treeView.String() == "" || column == "" {
		return 0
	}
	fields := strings.Fields(treeView.Column(column))
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "-width" {
			return tkutil.Atoi(strings.Trim(fields[i+1], "{}"))
		}
	}
	return 0
}

func resolveFirstCommitIndex(
	firstItem string,
	indexForItem func(string) (int, bool),
	next func(string) string,
) (idx int, skipped int, ok bool) {
	item := strings.TrimSpace(firstItem)
	for item != "" && skipped <= maxNonCommitRowSkips {
		if indexForItem != nil {
			if idx, ok := indexForItem(item); ok {
				return idx, skipped, true
			}
		} else if parsed, err := strconv.Atoi(item); err == nil && parsed >= 0 {
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

func indexForTreeItem(id string, indexByID map[string]int) (int, bool) {
	id = strings.TrimSpace(id)
	if id == "" || indexByID == nil {
		return 0, false
	}
	idx, ok := indexByID[id]
	return idx, ok
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
		fill := g.draw.theme.SelectedRowFill
		g.draw.canvas.CreateRectangle(
			0, yTop,
			g.draw.canvasWidth, yTop+height,
			Fill(fill),
			Width(0),
		)
	}
	yMid := graphRowMidY(yTop, height)
	radius := min(graphCanvasLaneSpacing/2, max(2, height/3))

	colors := g.draw.theme.LaneColors
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
			fill := g.draw.theme.NodeFill
			if head {
				fill = g.draw.theme.HeadNodeFill
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
		style := graphLabelStyleFor(g.draw.theme, label, nodeColor)
		textID := g.draw.canvas.CreateText(
			x+graphCanvasLabelPadX, yMid,
			Anchor(W),
			Txt(label),
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
		tkutil.EvalfOrEmpty("%s lower %s %s", canvasPath, rectID, textID)
		if !connected && x1 > nodeX+radius {
			connected = true
			g.draw.canvas.CreateLine(nodeX+radius, yMid, x1, yMid, Width(graphCanvasConnectorW), Fill(style.out))
		}
		x = x2 + graphCanvasLabelGap
	}
}

func graphLabelStyleFor(theme GraphCanvasTheme, label string, nodeColor string) graphLabelStyle {
	labelLower := strings.ToLower(label)
	if strings.HasPrefix(label, "HEAD") {
		return graphLabelStyle{
			fill: theme.HeadLabel.Fill,
			out:  theme.HeadLabel.Outline,
			text: theme.HeadLabel.Text,
		}
	}
	if strings.HasPrefix(labelLower, "tag:") {
		return graphLabelStyle{
			fill: theme.TagLabel.Fill,
			out:  theme.TagLabel.Outline,
			text: theme.TagLabel.Text,
		}
	}
	if strings.Contains(label, "/") {
		return graphLabelStyle{
			fill: theme.BranchLabel.Fill,
			out:  theme.BranchLabel.Outline,
			text: theme.BranchLabel.Text,
		}
	}
	style := graphLabelStyle{
		fill: theme.DefaultLabel.Fill,
		out:  theme.DefaultLabel.Outline,
		text: theme.DefaultLabel.Text,
	}
	if style.out == "" {
		style.out = nodeColor
	}
	return style
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
