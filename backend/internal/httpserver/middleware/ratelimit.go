package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// bucket is a token-bucket state for one IP address.
type bucket struct {
	tokens    float64
	lastRefil time.Time
}

// IPLimiter is an in-memory per-IP token-bucket rate limiter.
// Zero value is not usable; construct with NewIPLimiter.
type IPLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens added per second
	capacity float64 // maximum tokens per bucket
	window   time.Duration
}

// NewIPLimiter creates an IPLimiter allowing up to capacity requests per
// window, where window is the refill period to reach full capacity.
// Example: NewIPLimiter(10, time.Minute) → 10 req/min per IP.
func NewIPLimiter(capacity int, window time.Duration) *IPLimiter {
	return &IPLimiter{
		buckets:  make(map[string]*bucket),
		rate:     float64(capacity) / window.Seconds(),
		capacity: float64(capacity),
		window:   window,
	}
}

// Allow reports whether the request from ip is within the rate limit.
// It updates the bucket state as a side effect and evicts entries that have
// been idle for longer than one window to bound memory use.
//
// The second return value is the number of seconds until a token is available;
// it is 0 when the request is allowed. Callers should use this value directly
// rather than calling RetryAfter separately to avoid a TOCTOU window.
func (l *IPLimiter) Allow(ip string) (allowed bool, retryAfterSecs int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{tokens: l.capacity, lastRefil: now}
		l.buckets[ip] = b
	}

	// Compute elapsed time BEFORE updating lastRefil so the eviction check
	// can use it as the "idle since last request" duration.
	elapsed := now.Sub(b.lastRefil).Seconds()

	// Lazy eviction: if the IP has not made a request in longer than one window
	// it would be refilled to full capacity anyway — evict and allow as fresh.
	if elapsed > l.window.Seconds() {
		delete(l.buckets, ip)
		return true, 0
	}

	b.tokens = min(l.capacity, b.tokens+elapsed*l.rate)
	b.lastRefil = now

	if b.tokens < 1 {
		need := 1 - b.tokens
		secs := max(1, int(math.Ceil(need/l.rate)))
		return false, secs
	}
	b.tokens--
	return true, 0
}

// RateLimit returns a middleware that applies l to every request.
// Requests from IPs that exceed the limit receive 429 Too Many Requests with
// a Retry-After header computed from the actual token-refill time.
func RateLimit(l *IPLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if allowed, retryAfter := l.Allow(ip); !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				http.Error(w, `{"error":{"code":"rate_limited","message":"Too many requests."}}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the client IP from the request, stripping the port.
func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
