package gui

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
