# File Upload Example

This example demonstrates how to implement file upload functionality in Unicorn framework with:

- ✅ Single file upload
- ✅ Multiple file upload
- ✅ File type validation (images, documents)
- ✅ File size validation
- ✅ MIME type validation
- ✅ Local file storage
- ✅ File management (list, download, delete)

## Features

### 1. Upload Middleware

The framework provides flexible upload middleware with:

- **Size limits** - Configure maximum file size
- **Type validation** - Whitelist allowed extensions and MIME types
- **Security** - Automatic validation and sanitization
- **Multiple uploads** - Support for batch uploads

### 2. Storage Adapter

Local filesystem storage with:

- **Directory isolation** - Prevent path traversal attacks
- **Automatic directory creation** - Organize by date
- **URL generation** - Easy file access
- **File operations** - Copy, move, delete, list

### 3. Specialized Middleware

Pre-configured middleware for common use cases:

- `UploadImage()` - Images only (jpg, png, gif, webp, svg)
- `UploadDocument()` - Documents only (pdf, doc, xls, etc)
- `UploadMultiple(n)` - Multiple files with limit

## Quick Start

### 1. Run the Example

```bash
cd core/examples/file-upload
go run main.go
```

The server will start on `http://localhost:8080`

### 2. Upload a File

```bash
# Upload any file
curl -X POST http://localhost:8080/upload \
  -F 'file=@document.pdf'

# Upload an image
curl -X POST http://localhost:8080/upload/image \
  -F 'image=@photo.jpg'

# Upload a document
curl -X POST http://localhost:8080/upload/document \
  -F 'document=@report.pdf'

# Upload multiple files
curl -X POST http://localhost:8080/upload/multiple \
  -F 'file=@file1.pdf' \
  -F 'file=@file2.jpg' \
  -F 'file=@file3.doc'
```

### 3. List Files

```bash
curl http://localhost:8080/files
```

### 4. Download a File

```bash
curl http://localhost:8080/files/uploads/2024/01/document_20240115_120000.pdf \
  -o downloaded.pdf
```

### 5. Delete a File

```bash
curl -X DELETE http://localhost:8080/files/uploads/2024/01/document_20240115_120000.pdf
```

## API Endpoints

### Upload Endpoints

| Method | Endpoint | Description | Max Size | Allowed Types |
|--------|----------|-------------|----------|---------------|
| POST | `/upload` | Upload any file | 10MB | All |
| POST | `/upload/image` | Upload image | 5MB | jpg, png, gif, webp, svg |
| POST | `/upload/document` | Upload document | 20MB | pdf, doc, xls, txt, csv |
| POST | `/upload/multiple` | Upload multiple files | 10MB each | All (max 10 files) |

### File Management Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/files` | List all uploaded files |
| GET | `/files/:path` | Download a file |
| DELETE | `/files/:path` | Delete a file |

## Response Format

### Upload Response

```json
{
  "success": true,
  "message": "File uploaded successfully",
  "filename": "document_20240115_120000.pdf",
  "path": "uploads/2024/01/document_20240115_120000.pdf",
  "size": 524288,
  "url": "http://localhost:8080/files/uploads/2024/01/document_20240115_120000.pdf"
}
```

### Multiple Upload Response

```json
{
  "success": true,
  "message": "3 files uploaded successfully",
  "count": 3,
  "files": [
    {
      "filename": "file1_20240115_120000.pdf",
      "path": "uploads/2024/01/file1_20240115_120000.pdf",
      "size": 524288,
      "content_type": "application/pdf",
      "uploaded_at": "2024-01-15T12:00:00Z",
      "url": "http://localhost:8080/files/uploads/2024/01/file1_20240115_120000.pdf"
    }
  ]
}
```

## Configuration

### Environment Variables

```bash
# HTTP server
HTTP_HOST=0.0.0.0
HTTP_PORT=8080

# Storage
STORAGE_PATH=./storage
BASE_URL=http://localhost:8080/files
```

### Custom Upload Configuration

```go
// Custom upload middleware
uploadMiddleware := middleware.UploadWithConfig(&middleware.UploadConfig{
    MaxFileSize: 50 * 1024 * 1024, // 50MB
    MaxMemory:   64 * 1024 * 1024, // 64MB
    AllowedExtensions: []string{".pdf", ".jpg", ".png"},
    AllowedMimeTypes: []string{"application/pdf", "image/jpeg", "image/png"},
    FormField: "file",
    AllowMultiple: false,
    ErrorHandler: func(ctx *context.Context, err error) error {
        return ctx.Error(400, fmt.Sprintf("Upload error: %v", err))
    },
})

application.RegisterHandler(YourHandler).
    HTTP("POST", "/custom-upload").
    Middleware(uploadMiddleware).
    Done()
```

## Implementation Guide

### 1. Create a Basic Upload Handler

```go
func UploadFile(ctx *context.Context) (*UploadResponse, error) {
    // Get uploaded file from context (set by middleware)
    uploadedFile, ok := middleware.GetUploadedFile(ctx)
    if !ok {
        return &UploadResponse{
            Success: false,
            Message: "No file uploaded",
        }, nil
    }

    // Get storage service
    storage := ctx.GetService("storage").(contracts.Storage)

    // Save file
    err := storage.Put(ctx.Context(), "path/to/file", uploadedFile.File)
    if err != nil {
        return nil, err
    }

    return &UploadResponse{
        Success: true,
        Message: "File uploaded successfully",
    }, nil
}
```

### 2. Register with Upload Middleware

