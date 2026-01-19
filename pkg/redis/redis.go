package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Config holds the Redis configuration parameters
type Config struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// NewRedisClient initializes a new Redis client
func NewRedisClient(cfg Config) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return rdb, nil
}
