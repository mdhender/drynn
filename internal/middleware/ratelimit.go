package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	"golang.org/x/time/rate"
)

// Defaults for auth-endpoint rate limiting: one token every 6 seconds
// (10 requests/minute) with a burst of 5. Tuned for human typing while still
// frustrating credential-stuffing and reset-email abuse.
const (
	DefaultAuthRate  = rate.Limit(1.0 / 6.0)
	DefaultAuthBurst = 5
)

// RateLimiter is an in-process token-bucket rate limiter keyed by a string
// identifier (typically a client IP). Stale buckets are garbage collected on
// a lazy schedule so memory does not grow unbounded under IP churn.
type RateLimiter struct {
	rate  rate.Limit
	burst int

	mu         sync.Mutex
	visitors   map[string]*rateLimiterVisitor
	cleanupGap time.Duration
	lastClean  time.Time
}

type rateLimiterVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter returns a RateLimiter that refills tokens at r per second and
// permits short bursts up to burst tokens.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       r,
		burst:      burst,
		visitors:   make(map[string]*rateLimiterVisitor),
		cleanupGap: 10 * time.Minute,
		lastClean:  time.Now(),
	}
}

// Middleware returns an Echo middleware that rejects requests with HTTP 429
// once the caller's IP has exhausted its token bucket. The same RateLimiter
// can be mounted on several routes; budgets are shared across mount points so
// an attacker cannot spread a burst across endpoints on the same IP.
func (rl *RateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !rl.allow(c.RealIP()) {
				return c.String(http.StatusTooManyRequests, "too many requests; please slow down and try again")
			}
			return next(c)
		}
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, ok := rl.visitors[key]
	if !ok {
		v = &rateLimiterVisitor{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.visitors[key] = v
	}
	v.lastSeen = now

	if now.Sub(rl.lastClean) > rl.cleanupGap {
		for k, vv := range rl.visitors {
			if now.Sub(vv.lastSeen) > rl.cleanupGap {
				delete(rl.visitors, k)
			}
		}
		rl.lastClean = now
	}

	return v.limiter.AllowN(now, 1)
}
