package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sushitana/pkg/config"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

var (
	Module      = fx.Provide(New)
	ErrNotFound = errors.New("not found")
)

type Client interface {
	Save(ctx context.Context, key string, value any, dur time.Duration) error
	SaveObj(ctx context.Context, key string, value any, dur time.Duration) (bool, error)
	Find(ctx context.Context, key string) (value string, err error)
	Delete(ctx context.Context, key string) (err error)
	FindObj(ctx context.Context, key string, value any) error
}

type client struct {
	redis  redis.UniversalClient
	prefix string
}

type Params struct {
	fx.In

	Config config.IConfig
}

func New(p Params) (Client, error) {

	var (
		prefix  = p.Config.GetString("redis.prefix")
		timeout = 5 * time.Second
	)

	connOpt := redis.UniversalOptions{
		ClientName:   p.Config.GetString("redis.clientName"),
		Addrs:        p.Config.GetStringSlice("redis.addrs"),
		Username:     p.Config.GetString("redis.username"),
		Password:     p.Config.GetString("redis.password"),
		DB:           p.Config.GetInt("redis.db"),
		PoolSize:     p.Config.GetInt("redis.poolSize"),
		MaxRedirects: p.Config.GetInt("redis.maxRedirects"),
		DialTimeout:  timeout,
	}

	conn := redis.NewUniversalClient(&connOpt)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := conn.Ping(ctx)
	if cmd.Err() != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", cmd.Err())
	}

	return &client{
		redis:  conn,
		prefix: prefix,
	}, nil
}

func (c client) getPrefixedKey(key string) string {
	return c.prefix + "." + key
}

func (c client) Save(ctx context.Context, key string, value interface{}, dur time.Duration) error {
	err := c.redis.Set(ctx, c.getPrefixedKey(key), value, dur).Err()
	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

func (c client) SaveObj(ctx context.Context, key string, value any, dur time.Duration) (bool, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("failed to marshal: %w", err)
	}
	ok, err := c.redis.SetNX(ctx, c.getPrefixedKey(key), b, dur).Result()
	if err != nil {
		return false, fmt.Errorf("failed to setnx: %w", err)
	}
	return ok, nil
}

func (c client) Find(ctx context.Context, key string) (string, error) {
	value, err := c.redis.Get(ctx, c.getPrefixedKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("failed to get key: %w", err)
	}
	return value, nil
}

func (c client) Delete(ctx context.Context, key string) error {
	err := c.redis.Del(ctx, c.getPrefixedKey(key)).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

func (c client) FindObj(ctx context.Context, key string, value interface{}) error {
	val, err := c.redis.Get(ctx, c.getPrefixedKey(key)).Result()
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}

	err = json.Unmarshal([]byte(val), value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal value: %w", err)
	}
	return nil
}
