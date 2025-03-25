//go:build redis

package database

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
)

// 获取对话ID的MD5值，不存在则会直接返回空字符串
func (rc RedisConfig) read(conversationID string) (crc32 string, err error) {
	val, err := rc.Client.Get(context.Background(), conversationID).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return val, nil
}

// 写入对话ID和CRC32组合值
func (rc RedisConfig) write(conversationID string, crc32 string) (err error) {
	err = rc.Client.Set(context.Background(), conversationID, crc32, rc.Expire).Err()
	return err
}
