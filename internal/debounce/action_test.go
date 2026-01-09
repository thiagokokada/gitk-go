package debounce

import (
	"testing"
	"time"
)

func TestActionFlushClearsPending(t *testing.T) {
	var got string
	var action Action[string]
	action.Configure(time.Second, func(value string) {
		got = value
	})
	action.SetPending("hello")
	action.flush()

	if got != "hello" {
		t.Fatalf("expected handler to receive %q, got %q", "hello", got)
	}
	if action.HasPending() {
		t.Fatalf("expected pending to be cleared after flush")
	}
}

func TestActionTriggerCreatesDebouncer(t *testing.T) {
	var action Action[string]
	action.Configure(time.Hour, func(string) {})
	action.Trigger("hello")

	if !action.Active() {
		t.Fatalf("expected debouncer to be created on trigger")
	}
	action.Stop()
	if action.Active() {
		t.Fatalf("expected debouncer to be cleared on stop")
	}
}

func TestActionStopClearsPending(t *testing.T) {
	var action Action[string]
	action.Configure(time.Hour, func(string) {})
	action.SetPending("stale")
	action.Trigger("live")

	action.Stop()

	if action.HasPending() {
		t.Fatalf("expected pending to be cleared on stop")
	}
	if action.Active() {
		t.Fatalf("expected debouncer to be cleared on stop")
	}
}
