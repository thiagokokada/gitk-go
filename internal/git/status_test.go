package git

import (
	"errors"
	"testing"
)

func TestLocalChanges_NoBackend(t *testing.T) {
	t.Parallel()

	svc := NewWithBackend(nil)
	_, err := svc.LocalChanges()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLocalChanges_DelegatesToBackend(t *testing.T) {
	t.Parallel()

	want := LocalChanges{HasWorktree: true, HasStaged: false}
	backend := &fakeBackend{
		repoPath: "repo",
		localChangesStatusFunc: func() (LocalChanges, error) {
			return want, nil
		},
	}
	svc := NewWithBackend(backend)

	got, err := svc.LocalChanges()
	if err != nil {
		t.Fatalf("LocalChanges: %v", err)
	}
	if got != want {
		t.Fatalf("LocalChanges = %+v, want %+v", got, want)
	}
}

func TestLocalChanges_PropagatesError(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{
		repoPath: "repo",
		localChangesStatusFunc: func() (LocalChanges, error) {
			return LocalChanges{}, errors.New("boom")
		},
	}
	svc := NewWithBackend(backend)

	_, err := svc.LocalChanges()
	if err == nil {
		t.Fatal("expected error")
	}
}
