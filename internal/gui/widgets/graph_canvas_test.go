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

func TestResolveFirstCommitIndex(t *testing.T) {
	t.Run("numeric-fallback", func(t *testing.T) {
		idx, skipped, ok := resolveFirstCommitIndex("10", nil, func(string) string { return "" })
		if !ok || idx != 10 || skipped != 0 {
			t.Fatalf("expected ok idx=10 skipped=0, got ok=%v idx=%d skipped=%d", ok, idx, skipped)
		}
	})

	t.Run("mapped", func(t *testing.T) {
		index := func(item string) (int, bool) {
			if item == "hash" {
				return 12, true
			}
			return 0, false
		}
		idx, skipped, ok := resolveFirstCommitIndex("hash", index, func(string) string { return "" })
		if !ok || idx != 12 || skipped != 0 {
			t.Fatalf("expected ok idx=12 skipped=0, got ok=%v idx=%d skipped=%d", ok, idx, skipped)
		}
	})

	t.Run("skip-local-rows", func(t *testing.T) {
		index := func(item string) (int, bool) {
			if item == "hash" {
				return 0, true
			}
			return 0, false
		}
		next := func(item string) string {
			switch item {
			case "localUnstagedRow":
				return "localStagedRow"
			case "localStagedRow":
				return "hash"
			default:
				return ""
			}
		}
		idx, skipped, ok := resolveFirstCommitIndex("localUnstagedRow", index, next)
		if !ok || idx != 0 || skipped != 2 {
			t.Fatalf("expected ok idx=0 skipped=2, got ok=%v idx=%d skipped=%d", ok, idx, skipped)
		}
	})

	t.Run("no-commit-found", func(t *testing.T) {
		index := func(string) (int, bool) { return 0, false }
		idx, skipped, ok := resolveFirstCommitIndex("moreIndicatorID", index, func(string) string { return "" })
		if ok {
			t.Fatalf("expected ok=false, got ok=true idx=%d skipped=%d", idx, skipped)
		}
	})

	t.Run("max-skip-limit", func(t *testing.T) {
		index := func(string) (int, bool) { return 0, false }
		idx, skipped, ok := resolveFirstCommitIndex("x", index, func(string) string { return "x" })
		if ok {
			t.Fatalf("expected ok=false, got ok=true idx=%d skipped=%d", idx, skipped)
		}
		if skipped != maxNonCommitRowSkips+1 {
			t.Fatalf("expected skipped=%d, got %d", maxNonCommitRowSkips+1, skipped)
		}
	})
}
