package widgets

import "testing"

func TestTreeCoordsForOverlay(t *testing.T) {
	overlay := graphOverlayState{}
	if _, _, ok := treeCoordsForOverlay(overlay, 1, 2); ok {
		t.Fatalf("expected not ready overlay to return ok=false")
	}
	overlay.ready = true
	overlay.x = 10
	overlay.y = 5
	x, y, ok := treeCoordsForOverlay(overlay, 3, 4)
	if !ok {
		t.Fatalf("expected ready overlay to return ok=true")
	}
	if x != 13 || y != 9 {
		t.Fatalf("unexpected coords: got (%d,%d), want (13,9)", x, y)
	}
}
