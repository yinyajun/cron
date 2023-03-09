package store

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type List interface {
	RPush(key, member string) error
	LPush(key, member string) error
	RPop(key string) (string, error)
	LPop(key string) (string, error)
	LLen(key string) (int64, error)
	LRange(key string) ([]string, error)
}

func NewRedisList(cli *redis.Client) List { return redisKV{cli: cli} }

func (r redisKV) RPush(key, member string) error {
	return r.cli.RPush(context.Background(), key, member).Err()
}

func (r redisKV) LPush(key, member string) error {
	return r.cli.LPush(context.Background(), key, member).Err()
}

func (r redisKV) RPop(key string) (string, error) {
	res, err := r.cli.RPop(context.Background(), key).Result()
	if err != nil {
		return "", err
	}
	return res, nil
}

func (r redisKV) LPop(key string) (string, error) {
	res, err := r.cli.LPop(context.Background(), key).Result()
	if err != nil {
		return "", err
	}
	return res, nil
}

func (r redisKV) LLen(key string) (int64, error) {
	res, err := r.cli.LLen(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}
	return res, nil
}

func (r redisKV) LRange(key string) ([]string, error) {
	size, err := r.LLen(key)
	if err != nil {
		return nil, err
	}
	res, err := r.cli.LRange(context.Background(), key, 0, size-1).Result()
	if err != nil {
		return nil, err
	}
	return res, nil
}
