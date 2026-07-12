package kaapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a simple per-IP token bucket for enroll/issue abuse resistance.
type RateLimiter struct {
	mu       sync.Mutex
	hits     map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter allows `limit` events per window per key.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{hits: make(map[string][]time.Time), limit: limit, window: window}
}

// Allow reports whether key may proceed.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cut := now.Add(-r.window)
	arr := r.hits[key]
	n := 0
	for _, t := range arr {
		if t.After(cut) {
			arr[n] = t
			n++
		}
	}
	arr = arr[:n]
	if len(arr) >= r.limit {
		r.hits[key] = arr
		return false
	}
	r.hits[key] = append(arr, now)
	return true
}

// ClientIP extracts a coarse client key.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware rate-limits by IP for selected paths.
func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/v1/enroll" || path == "/v1/key/issue" || path == "/v1/vouchers" {
			if !r.Allow(ClientIP(req) + "|" + path) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(429)
				_, _ = w.Write([]byte(`{"error":"rate_limited"}`))
				return
			}
		}
		next.ServeHTTP(w, req)
	})
}
