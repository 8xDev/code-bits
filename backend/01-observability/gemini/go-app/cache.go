package main

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

// InitRedis initializes the Redis client
func InitRedis(ctx context.Context, addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Ping to check connection
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return rdb, nil

}
