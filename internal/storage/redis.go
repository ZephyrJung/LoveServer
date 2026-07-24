package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/ZephyrJung/LoveServer/internal/config"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(ctx context.Context, cfg config.RedisConfig) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) Client() *redis.Client {
	return s.client
}