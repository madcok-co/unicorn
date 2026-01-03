package middleware

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcok-co/unicorn/core/pkg/context"
)

// UploadedFile represents an uploaded file
type UploadedFile struct {
	Filename    string
	Size        int64
	ContentType string
	Header      *multipart.FileHeader
	File        multipart.File
}

// UploadConfig configures file upload middleware
type UploadConfig struct {
	// MaxFileSize is the maximum file size in bytes (default: 10MB)
	MaxFileSize int64

	// MaxMemory is the maximum memory to use for parsing (default: 32MB)
	MaxMemory int64

	// AllowedExtensions is a list of allowed file extensions (e.g., [".jpg", ".png"])
	// If empty, all extensions are allowed
	AllowedExtensions []string

	// AllowedMimeTypes is a list of allowed MIME types (e.g., ["image/jpeg", "image/png"])
	// If empty, all MIME types are allowed
	AllowedMimeTypes []string

	// FormField is the name of the form field containing the file (default: "file")
	FormField string

	// AllowMultiple allows multiple file uploads
	AllowMultiple bool

	// ErrorHandler is called when validation fails
	ErrorHandler func(*context.Context, error) error
}

// DefaultUploadConfig returns default upload configuration
func DefaultUploadConfig() *UploadConfig {
	return &UploadConfig{
		MaxFileSize:       10 * 1024 * 1024, // 10MB
		MaxMemory:         32 * 1024 * 1024, // 32MB
		AllowedExtensions: nil,              // Allow all
		AllowedMimeTypes:  nil,              // Allow all
		FormField:         "file",
		AllowMultiple:     false,
		ErrorHandler:      defaultUploadErrorHandler,
	}
}

// defaultUploadErrorHandler is the default error handler
func defaultUploadErrorHandler(ctx *context.Context, err error) error {
	return ctx.Error(http.StatusBadRequest, err.Error())
}

// Upload returns a middleware that handles file uploads
func Upload() context.MiddlewareFunc {
	return UploadWithConfig(DefaultUploadConfig())
}

// UploadWithConfig returns a middleware with custom configuration
func UploadWithConfig(config *UploadConfig) context.MiddlewareFunc {
	if config == nil {
		config = DefaultUploadConfig()
	}

	// Apply defaults
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 10 * 1024 * 1024
	}
	if config.MaxMemory == 0 {
		config.MaxMemory = 32 * 1024 * 1024
	}
	if config.FormField == "" {
		config.FormField = "file"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = defaultUploadErrorHandler
	}

	// Build extension map for fast lookup
	extMap := make(map[string]bool)
	for _, ext := range config.AllowedExtensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[strings.ToLower(ext)] = true
	}

	// Build MIME type map for fast lookup
	mimeMap := make(map[string]bool)
	for _, mime := range config.AllowedMimeTypes {
		mimeMap[strings.ToLower(mime)] = true
	}

	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			// Skip if not a multipart form request
			contentType := ctx.Request().Header("Content-Type")
			if !strings.HasPrefix(contentType, "multipart/form-data") {
				return next(ctx)
			}

			// Create a mock http.Request for multipart parsing
			// In a real implementation, this would come from the HTTP adapter
			req, ok := ctx.Get("http.Request")
			if !ok || req == nil {
				// If no raw request, skip middleware
				return next(ctx)
			}

			httpReq, ok := req.(*http.Request)
			if !ok {
				return next(ctx)
			}

			// Parse multipart form
			if err := httpReq.ParseMultipartForm(config.MaxMemory); err != nil {
				return config.ErrorHandler(ctx, fmt.Errorf("failed to parse multipart form: %w", err))
			}

			if httpReq.MultipartForm == nil {
				return config.ErrorHandler(ctx, fmt.Errorf("no multipart form data"))
			}

			// Get files from the specified form field
			files := httpReq.MultipartForm.File[config.FormField]
			if len(files) == 0 {
				// No files uploaded, continue to handler
				return next(ctx)
			}

			// Check if multiple files are allowed
			if !config.AllowMultiple && len(files) > 1 {
				return config.ErrorHandler(ctx, fmt.Errorf("multiple files not allowed"))
			}

			// Process files
			uploadedFiles := make([]*UploadedFile, 0, len(files))
			for _, fileHeader := range files {
				// Validate file size
				if fileHeader.Size > config.MaxFileSize {
					return config.ErrorHandler(ctx, fmt.Errorf("file %s exceeds maximum size of %d bytes", fileHeader.Filename, config.MaxFileSize))
				}

				// Validate file extension
				if len(extMap) > 0 {
					ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
					if !extMap[ext] {
						return config.ErrorHandler(ctx, fmt.Errorf("file extension %s not allowed", ext))
					}
				}

				// Open file to validate MIME type
				file, err := fileHeader.Open()
				if err != nil {
					return config.ErrorHandler(ctx, fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err))
				}

				// Detect MIME type from file content
				if len(mimeMap) > 0 {
					buffer := make([]byte, 512)
					n, err := file.Read(buffer)
					if err != nil && err != io.EOF {
						file.Close()
						return config.ErrorHandler(ctx, fmt.Errorf("failed to read file %s: %w", fileHeader.Filename, err))
					}

					mimeType := http.DetectContentType(buffer[:n])
					if !mimeMap[strings.ToLower(mimeType)] {
						file.Close()
						return config.ErrorHandler(ctx, fmt.Errorf("file MIME type %s not allowed", mimeType))
					}

					// Reset file pointer to beginning
					if seeker, ok := file.(io.Seeker); ok {
						seeker.Seek(0, 0)
					}
				}

				uploadedFiles = append(uploadedFiles, &UploadedFile{
					Filename:    fileHeader.Filename,
					Size:        fileHeader.Size,
					ContentType: fileHeader.Header.Get("Content-Type"),
					Header:      fileHeader,
					File:        file,
				})
			}

			// Store uploaded files in context
			if config.AllowMultiple {
				ctx.Set("uploaded_files", uploadedFiles)
			} else if len(uploadedFiles) > 0 {
				ctx.Set("uploaded_file", uploadedFiles[0])
			}

			// Continue to next handler
			err := next(ctx)

			// Cleanup: close all files
			for _, uf := range uploadedFiles {
				if uf.File != nil {
					uf.File.Close()
				}
			}

			return err
		}
	}
}

