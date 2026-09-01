package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimit describes a token-bucket allowance.
type RateLimit struct {
	// Rate is the sustained requests per second permitted per client.
	Rate float64
	// Burst is how many requests may arrive at once before throttling starts.
	Burst float64
}

// bucket is one client's allowance. Tokens are computed lazily from elapsed
// time, so no background goroutine is needed to refill them.
type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// Limiter throttles per client. Writes and commands are limited separately from
// reads, because the damage a flood of actuator commands can do is not the same
// as the cost of a flood of reads.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   RateLimit
	now     func() time.Time
	// idleEviction bounds memory: a client that stops calling is forgotten.
	idleEviction time.Duration
}

func NewLimiter(limit RateLimit) *Limiter {
	if limit.Rate <= 0 {
		limit.Rate = 5
	}
	if limit.Burst < 1 {
		limit.Burst = limit.Rate
	}
	return &Limiter{
		buckets: map[string]*bucket{}, limit: limit,
		now: time.Now, idleEviction: 10 * time.Minute,
	}
}

// Allow reports whether a client may proceed, consuming one token if so.
func (limiter *Limiter) Allow(client string) bool {
	now := limiter.now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	existing, found := limiter.buckets[client]
	if !found {
		limiter.evictIdle(now)
		limiter.buckets[client] = &bucket{tokens: limiter.limit.Burst - 1, lastSeen: now}
		return true
	}
	elapsed := now.Sub(existing.lastSeen).Seconds()
	existing.tokens = min(limiter.limit.Burst, existing.tokens+elapsed*limiter.limit.Rate)
	existing.lastSeen = now
	if existing.tokens < 1 {
		return false
	}
	existing.tokens--
	return true
}

func (limiter *Limiter) evictIdle(now time.Time) {
	for client, entry := range limiter.buckets {
		if now.Sub(entry.lastSeen) > limiter.idleEviction {
			delete(limiter.buckets, client)
		}
	}
}

// RetryAfter reports how long a throttled client should wait, so the response
// tells the caller what to do rather than only that it failed.
func (limiter *Limiter) RetryAfter() time.Duration {
	if limiter.limit.Rate <= 0 {
		return time.Second
	}
	return time.Duration(float64(time.Second) / limiter.limit.Rate)
}

// clientKey identifies a caller for rate limiting. The remote address is used
// directly: forwarded headers are attacker-controlled unless a trusted proxy
// sets them, and trusting them here would let one client masquerade as many.
func clientKey(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return request.RemoteAddr
	}
	return host
}

// isMutating reports whether a request changes state, and therefore whether it
// should be counted against the stricter allowance.
func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}
