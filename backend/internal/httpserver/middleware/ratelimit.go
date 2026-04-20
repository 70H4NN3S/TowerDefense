package middleware

import (
	"net"
	"net/http"
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
}

// NewIPLimiter creates an IPLimiter allowing up to capacity requests per
// window, where window is the refill period to reach full capacity.
// Example: NewIPLimiter(10, time.Minute) → 10 req/min per IP.
func NewIPLimiter(capacity int, window time.Duration) *IPLimiter {
	return &IPLimiter{
		buckets:  make(map[string]*bucket),
		rate:     float64(capacity) / window.Seconds(),
		capacity: float64(capacity),
	}
}

// Allow reports whether the request from ip is within the rate limit.
// It updates the bucket state as a side effect.
func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{tokens: l.capacity, lastRefil: now}
		l.buckets[ip] = b
	}

	// Refill tokens proportional to elapsed time.
	elapsed := now.Sub(b.lastRefil).Seconds()
	b.tokens = min(l.capacity, b.tokens+elapsed*l.rate)
	b.lastRefil = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit returns a middleware that applies l to every request.
// Requests from IPs that exceed the limit receive 429 Too Many Requests.
func RateLimit(l *IPLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !l.Allow(ip) {
				w.Header().Set("Retry-After", "60")
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
