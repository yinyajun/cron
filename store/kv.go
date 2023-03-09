package store

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type KV interface {
	Set(key string, content []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

type redisKV struct {
	cli *redis.Client
}

func NewRedisKV(cli *redis.Client) KV { return redisKV{cli: cli} }

func (r redisKV) Set(key string, value []byte) error {
	return r.cli.Set(context.Background(), key, value, 0).Err()
}

func (r redisKV) Get(key string) ([]byte, error) {
	res, err := r.cli.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func (r redisKV) Delete(key string) error {
	return r.cli.Del(context.Background(), key).Err()
}
