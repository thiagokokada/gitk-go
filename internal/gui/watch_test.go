package gui

import (
	"testing"

	"github.com/sgtdi/fswatcher"
)

func TestEventTriggersReload(t *testing.T) {
	tests := []struct {
		name  string
		types []fswatcher.EventType
		want  bool
	}{
		{name: "empty", types: nil, want: true},
		{name: "unknown", types: []fswatcher.EventType{fswatcher.EventUnknown}, want: true},
		{name: "create", types: []fswatcher.EventType{fswatcher.EventCreate}, want: true},
		{name: "remove", types: []fswatcher.EventType{fswatcher.EventRemove}, want: true},
		{name: "mod", types: []fswatcher.EventType{fswatcher.EventMod}, want: true},
		{name: "rename", types: []fswatcher.EventType{fswatcher.EventRename}, want: true},
		{name: "chmod", types: []fswatcher.EventType{fswatcher.EventChmod}, want: true},
		{name: "invalid", types: []fswatcher.EventType{fswatcher.EventType(99)}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := fswatcher.WatchEvent{Types: tt.types}
			if got := eventTriggersReload(event); got != tt.want {
				t.Fatalf("eventTriggersReload(%v) = %v, want %v", tt.types, got, tt.want)
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
