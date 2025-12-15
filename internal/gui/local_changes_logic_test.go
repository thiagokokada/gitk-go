package gui

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestLocalChangePlanRepoNotReady(t *testing.T) {
	status := git.LocalChanges{HasWorktree: true, HasStaged: true}
	actions := (treeState{showLocalUnstaged: true, showLocalStaged: true}).localChangePlan(false, true, status)
	if actions.showUnstaged || actions.showStaged {
		t.Fatalf("expected no rows shown when repo is not ready, got %+v", actions)
	}
	if !actions.resetUnstaged || !actions.resetStaged {
		t.Fatalf("expected both states reset when repo is not ready, got %+v", actions)
	}
	if actions.loadUnstaged || actions.loadStaged {
		t.Fatalf("expected no loads when repo is not ready, got %+v", actions)
	}
}

func TestLocalChangePlanPrefetch(t *testing.T) {
	status := git.LocalChanges{HasWorktree: true, HasStaged: false}
	actions := (treeState{showLocalUnstaged: false, showLocalStaged: true}).localChangePlan(true, true, status)
	if !actions.showUnstaged || actions.showStaged {
		t.Fatalf("unexpected show flags: %+v", actions)
	}
	if !actions.loadUnstaged || actions.loadStaged {
		t.Fatalf("unexpected load flags: %+v", actions)
	}
	if actions.resetUnstaged || !actions.resetStaged {
		t.Fatalf("unexpected reset flags: %+v", actions)
	}
}

func TestLocalChangePlanNoPrefetchLoadsOnTransition(t *testing.T) {
	tests := []struct {
		name         string
		status       git.LocalChanges
		prevUnstaged bool
		prevStaged   bool
		want         localChangeActions
	}{
		{
			name:         "worktree becomes visible triggers load",
			status:       git.LocalChanges{HasWorktree: true, HasStaged: false},
			prevUnstaged: false,
			prevStaged:   false,
			want: localChangeActions{
				showUnstaged: true,
				showStaged:   false,
				loadUnstaged: true,
				resetStaged:  true,
			},
		},
		{
			name:         "already visible does not reload",
			status:       git.LocalChanges{HasWorktree: true, HasStaged: true},
			prevUnstaged: true,
			prevStaged:   true,
			want: localChangeActions{
				showUnstaged: true,
				showStaged:   true,
			},
		},
		{
			name:         "staged becomes visible triggers load",
			status:       git.LocalChanges{HasWorktree: false, HasStaged: true},
			prevUnstaged: false,
			prevStaged:   false,
			want: localChangeActions{
				showUnstaged:  false,
				showStaged:    true,
				loadStaged:    true,
				resetUnstaged: true,
			},
		},
		{
			name:         "no local changes resets without loading",
			status:       git.LocalChanges{HasWorktree: false, HasStaged: false},
			prevUnstaged: true,
			prevStaged:   true,
			want: localChangeActions{
				showUnstaged:  false,
				showStaged:    false,
				resetUnstaged: true,
				resetStaged:   true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := treeState{showLocalUnstaged: tc.prevUnstaged, showLocalStaged: tc.prevStaged}
			got := tree.localChangePlan(true, false, tc.status)
			if got != tc.want {
				t.Fatalf("want %+v, got %+v", tc.want, got)
			}
		})
	}
}
