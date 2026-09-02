package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultMaximumClientBuckets = 10000

type RateLimit struct {
	Rate  float64
	Burst float64
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type Limiter struct {
	mu             sync.Mutex
	buckets        map[string]*bucket
	overflow       *bucket
	limit          RateLimit
	now            func() time.Time
	idleEviction   time.Duration
	maximumBuckets int
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
		now: time.Now, idleEviction: 10 * time.Minute, maximumBuckets: defaultMaximumClientBuckets,
	}
}

func (limiter *Limiter) Allow(client string) bool {
	now := limiter.now()
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	existing, found := limiter.buckets[client]
	if found {
		return limiter.allowBucket(existing, now)
	}

	limiter.evictIdle(now)
	if len(limiter.buckets) >= limiter.maximumBuckets {
		if limiter.overflow == nil {
			limiter.overflow = &bucket{tokens: limiter.limit.Burst, lastSeen: now}
		}
		return limiter.allowBucket(limiter.overflow, now)
	}

	limiter.buckets[client] = &bucket{tokens: limiter.limit.Burst - 1, lastSeen: now}
	return true
}

func (limiter *Limiter) allowBucket(entry *bucket, now time.Time) bool {
	elapsed := now.Sub(entry.lastSeen).Seconds()
	if elapsed > 0 {
		entry.tokens = min(limiter.limit.Burst, entry.tokens+elapsed*limiter.limit.Rate)
	}
	entry.lastSeen = now
	if entry.tokens < 1 {
		return false
	}
	entry.tokens--
	return true
}

func (limiter *Limiter) evictIdle(now time.Time) {
	for client, entry := range limiter.buckets {
		if now.Sub(entry.lastSeen) > limiter.idleEviction {
			delete(limiter.buckets, client)
		}
	}
	if limiter.overflow != nil && now.Sub(limiter.overflow.lastSeen) > limiter.idleEviction {
		limiter.overflow = nil
	}
}

func (limiter *Limiter) RetryAfter() time.Duration {
	if limiter.limit.Rate <= 0 {
		return time.Second
	}
	return time.Duration(float64(time.Second) / limiter.limit.Rate)
}

func parseTrustedProxyCIDRs(values []string) []*net.IPNet {
	proxies := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err == nil {
			proxies = append(proxies, network)
		}
	}
	return proxies
}

func clientKey(request *http.Request) string {
	return clientKeyWithProxies(request, nil)
}

func trustedIP(ip net.IP, trusted []*net.IPNet) bool {
	for _, network := range trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// clientKeyWithProxies trusts forwarding data only when the direct TCP peer is
// explicitly trusted. It then walks X-Forwarded-For from right to left, skipping
// trusted proxy hops until it finds the client-facing untrusted address. This
// prevents a client-supplied leftmost XFF value from creating fresh rate-limit
// buckets when a reverse proxy appends instead of replacing the header.
func clientKeyWithProxies(request *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return host
	}
	if !trustedIP(peer, trusted) {
		return peer.String()
	}

	chain := make([]net.IP, 0, 4)
	for _, candidate := range strings.Split(request.Header.Get("X-Forwarded-For"), ",") {
		if ip := net.ParseIP(strings.TrimSpace(candidate)); ip != nil {
			chain = append(chain, ip)
		}
	}
	if len(chain) == 0 {
		return peer.String()
	}

	for index := len(chain) - 1; index >= 0; index-- {
		if !trustedIP(chain[index], trusted) {
			return chain[index].String()
		}
	}
	return chain[0].String()
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}
