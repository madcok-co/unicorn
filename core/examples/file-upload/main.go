package main

import (
	"bytes"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
	"github.com/madcok-co/unicorn/core/pkg/adapters/storage"
	"github.com/madcok-co/unicorn/core/pkg/app"
	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ============================================================
// MODELS & DTOs
// ============================================================

type UploadResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
	Size     int64  `json:"size,omitempty"`
	URL      string `json:"url,omitempty"`
}

type FileInfo struct {
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url,omitempty"`
}

type MultipleUploadResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Files   []*FileInfo `json:"files"`
	Count   int         `json:"count"`
}

// ============================================================
// FILE UPLOAD HELPERS
// ============================================================

// ValidateFileSize validates file size
func ValidateFileSize(size int64, maxSize int64) error {
	if size > maxSize {
		return fmt.Errorf("file size %d exceeds maximum of %d bytes", size, maxSize)
	}
	return nil
}

// ValidateFileExtension validates file extension
func ValidateFileExtension(filename string, allowedExts []string) error {
	if len(allowedExts) == 0 {
		return nil // No restriction
	}

	ext := strings.ToLower(filepath.Ext(filename))
	for _, allowed := range allowedExts {
		if !strings.HasPrefix(allowed, ".") {
			allowed = "." + allowed
		}
		if ext == strings.ToLower(allowed) {
			return nil
		}
	}

	return fmt.Errorf("file extension %s not allowed", ext)
}

// GetMultipartFile retrieves file from multipart form
func GetMultipartFile(ctx *ucontext.Context, fieldName string) (multipart.File, *multipart.FileHeader, error) {
	// Get raw HTTP request from context
	rawReq, ok := ctx.Get("http.Request")
	if !ok {
		return nil, nil, fmt.Errorf("HTTP request not available")
	}

	httpReq, ok := rawReq.(*http.Request)
	if !ok {
		return nil, nil, fmt.Errorf("invalid HTTP request type")
	}

	// Parse multipart form
	if err := httpReq.ParseMultipartForm(32 << 20); err != nil { // 32MB
		return nil, nil, fmt.Errorf("failed to parse multipart form: %w", err)
	}

	// Get file from form
	file, header, err := httpReq.FormFile(fieldName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file: %w", err)
	}

	return file, header, nil
}

// ============================================================
// UPLOAD HANDLERS
// ============================================================

