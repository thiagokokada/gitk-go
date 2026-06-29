package watch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sgtdi/fswatcher"
	"github.com/thiagokokada/gitk-go/internal/debounce"
)

const autoReloadDebounceDelay = 350 * time.Millisecond

type Watcher struct {
	mu         sync.Mutex
	configured bool
	enabled    bool
	watcher    fswatcher.Watcher
	cancel     context.CancelFunc
	debounce   *debounce.Debouncer
}

func New(requested bool) *Watcher {
	return &Watcher{configured: requested}
}

func (s *Watcher) Init(requested bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configured = requested
}

func (s *Watcher) IsConfigured() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configured
}

func (s *Watcher) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

func (s *Watcher) Start(repoPath string, triggerReload func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.configured || s.enabled {
		return nil
	}

	slog.Debug("adding path to fswatcher", slog.String("path", repoPath))
	watcher, err := fswatcher.New(
		fswatcher.WithPath(repoPath, fswatcher.WithDepth(fswatcher.WatchNested)),
	)
	if err != nil {
		return fmt.Errorf("watch %s: %w", repoPath, err)
	}

	if s.debounce == nil {
		s.debounce = debounce.New(autoReloadDebounceDelay, triggerReload)
	}
	s.watcher = watcher
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.enabled = true

	go s.watchLoop(watcher.Events())
	go func() {
		if err := watcher.Watch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("auto reload watch failed", slog.Any("error", err))
		}
	}()
	return nil
}

func (s *Watcher) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debounce != nil {
		s.debounce.Stop()
		s.debounce = nil
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}
	s.enabled = false
}

func (s *Watcher) watchLoop(events <-chan fswatcher.WatchEvent) {
	for ev := range events {
		if !eventTriggersReload(ev.Types) {
			continue
		}
		if shouldIgnoreWatchPath(ev.Path) {
			continue
		}
		slog.Debug(
			"fswatcher event",
			slog.String("event", eventTypesString(ev.Types)),
			slog.String("path", ev.Path),
		)
		s.Schedule()
	}
}

func (s *Watcher) Schedule() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled || s.debounce == nil {
		return
	}
	slog.Debug("auto reload scheduled")
	s.debounce.Trigger()
}

func shouldIgnoreWatchPath(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".lock" || ext == ".ipc"
}

func eventTriggersReload(types []fswatcher.EventType) bool {
	for _, eventType := range types {
		if eventType != fswatcher.EventUnknown {
			return true
		}
	}
	return false
}

func eventTypesString(types []fswatcher.EventType) string {
	if len(types) == 0 {
		return "Unknown"
	}
	names := make([]string, 0, len(types))
	for _, eventType := range types {
		names = append(names, eventType.String())
	}
	return strings.Join(names, "|")
}
