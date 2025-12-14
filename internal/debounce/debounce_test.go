package debounce

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerTriggerOnce(t *testing.T) {
	var count int32
	done := make(chan struct{})
	d := New(10*time.Millisecond, func() {
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
	d := New(20*time.Millisecond, func() {
		atomic.AddInt32(&count, 1)
	})
	d.Trigger()
	d.Stop()
	time.Sleep(40 * time.Millisecond)
	if atomic.LoadInt32(&count) != 0 {
		t.Fatalf("expected no invocations after stop, got %d", count)
	}
}

func TestEnsureReturnsExisting(t *testing.T) {
	var called int32
	handler := func() { atomic.AddInt32(&called, 1) }
	var d *Debouncer
	got := Ensure(&d, 5*time.Millisecond, handler)
	if got == nil || d == nil {
		t.Fatal("Ensure should initialize debouncer")
	}
	if got != d {
		t.Fatal("Ensure should return the stored debouncer")
	}
	got.Trigger()
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("expected handler to be called once, got %d", called)
	}
}

func TestEnsureReusesDebouncer(t *testing.T) {
	var called int32
	handler := func() { atomic.AddInt32(&called, 1) }
	var d *Debouncer
	first := Ensure(&d, 5*time.Millisecond, handler)
	second := Ensure(&d, 5*time.Millisecond, func() {
		atomic.AddInt32(&called, 10)
	})
	if first != second {
		t.Fatal("Ensure should not allocate a new debouncer when already set")
	}
	first.Trigger()
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("new handler should not replace existing debouncer")
	}
}
