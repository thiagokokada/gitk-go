package gui

import "testing"

func TestShouldLoadMoreOnScroll(t *testing.T) {
	tests := []struct {
		name       string
		tree       treeState
		filter     string
		visibleLen int
		batch      int
		start      float64
		end        float64
		want       bool
	}{
		{
			name: "no more commits",
			tree: treeState{hasMore: false},
			want: false,
		},
		{
			name: "loading batch",
			tree: treeState{hasMore: true, loadingBatch: true},
			want: false,
		},
		{
			name:       "empty visible list loads",
			tree:       treeState{hasMore: true},
			visibleLen: 0,
			want:       true,
		},
		{
			name:       "full view with empty filter does not autoload",
			tree:       treeState{hasMore: true},
			filter:     "",
			visibleLen: 200,
			batch:      200,
			start:      0,
			end:        1,
			want:       false,
		},
		{
			name:       "near bottom triggers load",
			tree:       treeState{hasMore: true},
			filter:     "abc",
			visibleLen: 10,
			batch:      200,
			start:      0.5,
			end:        autoLoadThreshold,
			want:       true,
		},
		{
			name:       "not near bottom does not load",
			tree:       treeState{hasMore: true},
			filter:     "abc",
			visibleLen: 10,
			batch:      200,
			start:      0.1,
			end:        autoLoadThreshold - 0.01,
			want:       false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tree.shouldLoadMoreOnScroll(tc.filter, tc.visibleLen, tc.batch, tc.start, tc.end); got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}
