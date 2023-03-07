package cron

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type KVStore interface {
	Write(name string, content []byte) error
	Read(name string) ([]byte, error)
	Delete(name string) error
}

type redisKV struct {
	cli       *redis.Client
	keyPrefix string
}

func NewRedisKV(cli *redis.Client, prefix string) KVStore {
	return redisKV{
		cli:       cli,
		keyPrefix: prefix,
	}
}

func (r redisKV) Write(key string, value []byte) error {
	return r.cli.Set(context.Background(), r._key(key), value, 0).Err()
}

func (r redisKV) Read(key string) ([]byte, error) {
	res, err := r.cli.Get(context.Background(), r._key(key)).Result()
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func (r redisKV) Delete(key string) error {
	return r.cli.Del(context.Background(), r._key(key)).Err()
}

func (r redisKV) _key(name string) string {
	return r.keyPrefix + "_" + name
}
