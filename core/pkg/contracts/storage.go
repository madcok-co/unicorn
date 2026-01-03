package contracts

import (
	"context"
	"io"
	"time"
)

// Storage defines the interface for file storage operations
type Storage interface {
	// Put stores a file from a reader
	Put(ctx context.Context, path string, reader io.Reader) error

	// PutWithOptions stores a file with additional options
	PutWithOptions(ctx context.Context, path string, reader io.Reader, opts *PutOptions) error

	// Get retrieves a file
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes a file
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, path string) (bool, error)

	// URL generates a URL for accessing the file
	URL(ctx context.Context, path string) (string, error)

	// TemporaryURL generates a temporary signed URL
	TemporaryURL(ctx context.Context, path string, expiration time.Duration) (string, error)

	// Size returns the file size in bytes
	Size(ctx context.Context, path string) (int64, error)

	// LastModified returns the last modification time
	LastModified(ctx context.Context, path string) (time.Time, error)

	// List lists files in a directory
	List(ctx context.Context, directory string) ([]string, error)

	// Copy copies a file
	Copy(ctx context.Context, src, dst string) error

	// Move moves a file
	Move(ctx context.Context, src, dst string) error
}

// PutOptions contains options for putting files
type PutOptions struct {
	// ContentType sets the MIME type
	ContentType string

	// Metadata contains custom metadata
	Metadata map[string]string

	// CacheControl sets cache control header
	CacheControl string

	// ACL sets access control (e.g., "public-read", "private")
	ACL string
}

// FileInfo contains information about a stored file
type FileInfo struct {
	Path         string
	Size         int64
	ContentType  string
	LastModified time.Time
	ETag         string
	Metadata     map[string]string
}