```go
application.RegisterHandler(UploadFile).
    Named("upload").
    HTTP("POST", "/upload").
    Middleware(middleware.Upload()). // Apply upload middleware
    Done()
```

### 3. Setup Storage Service

```go
// Create local storage
storageService, err := storage.NewLocalStorage(&storage.LocalStorageConfig{
    BasePath:   "./storage",
    BaseURL:    "http://localhost:8080/files",
    CreateDirs: true,
})

// Register as service
application.RegisterService("storage", storageService)
```

## Security Features

### 1. File Type Validation

The middleware validates files by:

- **Extension check** - Whitelist allowed extensions
- **MIME type detection** - Verify actual file content
- **Double validation** - Both extension and content must match

### 2. Size Limits

- **Per-file limit** - Maximum size per file
- **Memory limit** - Maximum memory for parsing
- **Total size** - Can limit total upload size

### 3. Path Traversal Prevention

The storage adapter prevents:

- Absolute paths
- Directory traversal (`../`)
- Paths outside base directory

### 4. Automatic Sanitization

- Filename sanitization
- Unique filenames with timestamps
- Safe path construction

## Storage Backends

### Local Storage (Built-in)

```go
storage.NewLocalStorage(&storage.LocalStorageConfig{
    BasePath:   "./storage",
    BaseURL:    "http://localhost:8080/files",
    CreateDirs: true,
})
```

### Cloud Storage (Future)

The `contracts.Storage` interface supports:

- AWS S3
- Google Cloud Storage
- Azure Blob Storage
- MinIO
- Custom implementations

## Testing

### Test Upload with cURL

```bash
# Create a test file
echo "Hello World" > test.txt

# Upload it
curl -X POST http://localhost:8080/upload \
  -F 'file=@test.txt' \
  -v

# Check response
# Should return JSON with file info
```

### Test File Type Validation

```bash
# Try uploading non-image to image endpoint (should fail)
curl -X POST http://localhost:8080/upload/image \
  -F 'image=@document.pdf'

# Try uploading image to image endpoint (should succeed)
curl -X POST http://localhost:8080/upload/image \
  -F 'image=@photo.jpg'
```

### Test Size Limits

```bash
# Create large file
dd if=/dev/zero of=large.bin bs=1M count=50

# Try uploading (should fail if > max size)
curl -X POST http://localhost:8080/upload \
  -F 'file=@large.bin'
```

## Advanced Usage

### Custom Storage Implementation

```go
type MyCustomStorage struct {
    // Your fields
}

func (s *MyCustomStorage) Put(ctx context.Context, path string, reader io.Reader) error {
    // Your implementation
}

// Implement all other contracts.Storage methods
```

### With Image Processing

```go
func UploadAndProcessImage(ctx *context.Context) error {
    uploadedFile, _ := middleware.GetUploadedFile(ctx)
    
    // Process image (resize, compress, etc)
    processed := processImage(uploadedFile.File)
    
    // Save processed image
    storage := ctx.GetService("storage").(contracts.Storage)
    storage.Put(ctx.Context(), "processed/image.jpg", processed)
    
    return ctx.Success("Image uploaded and processed")
}
```

### With Database Tracking

```go
type FileRecord struct {
    ID          string
    Filename    string
    Path        string
    Size        int64
    ContentType string
    UserID      string
    CreatedAt   time.Time
}

func UploadWithTracking(ctx *context.Context) error {
    uploadedFile, _ := middleware.GetUploadedFile(ctx)
    storage := ctx.GetService("storage").(contracts.Storage)
    db := ctx.DB()
    
    // Save file
    path := "uploads/" + uploadedFile.Filename
    storage.Put(ctx.Context(), path, uploadedFile.File)
    
    // Save metadata to database
    record := &FileRecord{
        ID:          generateID(),
        Filename:    uploadedFile.Filename,
        Path:        path,
        Size:        uploadedFile.Size,
        ContentType: uploadedFile.ContentType,
        UserID:      ctx.Identity().ID,
        CreatedAt:   time.Now(),
    }
    
    db.Create(record)
    
    return ctx.Success(record)
}
```

## Best Practices

1. **Always validate files** - Use middleware validation
2. **Use unique filenames** - Prevent overwrites
3. **Organize by date** - Easy to manage
4. **Set appropriate limits** - Don't allow huge files
5. **Clean up failed uploads** - Delete partial files
6. **Use storage abstraction** - Easy to switch backends
7. **Generate temporary URLs** - For secure access
8. **Implement file cleanup** - Remove old files
9. **Log all operations** - For debugging and audit
10. **Handle errors gracefully** - User-friendly messages

## Troubleshooting

### Issue: "No file uploaded"

**Solution**: Ensure the form field name matches the middleware configuration:

```bash
# If using default middleware (field name: "file")
curl -F 'file=@test.pdf' ...

# If using UploadImage() (field name: "image")
curl -F 'image=@test.jpg' ...
```

### Issue: "File too large"

**Solution**: Adjust `MaxFileSize` in middleware configuration.

### Issue: "File type not allowed"

**Solution**: Check `AllowedExtensions` and `AllowedMimeTypes` in middleware config.

### Issue: "Path outside base directory"

**Solution**: Don't use absolute paths or `../` in file paths.

## Next Steps

- Add cloud storage support (S3, GCS, etc)
- Implement image processing (resize, compress)
- Add virus scanning integration
- Implement chunked uploads for large files
- Add progress tracking
- Implement file encryption

## License

MIT
