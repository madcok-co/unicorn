// Package configwatcher provides the ConfigWatcher sidecar for hot-reloading
// configuration without restarting the service. Uses fsnotify (event-based)
// with automatic fallback to polling when a watcher is unavailable.
package configwatcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ReloadFunc is called whenever a watched config file changes.
// content is the latest file contents. Returning an error discards the change and logs it.
type ReloadFunc func(path string, content []byte) error

// Config holds ConfigWatcher configuration.
type Config struct {
	// Paths is the list of files or directories to watch.
	Paths []string

	// OnReload is called when a config change is detected.
	OnReload ReloadFunc

	// Debounce prevents rapid-fire reloads during burst writes (e.g. editor rename-on-save).
	// Default: 200ms
	Debounce time.Duration

	// PollInterval is used when fsnotify is unavailable (fallback mode).
	// Default: 30s
	PollInterval time.Duration

	// ErrHandler is called on reload errors. Defaults to printing to stderr.
	ErrHandler func(path string, err error)
}

// ConfigWatcher is a sidecar that monitors config files and triggers reloads.
type ConfigWatcher struct {
	config  *Config
	watcher *fsnotify.Watcher
	mu      sync.Mutex
	timers  map[string]*time.Timer
	stopped atomic.Bool
}

// New creates a new ConfigWatcher. Call Start() to begin monitoring.
func New(config *Config) *ConfigWatcher {
	if config.Debounce == 0 {
		config.Debounce = 200 * time.Millisecond
	}
	if config.PollInterval == 0 {
		config.PollInterval = 30 * time.Second
	}
	if config.ErrHandler == nil {
		config.ErrHandler = func(path string, err error) {
			fmt.Fprintf(os.Stderr, "[configwatcher] reload error %s: %v\n", path, err)
		}
	}
	return &ConfigWatcher{
		config: config,
		timers: make(map[string]*time.Timer),
	}
}

// Name implements contracts.Sidecar.
func (w *ConfigWatcher) Name() string {
	return "config-watcher"
}

// Start implements contracts.Sidecar. Blocks until ctx is cancelled.
func (w *ConfigWatcher) Start(ctx context.Context) error {
	// Perform initial load of all paths
	for _, p := range w.config.Paths {
		if err := w.loadAndNotify(p); err != nil {
			w.config.ErrHandler(p, fmt.Errorf("initial load: %w", err))
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		// fsnotify unavailable — fall back to polling
		return w.runPolling(ctx)
	}
	// Store under mu so Stop() can read w.watcher safely from another goroutine.
	w.mu.Lock()
	w.watcher = watcher
	w.mu.Unlock()
	defer watcher.Close()

	// Register all paths with fsnotify.
	// Watch the parent directory to catch atomic writes (editor rename-on-save).
	for _, p := range w.config.Paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			w.config.ErrHandler(p, err)
			continue
		}
		dir := filepath.Dir(absPath)
		if err := watcher.Add(dir); err != nil {
			w.config.ErrHandler(p, fmt.Errorf("watch %s: %w", dir, err))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !w.isTracked(event.Name) {
				continue
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				w.scheduleReload(event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.config.ErrHandler("watcher", err)
		}
	}
}

// Stop implements contracts.Sidecar.
func (w *ConfigWatcher) Stop(_ context.Context) error {
	// Signal stopped before cancelling timers so any in-flight AfterFunc
	// callbacks that fire after this point are no-ops.
	w.stopped.Store(true)

	w.mu.Lock()
	for _, t := range w.timers {
		t.Stop()
	}
	w.mu.Unlock()

	w.mu.Lock()
	fw := w.watcher
	w.mu.Unlock()

	if fw != nil {
		return fw.Close()
	}
	return nil
}

// ============ Polling fallback ============

func (w *ConfigWatcher) runPolling(ctx context.Context) error {
	snapshots := w.takeSnapshots()

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, p := range w.config.Paths {
				curr, err := fileModTime(p)
				if err != nil {
					continue
				}
				if prev, ok := snapshots[p]; !ok || curr.After(prev) {
					snapshots[p] = curr
					if err := w.loadAndNotify(p); err != nil {
						w.config.ErrHandler(p, err)
					}
				}
			}
		}
	}
}

func (w *ConfigWatcher) takeSnapshots() map[string]time.Time {
	m := make(map[string]time.Time, len(w.config.Paths))
	for _, p := range w.config.Paths {
		if t, err := fileModTime(p); err == nil {
			m[p] = t
		}
	}
	return m
}

// ============ Helpers ============

// scheduleReload debounces reloads: resets the timer on every incoming event.
func (w *ConfigWatcher) scheduleReload(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}
	w.timers[path] = time.AfterFunc(w.config.Debounce, func() {
		// Guard against callbacks that fire after Stop() has been called.
		if w.stopped.Load() {
			return
		}
		if err := w.loadAndNotify(path); err != nil {
			w.config.ErrHandler(path, err)
		}
	})
}

func (w *ConfigWatcher) loadAndNotify(path string) error {
	// Resolve symlinks to prevent reading files outside the watched directories.
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve symlinks %s: %w", path, err)
	}
	if !w.isAllowedPath(resolved) {
		return fmt.Errorf("symlink %s resolves to %s outside watched directories", path, resolved)
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return w.config.OnReload(path, content)
}

// isTracked reports whether path is in the list of watched paths.
func (w *ConfigWatcher) isTracked(path string) bool {
	absPath, _ := filepath.Abs(path)
	for _, p := range w.config.Paths {
		tracked, _ := filepath.Abs(p)
		if absPath == tracked {
			return true
		}
	}
	return false
}

// isAllowedPath reports whether a resolved (symlink-free) path is within the
// same directory as one of the configured watched paths.
func (w *ConfigWatcher) isAllowedPath(resolved string) bool {
	for _, p := range w.config.Paths {
		watched, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		// Direct match or within the same parent directory.
		if resolved == watched {
			return true
		}
		dir := filepath.Dir(watched) + string(filepath.Separator)
		if strings.HasPrefix(resolved, dir) {
			return true
		}
	}
	return false
}

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
