package gui

import (
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/thiagokokada/gitk-go/internal/debounce"
	. "modernc.org/tk9.0"
)

const autoReloadDebounceDelay = 350 * time.Millisecond

type autoReloadState struct {
	mu         sync.Mutex
	configured bool
	enabled    bool
	watcher    *fsnotify.Watcher
	debounce   *debounce.Debouncer
	button     *TButtonWidget
}

func (a *Controller) initAutoReload(requested bool) {
	a.watch.mu.Lock()
	a.watch.configured = requested
	a.watch.mu.Unlock()
	if requested {
		if err := a.enableAutoReload(); err != nil {
			slog.Error("auto reload disabled", slog.Any("error", err))
			a.watch.mu.Lock()
			a.watch.configured = false
			a.watch.mu.Unlock()
		}
	}
	a.updateReloadButtonLabel()
}

func (a *Controller) enableAutoReload() error {
	a.watch.mu.Lock()
	defer a.watch.mu.Unlock()
	if !a.watch.configured {
		return nil
	}
	if a.watch.enabled {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	for path := range watchPaths(a.repoPath) {
		slog.Debug("adding path to FS watcher", slog.String("path", path))
		if err := watcher.Add(path); err != nil {
			err := errors.Join(err, watcher.Close())
			return fmt.Errorf("watch %s: %w", path, err)
		}
	}
	if a.watch.debounce == nil {
		a.watch.debounce = debounce.New(autoReloadDebounceDelay, func() {
			PostEvent(func() {
				a.reloadCommitsAsync()
			}, false)
		})
	}
	a.watch.watcher = watcher
	a.watch.enabled = true
	go a.watchLoop(watcher)
	return nil
}

func (a *Controller) disableAutoReload() {
	a.watch.mu.Lock()
	defer a.watch.mu.Unlock()
	if a.watch.debounce != nil {
		a.watch.debounce.Stop()
		a.watch.debounce = nil
	}
	if a.watch.watcher != nil {
		err := a.watch.watcher.Close()
		if err != nil {
			slog.Error("watcher close", slog.Any("error", err))
		}
		a.watch.watcher = nil
	}
	a.watch.enabled = false
}

func (a *Controller) shutdown() {
	a.disableAutoReload()
}

func (a *Controller) watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if shouldIgnoreWatchPath(ev.Name) {
				continue
			}
			slog.Debug("fsnotify event",
				slog.String("op", ev.Op.String()),
				slog.String("path", ev.Name),
			)
			a.scheduleAutoReload()
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify error", slog.Any("error", err))
		}
	}
}

func (a *Controller) scheduleAutoReload() {
	a.watch.mu.Lock()
	defer a.watch.mu.Unlock()
	if !a.watch.enabled || a.watch.debounce == nil {
		return
	}
	slog.Debug("auto reload scheduled")
	a.watch.debounce.Trigger()
}

func watchPaths(root string) iter.Seq[string] {
	if root == "" {
		return nil
	}
	uniquePaths := map[string]struct{}{}
	appendUnique := func(p string) { uniquePaths[p] = struct{}{} }
	gitDir := filepath.Join(root, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		appendUnique(gitDir)
		return maps.Keys(uniquePaths)
	}
	appendUnique(root)
	return maps.Keys(uniquePaths)
}

func shouldIgnoreWatchPath(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".lock" || ext == ".ipc" {
		return true
	}
	return false
}

func (a *Controller) updateReloadButtonLabel() {
	if a.watch.button == nil {
		return
	}
	label := "Reload"
	a.watch.mu.Lock()
	configured := a.watch.configured
	enabled := a.watch.enabled
	a.watch.mu.Unlock()
	if configured {
		state := "Off"
		if enabled {
			state = "On"
		}
		label = fmt.Sprintf("Reload (Auto %s)", state)
	}
	a.watch.button.Configure(Txt(label))
}

func (a *Controller) onReloadButton() {
	a.watch.mu.Lock()
	configured := a.watch.configured
	enabled := a.watch.enabled
	a.watch.mu.Unlock()
	if !configured {
		a.reloadCommitsAsync()
		return
	}
	if enabled {
		a.disableAutoReload()
	} else {
		if err := a.enableAutoReload(); err != nil {
			slog.Error("auto reload enable failed", slog.Any("error", err))
		}
	}
	a.updateReloadButtonLabel()
	a.reloadCommitsAsync()
}