// GetUploadedFile retrieves a single uploaded file from context
func GetUploadedFile(ctx *context.Context) (*UploadedFile, bool) {
	file, ok := ctx.Get("uploaded_file")
	if !ok {
		return nil, false
	}
	uploadedFile, ok := file.(*UploadedFile)
	return uploadedFile, ok
}

// GetUploadedFiles retrieves multiple uploaded files from context
func GetUploadedFiles(ctx *context.Context) ([]*UploadedFile, bool) {
	files, ok := ctx.Get("uploaded_files")
	if !ok {
		return nil, false
	}
	uploadedFiles, ok := files.([]*UploadedFile)
	return uploadedFiles, ok
}

// SaveUploadedFile saves an uploaded file to a destination path
func SaveUploadedFile(file *UploadedFile, dst string) error {
	if file == nil || file.File == nil {
		return fmt.Errorf("invalid file")
	}

	// Reset file pointer if possible
	if seeker, ok := file.File.(io.Seeker); ok {
		seeker.Seek(0, 0)
	}

	// Create destination file
	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	// Copy data
	if _, err := io.Copy(outFile, file.File); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	return nil
}

// SaveUploadedFileWithWriter saves using a custom writer function
// The writerFunc receives the reader and should handle writing
func SaveUploadedFileWithWriter(file *UploadedFile, writerFunc func(io.Reader) error) error {
	if file == nil || file.File == nil {
		return fmt.Errorf("invalid file")
	}

	// Reset file pointer if possible
	if seeker, ok := file.File.(io.Seeker); ok {
		seeker.Seek(0, 0)
	}

	return writerFunc(file.File)
}

// UploadImage returns a middleware configured for image uploads
func UploadImage() context.MiddlewareFunc {
	return UploadWithConfig(&UploadConfig{
		MaxFileSize: 5 * 1024 * 1024, // 5MB
		MaxMemory:   32 * 1024 * 1024,
		AllowedExtensions: []string{
			".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg",
		},
		AllowedMimeTypes: []string{
			"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml",
		},
		FormField:     "image",
		AllowMultiple: false,
		ErrorHandler:  defaultUploadErrorHandler,
	})
}

// UploadDocument returns a middleware configured for document uploads
func UploadDocument() context.MiddlewareFunc {
	return UploadWithConfig(&UploadConfig{
		MaxFileSize: 20 * 1024 * 1024, // 20MB
		MaxMemory:   32 * 1024 * 1024,
		AllowedExtensions: []string{
			".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".csv",
		},
		AllowedMimeTypes: []string{
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.ms-powerpoint",
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"text/plain",
			"text/csv",
		},
		FormField:     "document",
		AllowMultiple: false,
		ErrorHandler:  defaultUploadErrorHandler,
	})
}

// UploadMultiple returns a middleware that allows multiple file uploads
func UploadMultiple(maxFiles int) context.MiddlewareFunc {
	config := DefaultUploadConfig()
	config.AllowMultiple = true

	// Wrap the error handler to check max files
	originalErrorHandler := config.ErrorHandler
	config.ErrorHandler = func(ctx *context.Context, err error) error {
		files, ok := GetUploadedFiles(ctx)
		if ok && maxFiles > 0 && len(files) > maxFiles {
			return originalErrorHandler(ctx, fmt.Errorf("maximum %d files allowed", maxFiles))
		}
		return originalErrorHandler(ctx, err)
	}

	return UploadWithConfig(config)
}
