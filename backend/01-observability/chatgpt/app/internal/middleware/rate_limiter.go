package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rlRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rate_limiter_requests_total",
		Help: "Total requests seen by rate limiter",
	})
	rlBlocked = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rate_limiter_blocked_total",
		Help: "Total requests blocked by rate limiter",
	})
)

// RedisRateLimiter implements a simple fixed-window counter in Redis.
type RedisRateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewRedisRateLimiter(redisAddr string, limit int, window time.Duration) *RedisRateLimiter {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	return &RedisRateLimiter{
		client: rdb,
		limit:  limit,
		window: window,
	}
}

func (rl *RedisRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rlRequests.Inc()
		ctx := context.Background()
		// identify by IP (could use API key / user id in real apps)
		ip := realIP(r)
		key := fmt.Sprintf("rl:%s", ip)
		// increment
		cnt, err := rl.client.Incr(ctx, key).Result()
		if err != nil {
			log.Error().Err(err).Msg("redis incr failed")
			// fail open (allow) but log
			next.ServeHTTP(w, r)
			return
		}
		if cnt == 1 {
			// set TTL on first increment
			_ = rl.client.Expire(ctx, key, rl.window).Err()
		}
		if int(cnt) > rl.limit {
			rlBlocked.Inc()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func realIP(r *http.Request) string {
	// try X-Forwarded-For, else remote addr
	x := r.Header.Get("X-Forwarded-For")
	if x != "" {
		return x
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
