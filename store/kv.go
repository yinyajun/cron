package store

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type KV interface {
	SetEx(key string, value interface{}, expiration time.Duration) error
	Get(key string) (string, error)
	MSet(values ...interface{}) error
	MGet(keys ...string) ([]interface{}, error)
	Del(key string) error
}

type redisKV struct {
	cli *redis.Client
}

func NewRedisKV(cli *redis.Client) KV { return redisKV{cli: cli} }

func (r redisKV) SetEx(key string, value interface{}, expiration time.Duration) error {
	return r.cli.Set(context.Background(), key, value, expiration).Err()
}

func (r redisKV) Get(key string) (string, error) {
	res, err := r.cli.Get(context.Background(), key).Result()
	if err != nil {
		return "", err
	}
	return res, nil
}

func (r redisKV) MGet(keys ...string) ([]interface{}, error) {
	res, err := r.cli.MGet(context.Background(), keys...).Result()
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r redisKV) MSet(values ...interface{}) error {
	return r.cli.MSet(context.Background(), values).Err()
}

func (r redisKV) Del(key string) error {
	return r.cli.Del(context.Background(), key).Err()
}
