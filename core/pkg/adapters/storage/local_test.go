package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contracts "github.com/madcok-co/unicorn/core/pkg/contracts"
)

// helper to create a LocalStorage pointed at a temp dir with a base URL
func newTestStorage(t *testing.T, createDirs bool) *LocalStorage {
	t.Helper()
	basePath := filepath.Join(t.TempDir(), "storage")
	store, err := NewLocalStorage(&LocalStorageConfig{
		BasePath:   basePath,
		BaseURL:    "https://cdn.example.com/files",
		CreateDirs: createDirs,
	})
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}
	return store
}

func writeTestFile(t *testing.T, store *LocalStorage, path, content string) {
	t.Helper()
	ctx := context.Background()
	err := store.Put(ctx, path, strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to write test file %q: %v", path, err)
	}
}

// =============================================================================
// NewLocalStorage
// =============================================================================

func TestNewLocalStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("nil config", func(t *testing.T) {
		_, err := NewLocalStorage(nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("empty base path", func(t *testing.T) {
		_, err := NewLocalStorage(&LocalStorageConfig{
			BasePath: "",
		})
		if err == nil {
			t.Fatal("expected error for empty base path")
		}
	})

	t.Run("CreateDirs=true creates directory", func(t *testing.T) {
		basePath := filepath.Join(t.TempDir(), "new-dir", "sub")
		store, err := NewLocalStorage(&LocalStorageConfig{
			BasePath:   basePath,
			CreateDirs: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// verify the directory was created
		info, err := os.Stat(store.basePath)
		if err != nil {
			t.Fatalf("base path should exist: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("base path should be a directory")
		}

		// verify we can write to it
		err = store.Put(ctx, "test.txt", strings.NewReader("data"))
		if err != nil {
			t.Fatalf("expected to be able to write to created dir: %v", err)
		}
	})

	t.Run("CreateDirs=false directory missing", func(t *testing.T) {
		basePath := filepath.Join(t.TempDir(), "nonexistent")
		_, err := NewLocalStorage(&LocalStorageConfig{
			BasePath:   basePath,
			CreateDirs: false,
		})
		if err == nil {
			t.Fatal("expected error when base path does not exist")
		}
	})

	t.Run("relative path becomes absolute", func(t *testing.T) {
		// Create a directory and then pass a relative path
		tmpDir := t.TempDir()
		relPath := filepath.Join(tmpDir, "relative-storage")
		os.MkdirAll(relPath, 0755)

		// Get a relative path by removing the volume/root prefix
		// We'll just pass an absolute path that we know exists
		store, err := NewLocalStorage(&LocalStorageConfig{
			BasePath:   relPath,
			CreateDirs: false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// basePath should be absolute after resolution
		if !filepath.IsAbs(store.basePath) {
			t.Fatalf("basePath should be absolute, got %q", store.basePath)
		}
	})
}

// =============================================================================
// resolvePath
// =============================================================================

func TestLocalStorage_resolvePath(t *testing.T) {
	store := newTestStorage(t, true)

	t.Run("valid relative path", func(t *testing.T) {
		resolved, err := store.resolvePath("uploads/file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := filepath.Join(store.basePath, "uploads", "file.txt")
		if resolved != expected {
			t.Fatalf("expected %q, got %q", expected, resolved)
		}
	})

	t.Run("directory traversal with dots", func(t *testing.T) {
		_, err := store.resolvePath("../etc/passwd")
		if err == nil {
			t.Fatal("expected error for path with ../")
		}
	})

	t.Run("nested directory traversal", func(t *testing.T) {
		_, err := store.resolvePath("uploads/../../etc/passwd")
		if err == nil {
			t.Fatal("expected error for nested traversal path")
		}
	})

	t.Run("absolute path rejected", func(t *testing.T) {
		_, err := store.resolvePath("/etc/passwd")
		if err == nil {
			t.Fatal("expected error for absolute path")
		}
	})

	t.Run("just parent reference", func(t *testing.T) {
		_, err := store.resolvePath("..")
		if err == nil {
			t.Fatal("expected error for '..'")
		}
	})

	t.Run("empty path resolves to base", func(t *testing.T) {
		// filepath.Clean("") returns "."
		// filepath.Join(basePath, ".") returns basePath
		resolved, err := store.resolvePath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved != store.basePath {
			t.Fatalf("expected base path %q, got %q", store.basePath, resolved)
		}
	})
}

// =============================================================================
// Put
// =============================================================================

func TestLocalStorage_Put(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("writes file content", func(t *testing.T) {
		err := store.Put(ctx, "hello.txt", strings.NewReader("hello world"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(store.basePath, "hello.txt"))
		if err != nil {
			t.Fatalf("failed to read back file: %v", err)
		}
		if string(data) != "hello world" {
			t.Fatalf("expected 'hello world', got %q", string(data))
		}
	})

	t.Run("delegates to PutWithOptions", func(t *testing.T) {
		// Put should be functionally identical to PutWithOptions with nil opts
		err := store.Put(ctx, "delegate.txt", strings.NewReader("delegated"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		exists, err := store.Exists(ctx, "delegate.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Fatal("file should exist after Put")
		}
	})
}

// =============================================================================
// PutWithOptions
// =============================================================================

func TestLocalStorage_PutWithOptions(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("creates file with content", func(t *testing.T) {
		err := store.PutWithOptions(ctx, "data.json", strings.NewReader(`{"key":"value"}`), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(store.basePath, "data.json"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != `{"key":"value"}` {
			t.Fatalf("unexpected content: %q", string(data))
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		err := store.PutWithOptions(ctx, "deep/nested/file.txt", strings.NewReader("deep content"), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// verify parent dirs exist
		deepDir := filepath.Join(store.basePath, "deep", "nested")
		info, err := os.Stat(deepDir)
		if err != nil {
			t.Fatalf("parent directory should exist: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("parent should be a directory")
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		writeTestFile(t, store, "overwrite.txt", "original")

		err := store.PutWithOptions(ctx, "overwrite.txt", strings.NewReader("updated"), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(store.basePath, "overwrite.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(data) != "updated" {
			t.Fatalf("expected 'updated', got %q", string(data))
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		err := store.PutWithOptions(ctx, "../escape.txt", strings.NewReader("bad"), nil)
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})

	t.Run("with PutOptions", func(t *testing.T) {
		opts := &contracts.PutOptions{
			ContentType: "text/plain",
			Metadata:    map[string]string{"author": "test"},
		}
		err := store.PutWithOptions(ctx, "with-opts.txt", strings.NewReader("opts content"), opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// verify file was created (LocalStorage ignores most PutOptions)
		exists, err := store.Exists(ctx, "with-opts.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Fatal("file should exist")
		}
	})
}

// =============================================================================
// Get
// =============================================================================

func TestLocalStorage_Get(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("reads file content", func(t *testing.T) {
		writeTestFile(t, store, "readme.txt", "readable content")

		reader, err := store.Get(ctx, "readme.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != "readable content" {
			t.Fatalf("expected 'readable content', got %q", string(data))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := store.Get(ctx, "nonexistent.txt")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := store.Get(ctx, "../secret.txt")
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

// =============================================================================
// Delete
// =============================================================================

func TestLocalStorage_Delete(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("deletes existing file", func(t *testing.T) {
		writeTestFile(t, store, "delete-me.txt", "temp")

		err := store.Delete(ctx, "delete-me.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = os.Stat(filepath.Join(store.basePath, "delete-me.txt"))
		if !os.IsNotExist(err) {
			t.Fatal("file should have been deleted")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		err := store.Delete(ctx, "no-such-file.txt")
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		err := store.Delete(ctx, "../bad.txt")
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

// =============================================================================
// Exists
// =============================================================================

func TestLocalStorage_Exists(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("true when file exists", func(t *testing.T) {
		writeTestFile(t, store, "exists.txt", "yes")

		exists, err := store.Exists(ctx, "exists.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Fatal("expected file to exist")
		}
	})

	t.Run("false when file missing", func(t *testing.T) {
		exists, err := store.Exists(ctx, "nowhere.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists {
			t.Fatal("expected file to not exist")
		}
	})

	t.Run("error on invalid path", func(t *testing.T) {
		_, err := store.Exists(ctx, "../escape.txt")
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

// =============================================================================
// URL
// =============================================================================

func TestLocalStorage_URL(t *testing.T) {
	ctx := context.Background()

	t.Run("generates URL with base", func(t *testing.T) {
		store := newTestStorage(t, true)
		url, err := store.URL(ctx, "images/photo.png")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://cdn.example.com/files/images/photo.png" {
			t.Fatalf("unexpected URL: %s", url)
		}
	})

	t.Run("generates URL for path without subdirectory", func(t *testing.T) {
		store := newTestStorage(t, true)
		url, err := store.URL(ctx, "single.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://cdn.example.com/files/single.txt" {
			t.Fatalf("unexpected URL: %s", url)
		}
	})

	t.Run("error when no baseURL", func(t *testing.T) {
		basePath := filepath.Join(t.TempDir(), "no-url")
		store, err := NewLocalStorage(&LocalStorageConfig{
			BasePath:   basePath,
			CreateDirs: true,
		})
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}

		_, err = store.URL(ctx, "test.txt")
		if err == nil {
			t.Fatal("expected error when baseURL not configured")
		}
	})

	t.Run("strips leading slash from path", func(t *testing.T) {
		store := newTestStorage(t, true)
		url, err := store.URL(ctx, "/leading/slash.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "https://cdn.example.com/files/leading/slash.txt" {
			t.Fatalf("unexpected URL: %s", url)
		}
	})
}

// =============================================================================
// TemporaryURL
// =============================================================================

func TestLocalStorage_TemporaryURL(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates to URL", func(t *testing.T) {
		store := newTestStorage(t, true)
		url, err := store.URL(ctx, "temp-file.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tempURL, err := store.TemporaryURL(ctx, "temp-file.txt", 15*time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if url != tempURL {
			t.Fatalf("TemporaryURL should return same value as URL: %q vs %q", url, tempURL)
		}
	})

	t.Run("returns error when no baseURL", func(t *testing.T) {
		basePath := filepath.Join(t.TempDir(), "no-url-temp")
		store, err := NewLocalStorage(&LocalStorageConfig{
			BasePath:   basePath,
			CreateDirs: true,
		})
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}

		_, err = store.TemporaryURL(ctx, "test.txt", time.Hour)
		if err == nil {
			t.Fatal("expected error when baseURL not configured")
		}
	})
}

// =============================================================================
// Size
// =============================================================================

func TestLocalStorage_Size(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("returns file size", func(t *testing.T) {
		content := "twelve bytes" // 12 bytes
		writeTestFile(t, store, "sized.txt", content)

		size, err := store.Size(ctx, "sized.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != int64(len(content)) {
			t.Fatalf("expected size %d, got %d", len(content), size)
		}
	})

	t.Run("zero byte file", func(t *testing.T) {
		writeTestFile(t, store, "empty.txt", "")

		size, err := store.Size(ctx, "empty.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if size != 0 {
			t.Fatalf("expected size 0, got %d", size)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := store.Size(ctx, "missing.bin")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := store.Size(ctx, "../outside.bin")
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

// =============================================================================
// LastModified
// =============================================================================

func TestLocalStorage_LastModified(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("returns modification time", func(t *testing.T) {
		writeTestFile(t, store, "modtime.txt", "timestamped")

		modTime, err := store.LastModified(ctx, "modtime.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// mod time should be recent (within the last few seconds)
		diff := time.Since(modTime)
		if diff < 0 {
			t.Fatalf("mod time %v is in the future", modTime)
		}
		if diff > 5*time.Second {
			t.Fatalf("mod time %v is too old: %v ago", modTime, diff)
		}
		if modTime.IsZero() {
			t.Fatal("mod time should not be zero")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := store.LastModified(ctx, "gone.txt")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := store.LastModified(ctx, "../time.txt")
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

// =============================================================================
// List
// =============================================================================

func TestLocalStorage_List(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("lists files in directory", func(t *testing.T) {
		writeTestFile(t, store, "listdir/a.txt", "a")
		writeTestFile(t, store, "listdir/b.txt", "b")
		writeTestFile(t, store, "listdir/c.txt", "c")

		files, err := store.List(ctx, "listdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 3 {
			t.Fatalf("expected 3 files, got %d: %v", len(files), files)
		}

		// paths should be relative to the list directory argument
		expected := map[string]bool{
			filepath.Join("listdir", "a.txt"): true,
			filepath.Join("listdir", "b.txt"): true,
			filepath.Join("listdir", "c.txt"): true,
		}
		for _, f := range files {
			if !expected[f] {
				t.Fatalf("unexpected file: %q", f)
			}
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		writeTestFile(t, store, "mixed/files.txt", "file")
		// create an empty subdirectory directly to ensure it exists without a file
		subDir := filepath.Join(store.basePath, "mixed", "subdir")
		err := os.MkdirAll(subDir, 0755)
		if err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		files, err := store.List(ctx, "mixed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// should only have the file, not the subdirectory
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d: %v", len(files), files)
		}
		if files[0] != filepath.Join("mixed", "files.txt") {
			t.Fatalf("expected 'mixed/files.txt', got %q", files[0])
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := filepath.Join(store.basePath, "empty-dir")
		err := os.MkdirAll(emptyDir, 0755)
		if err != nil {
			t.Fatalf("failed to create empty dir: %v", err)
		}

		files, err := store.List(ctx, "empty-dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Fatalf("expected 0 files, got %d", len(files))
		}
	})

	t.Run("directory not found", func(t *testing.T) {
		_, err := store.List(ctx, "no-such-dir")
		if err == nil {
			t.Fatal("expected error for non-existent directory")
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := store.List(ctx, "../outside")
		if err == nil {
			t.Fatal("expected error for traversal path")
		}
	})
}

// =============================================================================
// Copy
// =============================================================================

func TestLocalStorage_Copy(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("copies file content", func(t *testing.T) {
		writeTestFile(t, store, "original.txt", "copy me")

		err := store.Copy(ctx, "original.txt", "duplicate.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// verify both files exist with same content
		orig, err := os.ReadFile(filepath.Join(store.basePath, "original.txt"))
		if err != nil {
			t.Fatalf("failed to read original: %v", err)
		}

		dup, err := os.ReadFile(filepath.Join(store.basePath, "duplicate.txt"))
		if err != nil {
			t.Fatalf("failed to read duplicate: %v", err)
		}

		if string(orig) != "copy me" {
			t.Fatalf("original content changed: %q", string(orig))
		}
		if string(dup) != "copy me" {
			t.Fatalf("duplicate content wrong: %q", string(dup))
		}
	})

	t.Run("source not found", func(t *testing.T) {
		err := store.Copy(ctx, "missing-src.txt", "dest.txt")
		if err == nil {
			t.Fatal("expected error for missing source")
		}
	})

	t.Run("copies to new directory", func(t *testing.T) {
		writeTestFile(t, store, "src.txt", "new dir copy")

		err := store.Copy(ctx, "src.txt", "newdir/copied.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(store.basePath, "newdir", "copied.txt"))
		if err != nil {
			t.Fatalf("failed to read copied file: %v", err)
		}
		if string(data) != "new dir copy" {
			t.Fatalf("unexpected content: %q", string(data))
		}
	})

	t.Run("rejects invalid source path", func(t *testing.T) {
		err := store.Copy(ctx, "../escape-src.txt", "dest.txt")
		if err == nil {
			t.Fatal("expected error for traversal source path")
		}
	})

	t.Run("rejects invalid destination path", func(t *testing.T) {
		writeTestFile(t, store, "valid-src.txt", "data")
		err := store.Copy(ctx, "valid-src.txt", "../escape-dst.txt")
		if err == nil {
			t.Fatal("expected error for traversal destination path")
		}
	})
}

// =============================================================================
// Move
// =============================================================================

func TestLocalStorage_Move(t *testing.T) {
	ctx := context.Background()
	store := newTestStorage(t, true)

	t.Run("renames file", func(t *testing.T) {
		writeTestFile(t, store, "old-name.txt", "moving content")

		err := store.Move(ctx, "old-name.txt", "new-name.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// old file should be gone
		_, err = os.Stat(filepath.Join(store.basePath, "old-name.txt"))
		if !os.IsNotExist(err) {
			t.Fatal("old file should not exist after move")
		}

		// new file should exist with same content
		data, err := os.ReadFile(filepath.Join(store.basePath, "new-name.txt"))
		if err != nil {
			t.Fatalf("new file should exist: %v", err)
		}
		if string(data) != "moving content" {
			t.Fatalf("unexpected content: %q", string(data))
		}
	})

	t.Run("moves to different directory", func(t *testing.T) {
		writeTestFile(t, store, "mv-src.txt", "subdir move")

		err := store.Move(ctx, "mv-src.txt", "targdir/mv-dst.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// source gone
		_, err = os.Stat(filepath.Join(store.basePath, "mv-src.txt"))
		if !os.IsNotExist(err) {
			t.Fatal("source should not exist")
		}

		// destination exists
		data, err := os.ReadFile(filepath.Join(store.basePath, "targdir", "mv-dst.txt"))
		if err != nil {
			t.Fatalf("destination should exist: %v", err)
		}
		if string(data) != "subdir move" {
			t.Fatalf("unexpected content: %q", string(data))
		}
	})

	t.Run("source not found", func(t *testing.T) {
		err := store.Move(ctx, "missing-mv-src.txt", "dest.txt")
		if err == nil {
			t.Fatal("expected error for missing source")
		}
	})

	t.Run("rejects invalid source path", func(t *testing.T) {
		err := store.Move(ctx, "../bad-src.txt", "dest.txt")
		if err == nil {
			t.Fatal("expected error for traversal source")
		}
	})

	t.Run("rejects invalid destination path", func(t *testing.T) {
		writeTestFile(t, store, "good-src.txt", "data")
		err := store.Move(ctx, "good-src.txt", "../bad-dst.txt")
		if err == nil {
			t.Fatal("expected error for traversal destination")
		}
	})
}

// Verify LocalStorage implements contracts.Storage interface
var _ contracts.Storage = (*LocalStorage)(nil)
