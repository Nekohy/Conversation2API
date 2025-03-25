package database

import (
	"github.com/redis/go-redis/v9"
	"go.etcd.io/bbolt"
	"time"
)

type RedisConfig struct {
	Client *redis.Client
	Expire time.Duration
}

type BoltConfig struct {
	DB              *bbolt.DB
	BucketName      []byte
	Expire          time.Duration // 0 表示不过期，负数立刻过期删除
	CleanupInterval time.Duration // 0 表示不清理
}

type Config struct {
	Redis *RedisConfig
	Bbolt *bbolt.DB
}
