package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// LocalStorage implements the Storage interface for local filesystem
type LocalStorage struct {
	basePath string
	baseURL  string
}

// LocalStorageConfig configures local storage
type LocalStorageConfig struct {
	// BasePath is the root directory for file storage
	BasePath string

	// BaseURL is the base URL for generating file URLs (optional)
	BaseURL string

	// CreateDirs automatically creates directories if they don't exist
	CreateDirs bool
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(config *LocalStorageConfig) (*LocalStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.BasePath == "" {
		return nil, fmt.Errorf("base path is required")
	}

	// Ensure base path is absolute
	absPath, err := filepath.Abs(config.BasePath)
	if err != nil {
		return nil, fmt.Errorf("invalid base path: %w", err)
	}

	// Create base directory if needed
	if config.CreateDirs {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create base directory: %w", err)
		}
	}

	// Verify base path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("base path does not exist: %s", absPath)
	}

	return &LocalStorage{
		basePath: absPath,
		baseURL:  strings.TrimSuffix(config.BaseURL, "/"),
	}, nil
}

// resolvePath resolves a relative path to an absolute path within the base directory
func (s *LocalStorage) resolvePath(path string) (string, error) {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)

	// Prevent absolute paths and directory traversal
	if filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "..") {
		return "", fmt.Errorf("invalid path: %s", path)
	}

	fullPath := filepath.Join(s.basePath, cleanPath)

	// Ensure the resolved path is still within base directory
	if !strings.HasPrefix(fullPath, s.basePath) {
		return "", fmt.Errorf("path outside base directory: %s", path)
	}

	return fullPath, nil
}

// Put stores a file from a reader
func (s *LocalStorage) Put(ctx context.Context, path string, reader io.Reader) error {
	return s.PutWithOptions(ctx, path, reader, nil)
}

// PutWithOptions stores a file with additional options
func (s *LocalStorage) PutWithOptions(ctx context.Context, path string, reader io.Reader, opts *contracts.PutOptions) error {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}

	// Create parent directory
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get retrieves a file
func (s *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists checks if a file exists
func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// URL generates a URL for accessing the file
func (s *LocalStorage) URL(ctx context.Context, path string) (string, error) {
	if s.baseURL == "" {
		return "", fmt.Errorf("base URL not configured")
	}

	// Clean path
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	return fmt.Sprintf("%s/%s", s.baseURL, cleanPath), nil
}

// TemporaryURL generates a temporary signed URL (not supported for local storage)
func (s *LocalStorage) TemporaryURL(ctx context.Context, path string, expiration time.Duration) (string, error) {
	// For local storage, we return the regular URL since we can't sign URLs
	return s.URL(ctx, path)
}

// Size returns the file size in bytes
func (s *LocalStorage) Size(ctx context.Context, path string) (int64, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found: %s", path)
		}
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	return info.Size(), nil
}

// LastModified returns the last modification time
func (s *LocalStorage) LastModified(ctx context.Context, path string) (time.Time, error) {
	fullPath, err := s.resolvePath(path)
	if err != nil {
		return time.Time{}, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, fmt.Errorf("file not found: %s", path)
		}
		return time.Time{}, fmt.Errorf("failed to stat file: %w", err)
	}

	return info.ModTime(), nil
}

// List lists files in a directory
func (s *LocalStorage) List(ctx context.Context, directory string) ([]string, error) {
	fullPath, err := s.resolvePath(directory)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", directory)
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			relPath := filepath.Join(directory, entry.Name())
			files = append(files, relPath)
		}
	}

	return files, nil
}

// Copy copies a file
func (s *LocalStorage) Copy(ctx context.Context, src, dst string) error {
	srcPath, err := s.resolvePath(src)
	if err != nil {
		return err
	}

	dstPath, err := s.resolvePath(dst)
	if err != nil {
		return err
	}

	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file not found: %s", src)
		}
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination directory
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create destination file
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy data
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Move moves a file
func (s *LocalStorage) Move(ctx context.Context, src, dst string) error {
	srcPath, err := s.resolvePath(src)
	if err != nil {
		return err
	}

	dstPath, err := s.resolvePath(dst)
	if err != nil {
		return err
	}

	// Create destination directory
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Rename file
	if err := os.Rename(srcPath, dstPath); err != nil {
		// If rename fails (e.g., cross-device), try copy + delete
		if err := s.Copy(ctx, src, dst); err != nil {
			return err
		}
		return s.Delete(ctx, src)
	}

	return nil
}
