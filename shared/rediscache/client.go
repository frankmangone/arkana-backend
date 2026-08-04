// Package rediscache constructs the shared Redis client used to store
// ephemeral, TTL-bound state (currently: quiz attempts). It holds no
// feature-specific logic - see features/quizzes/queries for that.
package rediscache

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// NewClient connects to addr and confirms it's reachable via PING before
// returning - mirrors db.go's initDB connect-then-Ping pattern, so a
// misconfigured REDIS_ADDR fails fast at startup instead of at the first
// quiz request.
func NewClient(addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Println("Redis connection established")
	return client, nil
}
