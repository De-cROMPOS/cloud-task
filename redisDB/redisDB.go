package redisdb

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(addr string) *Client {
	return &Client{
		rdb: redis.NewClient(&redis.Options{
			Addr: addr, // "localhost:6379"
		}),
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Adding client to DB
func (c *Client) AddClient(ctx context.Context, apiKey string, capacity, rate int) error {
	_, err := c.rdb.HSet(ctx, apiKey,
		"capacity", capacity,
		"rate", rate,
		"tokens", capacity,
	).Result()
	return err
}

// Deleting client info
func (c *Client) DeleteClient(ctx context.Context, apiKey string) error {
	return c.rdb.Del(ctx, apiKey).Err()
}

// Returning true if api-key has tokens, else false
func (c *Client) TakeToken(ctx context.Context, apiKey string) (bool, error) {
	script := `
		local key = KEYS[1]
		local data = redis.call("HMGET", key, "tokens")
		local tokens = tonumber(data[1])

		if tokens and tokens >= 1 then
			redis.call("HINCRBY", key, "tokens", -1)
			return 1
		else
			return 0
		end
	`
	res, err := c.rdb.Eval(ctx, script, []string{apiKey}).Int64()
	return res == 1, err
}
