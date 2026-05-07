// internal/adapter/outbound/redis/connection.go
package redis

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context) (*redis.Client, error) {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = "6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port),
	})

	// Ping to verify connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return rdb, nil
}
