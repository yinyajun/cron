package store

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type Set interface {
	SAdd(key, member string) error
	SRem(key string, members ...string) error
	SLen(key string) (int64, error)
	SRange(key string) ([]string, error)
}

func NewRedisSet(cli *redis.Client) Set { return redisKV{cli: cli} }

func (r redisKV) SAdd(key, member string) error {
	return r.cli.SAdd(context.Background(), key, member).Err()
}

func (r redisKV) SRem(key string, members ...string) error {
	return r.cli.SRem(context.Background(), key, members).Err()
}

func (r redisKV) SLen(key string) (int64, error) {
	res, err := r.cli.SCard(context.Background(), key).Result()
	if err != nil {
		return 0, err
	}
	return res, nil
}

func (r redisKV) SRange(key string) ([]string, error) {
	res, err := r.cli.SMembers(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	return res, nil
}
