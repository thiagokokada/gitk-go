package gui

import (
	"testing"

	"github.com/sgtdi/fswatcher"
)

func TestEventTriggersReload(t *testing.T) {
	tests := []struct {
		name  string
		event []fswatcher.EventType
		want  bool
	}{
		{name: "empty", event: nil, want: false},
		{name: "create", event: []fswatcher.EventType{fswatcher.EventCreate}, want: true},
		{name: "remove", event: []fswatcher.EventType{fswatcher.EventRemove}, want: true},
		{name: "write", event: []fswatcher.EventType{fswatcher.EventMod}, want: true},
		{name: "rename", event: []fswatcher.EventType{fswatcher.EventRename}, want: true},
		{name: "chmod", event: []fswatcher.EventType{fswatcher.EventChmod}, want: true},
		{name: "unknown", event: []fswatcher.EventType{fswatcher.EventUnknown}, want: false},
		{
			name:  "mixed",
			event: []fswatcher.EventType{fswatcher.EventUnknown, fswatcher.EventCreate},
			want:  true,
		},
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
