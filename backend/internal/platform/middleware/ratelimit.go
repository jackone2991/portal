package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter is a token-bucket limiter keyed by client IP. Use it on the
// outer perimeter of auth endpoints (login, refresh, oidc callback) to slow
// brute-force / token-spray attacks.
//
// Buckets that have not been used recently are GC'd to bound memory.
//
// For a multi-instance deployment behind Traefik, use the equivalent
// Traefik middleware OR replace this with a Redis-backed token bucket.
type IPRateLimiter struct {
	rate   rate.Limit
	burst  int
	mu     sync.Mutex
	buckets map[string]*ipBucket
	idleTTL time.Duration
}

type ipBucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter — `r` is the steady-state rate (requests per second);
// `burst` is the bucket capacity. Common preset: 5/s burst 10 for /auth/*.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	l := &IPRateLimiter{
		rate:    r,
		burst:   burst,
		buckets: make(map[string]*ipBucket),
		idleTTL: 10 * time.Minute,
	}
	go l.gcLoop()
	return l
}

// Middleware returns a chi-compatible middleware applying the limiter.
// On limit exceeded: 429 with Retry-After header.
func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.allow(ip) {
			w.Header().Set("Retry-After", "1")
			writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *IPRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	b, ok := l.buckets[ip]
	if !ok {
		b = &ipBucket{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.buckets[ip] = b
	}
	b.lastSeen = time.Now()
	l.mu.Unlock()
	return b.limiter.Allow()
}

func (l *IPRateLimiter) gcLoop() {
	t := time.NewTicker(l.idleTTL / 2)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-l.idleTTL)
		l.mu.Lock()
		for k, b := range l.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}

// clientIP extracts the best-effort client address. Trusts X-Forwarded-For
// only when set by Traefik (configure trusted proxies in production!).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first hop (client). Traefik appends; the leftmost is the source.
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return trimSpace(xff[:i])
			}
		}
		return trimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
