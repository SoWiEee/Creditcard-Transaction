package initialize

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedis(ctx context.Context, addr, password string, db int) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	ctxPing, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctxPing).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return rdb, nil
}
