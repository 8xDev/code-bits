package main

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	rateLimitRequests = 10
	rateLimitWindow   = 10 * time.Second
)

// RateLimiterMiddleware is a chi middleware for IP-based rate limiting using Redis
func RateLimiterMiddleware(rdb *redis.Client, m *Metrics, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get client IP
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// Fallback or error
				ip = r.RemoteAddr
			}

			// Log context
			log := logger.With("client_ip", ip)

			// Increment total requests metric
			m.RateLimiterRequestCounter.WithLabelValues(ip).Inc()

			// Use Redis INCR to implement a fixed window counter
			key := "rate_limit:" + ip
			count, err := rdb.Incr(ctx, key).Result()

			if err != nil {
				log.ErrorContext(ctx, "Rate limiter Redis INCR failed", "error", err)
				// Fail open (allow request) if Redis fails
				next.ServeHTTP(w, r)
				return
			}

			// Set expiry only on the first request in the window
			if count == 1 {
				if err := rdb.Expire(ctx, key, rateLimitWindow).Err(); err != nil {
					log.ErrorContext(ctx, "Rate limiter Redis EXPIRE failed", "error", err)
					// Not fatal, but log it
				}
			}

			// Check if limit is exceeded
			if count > rateLimitRequests {
				log.WarnContext(ctx, "Rate limit exceeded")

				// Increment blocked requests metric
				m.RateLimiterBlockedCounter.WithLabelValues(ip).Inc()

				// Send 429 Too Many Requests
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			// Allow request
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}

}
