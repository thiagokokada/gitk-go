package model

import (
	"testing"

	"github.com/thiagokokada/gitk-go/internal/git"
)

func TestLocalChangePlanRepoNotReady(t *testing.T) {
	status := git.LocalChanges{HasWorktree: true, HasStaged: true}
	actions := (TreeState{ShowLocalUnstaged: true, ShowLocalStaged: true}).LocalChangePlan(false, true, status)
	if actions.ShowUnstaged || actions.ShowStaged {
		t.Fatalf("expected no rows shown when repo is not ready, got %+v", actions)
	}
	if !actions.ResetUnstaged || !actions.ResetStaged {
		t.Fatalf("expected both states reset when repo is not ready, got %+v", actions)
	}
	if actions.LoadUnstaged || actions.LoadStaged {
		t.Fatalf("expected no loads when repo is not ready, got %+v", actions)
	}
}

func TestLocalChangePlanPrefetch(t *testing.T) {
	status := git.LocalChanges{HasWorktree: true, HasStaged: false}
	actions := (TreeState{ShowLocalUnstaged: false, ShowLocalStaged: true}).LocalChangePlan(true, true, status)
	if !actions.ShowUnstaged || actions.ShowStaged {
		t.Fatalf("unexpected show flags: %+v", actions)
	}
	if !actions.LoadUnstaged || actions.LoadStaged {
		t.Fatalf("unexpected load flags: %+v", actions)
	}
	if actions.ResetUnstaged || !actions.ResetStaged {
		t.Fatalf("unexpected reset flags: %+v", actions)
	}
}

func TestLocalChangePlanNoPrefetchLoadsOnTransition(t *testing.T) {
	tests := []struct {
		name         string
		status       git.LocalChanges
		prevUnstaged bool
		prevStaged   bool
		want         LocalChangeActions
	}{
		{
			name:         "worktree becomes visible triggers load",
			status:       git.LocalChanges{HasWorktree: true, HasStaged: false},
			prevUnstaged: false,
			prevStaged:   false,
			want: LocalChangeActions{
				ShowUnstaged: true,
				ShowStaged:   false,
				LoadUnstaged: true,
				ResetStaged:  true,
			},
		},
		{
			name:         "already visible does not reload",
			status:       git.LocalChanges{HasWorktree: true, HasStaged: true},
			prevUnstaged: true,
			prevStaged:   true,
			want: LocalChangeActions{
				ShowUnstaged: true,
				ShowStaged:   true,
			},
		},
		{
			name:         "staged becomes visible triggers load",
			status:       git.LocalChanges{HasWorktree: false, HasStaged: true},
			prevUnstaged: false,
			prevStaged:   false,
			want: LocalChangeActions{
				ShowUnstaged:  false,
				ShowStaged:    true,
				LoadStaged:    true,
				ResetUnstaged: true,
			},
		},
		{
			name:         "no local changes resets without loading",
			status:       git.LocalChanges{HasWorktree: false, HasStaged: false},
			prevUnstaged: true,
			prevStaged:   true,
			want: LocalChangeActions{
				ShowUnstaged:  false,
				ShowStaged:    false,
				ResetUnstaged: true,
				ResetStaged:   true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := TreeState{ShowLocalUnstaged: tc.prevUnstaged, ShowLocalStaged: tc.prevStaged}
			got := tree.LocalChangePlan(true, false, tc.status)
			if got != tc.want {
				t.Fatalf("want %+v, got %+v", tc.want, got)
			}
		})
	}
}
