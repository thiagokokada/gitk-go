package widgets

import "testing"

func TestMaxGraphCanvasCols(t *testing.T) {
	if got := maxGraphCanvasCols(0); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := maxGraphCanvasCols(2 * graphCanvasLaneMargin); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := maxGraphCanvasCols(2*graphCanvasLaneMargin + graphCanvasLaneSpacing); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestGraphRowMidY(t *testing.T) {
	if got := graphRowMidY(10, 0); got != 10 {
		t.Fatalf("expected yTop for empty height, got %d", got)
	}
	if got := graphRowMidY(10, 21); got != 20 {
		t.Fatalf("expected yTop+10 for odd height, got %d", got)
	}
	if got := graphRowMidY(10, 20); got != 19 {
		t.Fatalf("expected yTop+9 for even height, got %d", got)
	}
}
