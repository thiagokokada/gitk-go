package git

import (
	"errors"
	"testing"
)

func TestStagePatchToIndex_NoBackend(t *testing.T) {
	svc := &Service{}
	if err := svc.StagePatchToIndex("patch", false); err == nil {
		t.Fatalf("expected error for missing backend")
	}
}

func TestStagePatchToIndex_EmptyPatch(t *testing.T) {
	backend := &fakeBackend{repoPath: "/tmp"}
	svc := NewWithBackend(backend)
	if err := svc.StagePatchToIndex("   ", false); err == nil {
		t.Fatalf("expected error for empty patch")
	}
}

func TestStagePatchToIndex_DelegatesToBackend(t *testing.T) {
	backend := &fakeBackend{
		repoPath: "/tmp",
		applyPatchToIndexFunc: func(patch string, reverse bool) error {
			if patch != "patch" {
				t.Fatalf("patch = %q, want %q", patch, "patch")
			}
			if reverse {
				t.Fatalf("reverse = true, want false")
			}
			return nil
		},
	}
	svc := NewWithBackend(backend)
	if err := svc.StagePatchToIndex("patch", false); err != nil {
		t.Fatalf("StagePatchToIndex: %v", err)
	}
	if backend.lastPatch != "patch" {
		t.Fatalf("lastPatch = %q, want %q", backend.lastPatch, "patch")
	}
	if backend.lastPatchReverse == nil || *backend.lastPatchReverse {
		t.Fatalf("lastPatchReverse = %v, want false", backend.lastPatchReverse)
	}
}

func TestStagePatchToIndex_PropagatesError(t *testing.T) {
	backend := &fakeBackend{
		repoPath: "/tmp",
		applyPatchToIndexFunc: func(string, bool) error {
			return errors.New("boom")
		},
	}
	svc := NewWithBackend(backend)
	if err := svc.StagePatchToIndex("patch", true); err == nil {
		t.Fatalf("expected error to propagate")
	}
}
