package memory

import (
	"context"
	"time"
)

// Memory interface for agent memory/storage
type Memory interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (interface{}, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetTTL(ttl time.Duration)
}
