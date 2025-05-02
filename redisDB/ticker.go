package redisdb

import (
	"context"
	"log"
	"time"
)

// Ticker to add tokens to clients buckets
func (c *Client) StartRefillTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.refillAllBuckets(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// Adding tokens to all the clients
func (c *Client) refillAllBuckets(ctx context.Context) {

	// Getting all the api-keys from DB
	apiKeys, err := c.rdb.Keys(ctx, "*").Result()
	if err != nil {
		log.Printf("Failed to get client keys: %v", err)
		return
	}

	// Adding tokens to every client
	for _, key := range apiKeys {
		script := `
			local key = KEYS[1]
			local data = redis.call("HMGET", key, "capacity", "rate", "tokens")
			local capacity = tonumber(data[1])
			local rate = tonumber(data[2])
			local current_tokens = tonumber(data[3])

			local new_tokens = math.min(capacity, current_tokens + rate)

			redis.call("HSET", key, "tokens", new_tokens)

			return new_tokens
		`
		if err := c.rdb.Eval(ctx, script, []string{key}).Err(); err != nil {
			log.Printf("Failed to refill bucket %s: %v", key, err)
		}
	}
}
