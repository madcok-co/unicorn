package configwatcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ============ Initial load ============

func TestConfigWatcher_InitialLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "key: value")

	loaded := make(chan []byte, 1)
	w := New(&Config{
		Paths: []string{cfgPath},
		OnReload: func(_ string, content []byte) error {
			loaded <- content
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Start(ctx)

	select {
	case content := <-loaded:
		if string(content) != "key: value" {
			t.Fatalf("unexpected content: %q", content)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for initial load")
	}
}

func TestConfigWatcher_InitialLoad_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.yaml")
	fileB := filepath.Join(dir, "b.yaml")
	writeFile(t, fileA, "a")
	writeFile(t, fileB, "b")

	var loadedPaths []string
	done := make(chan struct{})

	w := New(&Config{
		Paths: []string{fileA, fileB},
		OnReload: func(path string, _ []byte) error {
			loadedPaths = append(loadedPaths, path)
			if len(loadedPaths) == 2 {
				close(done)
			}
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Start(ctx)

	select {
	case <-done:
		// both files loaded
	case <-ctx.Done():
		t.Fatalf("timed out; only loaded %v", loadedPaths)
	}
}

// ============ File change detection ============

func TestConfigWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "version: 1")

	var reloads atomic.Int32
	w := New(&Config{
		Paths:    []string{cfgPath},
		Debounce: 50 * time.Millisecond,
		OnReload: func(_ string, _ []byte) error {
			reloads.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go w.Start(ctx)

	// Wait for initial load
	time.Sleep(100 * time.Millisecond)
	initialCount := reloads.Load()

	// Modify the file
	writeFile(t, cfgPath, "version: 2")

	// Wait for debounced reload
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if reloads.Load() > initialCount {
			return // success
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for file change reload")
}

// ============ Debounce ============

func TestConfigWatcher_Debounce_CoalescesEvents(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "v0")

	var reloadCount atomic.Int32
	w := New(&Config{
		Paths:    []string{cfgPath},
		Debounce: 200 * time.Millisecond,
		OnReload: func(_ string, _ []byte) error {
			reloadCount.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go w.Start(ctx)
	time.Sleep(50 * time.Millisecond) // wait past initial load

	// Fire multiple rapid events
	before := reloadCount.Load()
	for range 5 {
		w.scheduleReload(cfgPath)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the single debounced reload
	time.Sleep(400 * time.Millisecond)
	after := reloadCount.Load()

	// Should be exactly 1 additional reload, not 5
	if delta := after - before; delta != 1 {
		t.Fatalf("expected 1 debounced reload, got %d", delta)
	}
}

// ============ Error handler ============

func TestConfigWatcher_ErrHandler_InitialLoadError(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "missing.yaml")

	var errPath string
	w := New(&Config{
		Paths: []string{nonExistent},
		OnReload: func(_ string, _ []byte) error {
			return nil
		},
		ErrHandler: func(path string, err error) {
			errPath = path
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w.Start(ctx)

	if errPath == "" {
		t.Fatal("expected ErrHandler to be called for missing file")
	}
}

func TestConfigWatcher_ErrHandler_ReloadError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "ok")

	var errCount atomic.Int32
	w := New(&Config{
		Paths:    []string{cfgPath},
		Debounce: 10 * time.Millisecond,
		OnReload: func(_ string, _ []byte) error {
			return os.ErrPermission // simulate reload failure
		},
		ErrHandler: func(_ string, _ error) {
			errCount.Add(1)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w.Start(ctx)

	if errCount.Load() == 0 {
		t.Fatal("expected ErrHandler called on reload error")
	}
}

// ============ Stop cancels AfterFunc (R4 fix) ============

func TestConfigWatcher_Stop_CancelsAfterFuncCallback(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "data")

	var callCount atomic.Int32
	initialDone := make(chan struct{}, 1)

	w := New(&Config{
		Paths:    []string{cfgPath},
		Debounce: 500 * time.Millisecond, // long debounce — gives us time to Stop first
		OnReload: func(_ string, _ []byte) error {
			if callCount.Add(1) == 1 {
				select {
				case initialDone <- struct{}{}:
				default:
				}
			}
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go w.Start(ctx)

	// Wait for the initial synchronous load to complete before proceeding.
	select {
	case <-initialDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial load")
	}

	// Schedule a debounced reload (will fire in ~500ms if not stopped)
	w.scheduleReload(cfgPath)

	// Stop immediately — before the 500ms debounce fires
	w.Stop(context.Background())

	// Wait longer than the debounce period
	time.Sleep(700 * time.Millisecond)

	// Only the initial load (count=1) should have fired; the post-Stop AfterFunc must not.
	if n := callCount.Load(); n > 1 {
		t.Fatalf("AfterFunc callback fired after Stop() — count=%d, want 1 (R4 fix)", n)
	}
}

func TestConfigWatcher_Stop_SetsStoppedFlag(t *testing.T) {
	// After Stop(), the stopped flag must be true (verifies R4 fix mechanism).
	w := New(&Config{
		Paths:    []string{"/tmp/x.yaml"},
		OnReload: func(_ string, _ []byte) error { return nil },
	})

	w.Stop(context.Background())

	if !w.stopped.Load() {
		t.Fatal("stopped flag should be true after Stop()")
	}
}

// ============ Symlink rejection (C2 fix) ============

func TestConfigWatcher_SymlinkEscape_Rejected(t *testing.T) {
	// Create a "sensitive" file OUTSIDE the watched directory.
	outsideDir := t.TempDir()
	sensitiveFile := filepath.Join(outsideDir, "sensitive.txt")
	writeFile(t, sensitiveFile, "secret-data")

	// Create a watched directory.
	watchDir := t.TempDir()

	// Create a symlink inside watchDir pointing to the outside file.
	symlinkPath := filepath.Join(watchDir, "evil.yaml")
	if err := os.Symlink(sensitiveFile, symlinkPath); err != nil {
		t.Skipf("cannot create symlink (may need privilege): %v", err)
	}

	var reloadContent []byte
	var errMsg string
	w := New(&Config{
		Paths: []string{symlinkPath},
		OnReload: func(_ string, content []byte) error {
			reloadContent = content
			return nil
		},
		ErrHandler: func(_ string, err error) {
			errMsg = err.Error()
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w.Start(ctx)

	if len(reloadContent) > 0 {
		t.Fatal("symlink pointing outside watched dir should not be loaded (C2 fix)")
	}
	if !strings.Contains(errMsg, "outside") {
		t.Fatalf("expected 'outside watched directories' error, got %q", errMsg)
	}
}

func TestConfigWatcher_SymlinkWithinDir_Allowed(t *testing.T) {
	// A symlink pointing to another file WITHIN the same directory is allowed.
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.yaml")
	writeFile(t, realFile, "real-content")

	symlinkPath := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	loaded := make(chan []byte, 1)
	w := New(&Config{
		Paths: []string{symlinkPath},
		OnReload: func(_ string, content []byte) error {
			loaded <- content
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Start(ctx)

	select {
	case content := <-loaded:
		if string(content) != "real-content" {
			t.Fatalf("expected real-content, got %q", content)
		}
	case <-ctx.Done():
		t.Fatal("timed out — symlink within same dir should be loadable")
	}
}

// ============ isAllowedPath ============

func TestIsAllowedPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "app.yaml")
	writeFile(t, cfgPath, "")

	w := New(&Config{
		Paths:    []string{cfgPath},
		OnReload: func(_ string, _ []byte) error { return nil },
	})

	if !w.isAllowedPath(cfgPath) {
		t.Fatal("direct match should be allowed")
	}
	sibling := filepath.Join(dir, "app.yaml.new")
	if !w.isAllowedPath(sibling) {
		t.Fatal("file in same directory should be allowed (atomic write target)")
	}
	outside := filepath.Join(t.TempDir(), "evil.yaml")
	if w.isAllowedPath(outside) {
		t.Fatal("file outside watched directory should NOT be allowed")
	}
}

// ============ Name ============

func TestConfigWatcher_Name(t *testing.T) {
	w := New(&Config{Paths: []string{"x.yaml"}, OnReload: func(_ string, _ []byte) error { return nil }})
	if w.Name() != "config-watcher" {
		t.Fatalf("expected 'config-watcher', got %q", w.Name())
	}
}

// ============ isTracked ============

func TestIsTracked(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	w := New(&Config{
		Paths:    []string{cfgPath},
		OnReload: func(_ string, _ []byte) error { return nil },
	})

	if !w.isTracked(cfgPath) {
		t.Fatal("expected tracked path to return true")
	}
	other := filepath.Join(dir, "other.yaml")
	if w.isTracked(other) {
		t.Fatal("unregistered path should not be tracked")
	}
}

// ============ Polling fallback ============

func TestConfigWatcher_RunPolling_DetectsNewFile(t *testing.T) {
	// When a watched file doesn't exist at startup, runPolling should detect
	// it once it is created (snapshot is empty → !ok → triggers reload).
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	loaded := make(chan []byte, 1)
	w := New(&Config{
		Paths:        []string{cfgPath},
		PollInterval: 20 * time.Millisecond,
		OnReload: func(_ string, content []byte) error {
			select {
			case loaded <- content:
			default:
			}
			return nil
		},
		ErrHandler: func(_ string, _ error) {}, // suppress initial "file not found"
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.runPolling(ctx)

	// Let snapshot be taken (file missing → empty snapshot)
	time.Sleep(50 * time.Millisecond)

	// Now create the file — poll should detect it on next tick
	writeFile(t, cfgPath, "created-content")

	select {
	case content := <-loaded:
		if string(content) != "created-content" {
			t.Fatalf("expected 'created-content', got %q", string(content))
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for polling to detect new file")
	}
}

func TestConfigWatcher_RunPolling_DetectsModification(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, "original")

	reloaded := make(chan []byte, 1)
	w := New(&Config{
		Paths:        []string{cfgPath},
		PollInterval: 20 * time.Millisecond,
		OnReload: func(_ string, content []byte) error {
			select {
			case reloaded <- content:
			default:
			}
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.runPolling(ctx)

	// Allow snapshot to be taken before modifying the file.
	time.Sleep(50 * time.Millisecond)
	writeFile(t, cfgPath, "modified")

	select {
	case content := <-reloaded:
		if string(content) != "modified" {
			t.Fatalf("expected 'modified', got %q", string(content))
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for poll to detect modification")
	}
}

func TestConfigWatcher_TakeSnapshots(t *testing.T) {
	dir := t.TempDir()
	existsPath := filepath.Join(dir, "exists.yaml")
	missingPath := filepath.Join(dir, "missing.yaml")
	writeFile(t, existsPath, "data")

	w := New(&Config{
		Paths:    []string{existsPath, missingPath},
		OnReload: func(_ string, _ []byte) error { return nil },
	})

	snaps := w.takeSnapshots()

	if _, ok := snaps[existsPath]; !ok {
		t.Fatal("existing file should appear in snapshots")
	}
	if _, ok := snaps[missingPath]; ok {
		t.Fatal("missing file should NOT appear in snapshots")
	}
}

// ============ fileModTime ============

func TestFileModTime_ExistingFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	mt, err := fileModTime(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mt.IsZero() {
		t.Fatal("expected non-zero mod time")
	}
}

func TestFileModTime_MissingFile(t *testing.T) {
	_, err := fileModTime("/nonexistent/path/to/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ============ Helper ============

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
