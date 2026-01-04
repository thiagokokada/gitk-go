package gui

import (
	"testing"

	"github.com/rjeczalik/notify"
)

func TestEventTriggersReload(t *testing.T) {
	tests := []struct {
		name  string
		event notify.Event
		want  bool
	}{
		{name: "empty", event: 0, want: false},
		{name: "create", event: notify.Create, want: true},
		{name: "remove", event: notify.Remove, want: true},
		{name: "write", event: notify.Write, want: true},
		{name: "rename", event: notify.Rename, want: true},
		{name: "all", event: notify.All, want: true},
		{name: "unknown", event: notify.Event(1 << 30), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventTriggersReload(tt.event); got != tt.want {
				t.Fatalf("eventTriggersReload(%v) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

func TestShouldIgnoreWatchPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "lock", path: "/tmp/index.lock", want: true},
		{name: "lockUpper", path: "/tmp/INDEX.LOCK", want: true},
		{name: "ipc", path: "/tmp/sock.ipc", want: true},
		{name: "regular", path: "/tmp/main.go", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIgnoreWatchPath(tt.path); got != tt.want {
				t.Fatalf("shouldIgnoreWatchPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
