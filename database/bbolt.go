//go:build bbolt

package database

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

type entry struct {
	CRC32     string    `json:"crc32"`
	Timestamp time.Time `json:"timestamp"`
}

type Write interface {
	write(conversationID string, crc32 string) error
	storeEntry(key, data []byte) error
}

type Read interface {
	initDB() error
	read(conversationID string) (string, error)
	deleteIfExpired(key []byte) error
}

// 初始化数据库，讲真的你应该先执行它
func (bc BoltConfig) initDB() (err error) {
	tx, err := bc.DB.Begin(true)
	if err != nil {
		return err
	}
	defer func(tx *bbolt.Tx) {
		_ = tx.Rollback()
	}(tx)
	_, err = tx.CreateBucketIfNotExists(bc.BucketName)
	if err != nil {
		return err
	}
	bc.StartCleanup()
	return tx.Commit()
}

// StartCleanup 定时清理协程
func (bc BoltConfig) StartCleanup() {
	if bc.Expire < 0 || bc.CleanupInterval < 0 {
		panic("BBolt Expire and CleanupInterval is not allowed to be negative")
	}
	if bc.Expire == 0 || bc.CleanupInterval == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(bc.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := bc.cleanupExpiredEntries(); err != nil {
				fmt.Printf("cleanup error: %v\n", err)
			}
		}
	}()
}

// 定时清理过期条目
func (bc BoltConfig) cleanupExpiredEntries() error {
	tx, err := bc.DB.Begin(true)
	if err != nil {
		return err
	}
	defer func(tx *bbolt.Tx) {
		_ = tx.Rollback()
	}(tx)

	bucket := tx.Bucket(bc.BucketName)
	if bucket == nil {
		return fmt.Errorf("bucket not found")
	}
	now := time.Now()
	cursor := bucket.Cursor()
	for k, v := cursor.First(); k != nil; {
		var e entry
		if err := json.Unmarshal(v, &e); err != nil {
			_ = cursor.Delete() // 删除无效条目
			continue
		}

		if now.Sub(e.Timestamp) > bc.Expire {
			_ = cursor.Delete()
			continue
		}
		k, v = cursor.Next()
	}
	return tx.Commit()
}

// 读取函数
func (bc BoltConfig) read(conversationID string) (string, error) {
	key := []byte(conversationID)

	// 开启事务
	tx, err := bc.DB.Begin(false)
	if err != nil {
		return "", err
	}
	defer func(tx *bbolt.Tx) {
		_ = tx.Rollback()
	}(tx)

	// 获取存储桶
	bucket := tx.Bucket(bc.BucketName)
	if bucket == nil {
		return "", fmt.Errorf("bucket not found")
	}

	// 读取数据
	data := bucket.Get(key)
	if data == nil {
		return "", nil
	}

	// 解析数据
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		return "", err
	}

	return e.CRC32, nil
}

// 写入函数
func (bc BoltConfig) write(conversationID string, crc32 string) error {
	// 准备键值数据
	key := []byte(conversationID)
	e := &entry{
		CRC32:     crc32,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}

	// 开启事务
	tx, err := bc.DB.Begin(true)
	if err != nil {
		return err
	}
	defer func(tx *bbolt.Tx) {
		_ = tx.Rollback() // 确保事务回滚（已提交的事务回滚无副作用）
	}(tx)

	// 获取存储桶
	bucket := tx.Bucket(bc.BucketName)
	if bucket == nil {
		return fmt.Errorf("bucket %q does not exist", bc.BucketName)
	}

	// 写入数据并提交
	if err := bucket.Put(key, data); err != nil {
		return err
	}
	return tx.Commit()
}

// 删除过期条目
func (bc BoltConfig) deleteIfExpired(key []byte) error {
	tx, err := bc.DB.Begin(true)
	if err != nil {
		return err
	}
	defer func(tx *bbolt.Tx) {
		_ = tx.Rollback()
	}(tx)

	bucket := tx.Bucket(bc.BucketName)
	if bucket == nil {
		return fmt.Errorf("bucket %q does not exist", bc.BucketName)
	}
	_ = bucket.Delete(key)
	return tx.Commit()
}
