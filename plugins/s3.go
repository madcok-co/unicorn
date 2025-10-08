// AWS S3 storage plugin
package plugins

import (
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Plugin struct {
	client *s3.S3
	bucket string
	region string
}

func NewS3Plugin(config map[string]interface{}) (*S3Plugin, error)
func (p *S3Plugin) UploadFile(key string, data []byte, contentType string) (string, error)
func (p *S3Plugin) DownloadFile(key string) ([]byte, error)
func (p *S3Plugin) DeleteFile(key string) error
func (p *S3Plugin) GetFileURL(key string, expires time.Duration) (string, error)
