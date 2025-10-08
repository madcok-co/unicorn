// Redis cache plugin
package plugins

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type CachePlugin struct {
	client     *redis.Client
	defaultTTL time.Duration
	keyPrefix  string
}

func NewCachePlugin(config map[string]interface{}) (*CachePlugin, error)
func (p *CachePlugin) Set(ctx context.Context, key string, value interface{}) error
func (p *CachePlugin) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error
func (p *CachePlugin) Get(ctx context.Context, key string, dest interface{}) error
func (p *CachePlugin) Delete(ctx context.Context, keys ...string) error
func (p *CachePlugin) Exists(ctx context.Context, key string) (bool, error)
