package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	rdb *redis.Client
}

// OpenRedis hace ping al arrancar
func OpenRedis(ctx context.Context, addr string) (*Redis, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("cache: conectar a Redis en %s: %w", addr, err)
	}
	return &Redis{rdb: rdb}, nil
}

func (r *Redis) Close() error { return r.rdb.Close() }

func (*Redis) Name() string { return "Redis" }

func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, err := r.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache: GET %s: %w", key, err)
	}
	return value, true, nil
}

func (r *Redis) Set(ctx context.Context, key string, value []byte) error {
	if err := r.rdb.Set(ctx, key, value, 0).Err(); err != nil {
		return fmt.Errorf("cache: SET %s: %w", key, err)
	}
	return nil
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	if err := r.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache: DEL: %w", err)
	}
	return nil
}

func (r *Redis) Flush(ctx context.Context) error {
	if err := r.rdb.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("cache: FLUSHDB: %w", err)
	}
	return nil
}
