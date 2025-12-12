package gui

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerTriggerOnce(t *testing.T) {
	var count int32
	done := make(chan struct{})
	d := newDebouncer(10*time.Millisecond, func() {
		atomic.AddInt32(&count, 1)
		close(done)
	})
	d.Trigger()
	d.Trigger()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("debouncer did not fire")
	}
	if atomic.LoadInt32(&count) != 1 {
		t.Fatalf("expected one invocation, got %d", count)
	}
}

func TestDebouncerStop(t *testing.T) {
	var count int32
	d := newDebouncer(20*time.Millisecond, func() {
		atomic.AddInt32(&count, 1)
	})
	d.Trigger()
	d.Stop()
	time.Sleep(40 * time.Millisecond)
	if atomic.LoadInt32(&count) != 0 {
		t.Fatalf("expected no invocations after stop, got %d", count)
	}
}
