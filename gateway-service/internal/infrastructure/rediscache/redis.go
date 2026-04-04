package rediscache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr         string
	Password     string
	DB           int
	KeyPrefix    string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type Client struct {
	redisClient *redis.Client
	keyPrefix   string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		_ = redisClient.Close()
		return nil, fmt.Errorf("connect redis cache: %w", err)
	}

	return &Client{
		redisClient: redisClient,
		keyPrefix:   strings.TrimSpace(cfg.KeyPrefix),
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.redisClient == nil {
		return nil
	}
	return c.redisClient.Close()
}

func (c *Client) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	if c == nil || c.redisClient == nil {
		return false, nil
	}

	value, err := c.redisClient.Get(ctx, c.fullKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}

	if err := json.Unmarshal(value, dest); err != nil {
		return false, err
	}

	return true, nil
}

func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if c == nil || c.redisClient == nil {
		return nil
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.redisClient.Set(ctx, c.fullKey(key), payload, ttl).Err()
}

func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if c == nil || c.redisClient == nil || len(keys) == 0 {
		return nil
	}

	qualifiedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		qualifiedKeys = append(qualifiedKeys, c.fullKey(key))
	}
	if len(qualifiedKeys) == 0 {
		return nil
	}

	return c.redisClient.Del(ctx, qualifiedKeys...).Err()
}

func (c *Client) fullKey(key string) string {
	if c.keyPrefix == "" {
		return key
	}
	return c.keyPrefix + ":" + key
}