// UploadFile handles single file upload
func UploadFile(ctx *ucontext.Context) (*UploadResponse, error) {
	logger := ctx.Logger()

	// Get file from request
	file, header, err := GetMultipartFile(ctx, "file")
	if err != nil {
		logger.Error("failed to get file from request", "error", err)
		return &UploadResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	defer file.Close()

	// Validate file size (10MB max)
	if err := ValidateFileSize(header.Size, 10*1024*1024); err != nil {
		return &UploadResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	logger.Info("file upload started",
		"filename", header.Filename,
		"size", header.Size)

	// Get storage service
	storageService := ctx.GetService("storage").(contracts.Storage)

	// Generate unique filename
	timestamp := time.Now().Format("20060102_150405")
	ext := filepath.Ext(header.Filename)
	baseFilename := header.Filename[:len(header.Filename)-len(ext)]
	uniqueFilename := fmt.Sprintf("%s_%s%s", baseFilename, timestamp, ext)
	storagePath := filepath.Join("uploads", time.Now().Format("2006/01"), uniqueFilename)

	// Save file to storage
	if err := storageService.Put(ctx.Context(), storagePath, file); err != nil {
		logger.Error("failed to save file", "error", err, "path", storagePath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Generate URL
	fileURL, _ := storageService.URL(ctx.Context(), storagePath)

	logger.Info("file uploaded successfully",
		"filename", uniqueFilename,
		"path", storagePath,
		"size", header.Size)

	return &UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		Filename: uniqueFilename,
		Path:     storagePath,
		Size:     header.Size,
		URL:      fileURL,
	}, nil
}

// UploadImage handles image upload with validation
func UploadImage(ctx *ucontext.Context) (*UploadResponse, error) {
	logger := ctx.Logger()

	// Get file from request
	file, header, err := GetMultipartFile(ctx, "image")
	if err != nil {
		logger.Error("failed to get image", "error", err)
		return &UploadResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	defer file.Close()

	// Validate file size (5MB max for images)
	if err := ValidateFileSize(header.Size, 5*1024*1024); err != nil {
		return &UploadResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Validate file extension
	allowedExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	if err := ValidateFileExtension(header.Filename, allowedExts); err != nil {
		return &UploadResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	logger.Info("image upload started",
		"filename", header.Filename,
		"size", header.Size)

	// Get storage service
	storageService := ctx.GetService("storage").(contracts.Storage)

	// Generate unique filename for image
	timestamp := time.Now().Format("20060102_150405")
	ext := filepath.Ext(header.Filename)
	uniqueFilename := fmt.Sprintf("image_%s%s", timestamp, ext)
	storagePath := filepath.Join("images", time.Now().Format("2006/01"), uniqueFilename)

	// Save image
	if err := storageService.Put(ctx.Context(), storagePath, file); err != nil {
		logger.Error("failed to save image", "error", err)
		return nil, fmt.Errorf("failed to save image: %w", err)
	}

	// Generate URL
	imageURL, _ := storageService.URL(ctx.Context(), storagePath)

	logger.Info("image uploaded successfully",
		"filename", uniqueFilename,
		"path", storagePath,
		"url", imageURL)

	return &UploadResponse{
		Success:  true,
		Message:  "Image uploaded successfully",
		Filename: uniqueFilename,
		Path:     storagePath,
		Size:     header.Size,
		URL:      imageURL,
	}, nil
}

// GetFile retrieves a file from storage
func GetFile(ctx *ucontext.Context) error {
	logger := ctx.Logger()

	// Get file path from URL parameter
	filePath := ctx.Request().Param("path")
	if filePath == "" {
		return ctx.Error(404, "File path required")
	}

	logger.Info("file download requested", "path", filePath)

	storageService := ctx.GetService("storage").(contracts.Storage)

	// Check if file exists
	exists, err := storageService.Exists(ctx.Context(), filePath)
	if err != nil {
		logger.Error("failed to check file existence", "error", err)
		return ctx.Error(500, "Internal server error")
	}
	if !exists {
		logger.Warn("file not found", "path", filePath)
		return ctx.Error(404, "File not found")
	}

	// Get file
	reader, err := storageService.Get(ctx.Context(), filePath)
	if err != nil {
		logger.Error("failed to get file", "error", err)
		return ctx.Error(500, "Failed to retrieve file")
	}
	defer reader.Close()

	// Read file content
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		logger.Error("failed to read file", "error", err)
		return ctx.Error(500, "Failed to read file")
	}

	// Set response headers
	ctx.Response().SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))
	ctx.Response().SetHeader("Content-Type", "application/octet-stream")
	ctx.Response().StatusCode = 200
	ctx.Response().Body = buf.Bytes()

	logger.Info("file downloaded", "path", filePath, "size", buf.Len())

	return nil
}

// ListFiles lists all uploaded files
func ListFiles(ctx *ucontext.Context) (map[string]interface{}, error) {
	logger := ctx.Logger()
	storageService := ctx.GetService("storage").(contracts.Storage)

	// List files in uploads directory
	files, err := storageService.List(ctx.Context(), "uploads")
	if err != nil {
		logger.Error("failed to list files", "error", err)
		// Return empty list if directory doesn't exist
		return map[string]interface{}{
			"files": []string{},
			"count": 0,
		}, nil
	}

	logger.Info("files listed", "count", len(files))

	return map[string]interface{}{
		"files": files,
		"count": len(files),
	}, nil
}

// DeleteFile deletes a file from storage
func DeleteFile(ctx *ucontext.Context) (*UploadResponse, error) {
	logger := ctx.Logger()

	filePath := ctx.Request().Param("path")
	if filePath == "" {
		return &UploadResponse{
			Success: false,
			Message: "File path required",
		}, nil
	}

	storageService := ctx.GetService("storage").(contracts.Storage)

	// Check if file exists
	exists, err := storageService.Exists(ctx.Context(), filePath)
	if err != nil {
		logger.Error("failed to check file existence", "error", err)
		return nil, fmt.Errorf("failed to check file: %w", err)
	}
	if !exists {
		return &UploadResponse{
			Success: false,
			Message: "File not found",
		}, nil
	}

	// Delete file
	if err := storageService.Delete(ctx.Context(), filePath); err != nil {
		logger.Error("failed to delete file", "error", err, "path", filePath)
		return nil, fmt.Errorf("failed to delete file: %w", err)
	}

	logger.Info("file deleted", "path", filePath)

	return &UploadResponse{
		Success: true,
		Message: "File deleted successfully",
		Path:    filePath,
	}, nil
}

// ============================================================
// MAIN APPLICATION
// ============================================================

func main() {
	// Get storage path from environment or use default
	storagePath := getEnv("STORAGE_PATH", "./storage")
	baseURL := getEnv("BASE_URL", "http://localhost:8080/files")

	// Create application
	application := app.New(&app.Config{
		Name:       "file-upload-example",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: getEnv("HTTP_HOST", "0.0.0.0"),
			Port: getEnvInt("HTTP_PORT", 8080),
		},
	})

	// Setup logger
	appLogger := loggerAdapter.NewConsoleLogger("info")
	application.SetLogger(appLogger)

	// Setup local storage
	storageService, err := storage.NewLocalStorage(&storage.LocalStorageConfig{
		BasePath:   storagePath,
		BaseURL:    baseURL,
		CreateDirs: true,
	})
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Register storage as a custom service
	application.RegisterService("storage", storageService)

	// ============================================================
	// REGISTER ROUTES
	// ============================================================

	// File upload endpoints
	application.RegisterHandler(UploadFile).
		Named("upload-file").
		HTTP("POST", "/upload").
		Done()

	application.RegisterHandler(UploadImage).
		Named("upload-image").
		HTTP("POST", "/upload/image").
		Done()

	// File management
	application.RegisterHandler(GetFile).
		Named("get-file").
		HTTP("GET", "/files/*path").
		Done()

	application.RegisterHandler(ListFiles).
		Named("list-files").
		HTTP("GET", "/files").
		Done()

	application.RegisterHandler(DeleteFile).
		Named("delete-file").
		HTTP("DELETE", "/files/*path").
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("🚀 File Upload Example Started!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  File Upload:")
		fmt.Println("    POST   /upload              - Upload single file (any type)")
		fmt.Println("    POST   /upload/image        - Upload image (jpg, png, gif, webp)")
		fmt.Println("\n  File Management:")
		fmt.Println("    GET    /files               - List all uploaded files")
		fmt.Println("    GET    /files/:path         - Download/view file")
		fmt.Println("    DELETE /files/:path         - Delete file")
		fmt.Println("\n📁 Storage:")
		fmt.Printf("  Path: %s\n", storagePath)
		fmt.Printf("  URL:  %s\n", baseURL)
		fmt.Println("\n📝 Example Usage:")
		fmt.Println("  # Upload a file")
		fmt.Println("  curl -X POST http://localhost:8080/upload \\")
		fmt.Println("    -F 'file=@/path/to/file.pdf'")
		fmt.Println("\n  # Upload an image")
		fmt.Println("  curl -X POST http://localhost:8080/upload/image \\")
		fmt.Println("    -F 'image=@/path/to/photo.jpg'")
		fmt.Println("\n  # List files")
		fmt.Println("  curl http://localhost:8080/files")
		fmt.Println("\n  # Delete file")
		fmt.Println("  curl -X DELETE http://localhost:8080/files/uploads/2024/01/example.pdf")
		fmt.Println()

		return nil
	})

	// Start application
	log.Printf("Starting on http://%s:%d\n",
		getEnv("HTTP_HOST", "0.0.0.0"),
		getEnvInt("HTTP_PORT", 8080))

	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

// ============================================================
// HELPER FUNCTIONS
// ============================================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		fmt.Sscanf(value, "%d", &intVal)
		return intVal
	}
	return defaultValue
}
