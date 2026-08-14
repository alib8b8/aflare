// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package api

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// DefaultRateLimitRPS is the default rate limit: 10 requests per minute.
	DefaultRateLimitRPS = 10.0 / 60.0 // 0.1667 req/s

	// DefaultRateLimitBurst allows a small burst of concurrent requests.
	DefaultRateLimitBurst = 3

	// rateLimiterCleanupInterval is how often stale entries are purged.
	rateLimiterCleanupInterval = 5 * time.Minute

	// rateLimiterMaxAge is the maximum age of an idle entry before cleanup.
	rateLimiterMaxAge = 10 * time.Minute
)

// ipRateLimiter tracks per-IP rate limiters.
type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rate     rate.Limit
	burst    int
}

// rateLimiterEntry holds a rate limiter and its last access time.
type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newIPRateLimiter creates a per-IP rate limiter with the given rate and burst.
func newIPRateLimiter(rps rate.Limit, burst int) *ipRateLimiter {
	rl := &ipRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rate:     rps,
		burst:    burst,
	}
	go rl.cleanupLoop()
	return rl
}

// allow checks whether the given IP is allowed to make a request.
// Returns true if allowed, false if rate limited.
func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	entry, exists := rl.limiters[ip]
	if !exists {
		entry = &rateLimiterEntry{
			limiter:  rate.NewLimiter(rl.rate, rl.burst),
			lastSeen: time.Now(),
		}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()

	return entry.limiter.Allow()
}

// cleanupLoop periodically removes stale entries.
func (rl *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rateLimiterCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		rl.cleanup()
	}
}

// cleanup removes entries that haven't been seen recently.
func (rl *ipRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rateLimiterMaxAge)
	for ip, entry := range rl.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.limiters, ip)
		}
	}
}

// extractClientIP extracts the client IP from the request.
//
// By default ONLY RemoteAddr is trusted — X-Forwarded-For / X-Real-IP are
// client-controlled headers and trusting them unconditionally allows an
// attacker to spoof a different IP per request and bypass per-IP rate
// limiting. Set AFLARE_TRUST_PROXY_HEADERS=1 when aflare runs behind a
// trusted reverse proxy that overwrites these headers.
func extractClientIP(r *http.Request) string {
	if os.Getenv("AFLARE_TRUST_PROXY_HEADERS") == "1" {
		// X-Forwarded-For: client, proxy1, proxy2
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first (original client) IP, trim whitespace
			xff = strings.TrimSpace(xff)
			for i := 0; i < len(xff); i++ {
				if xff[i] == ',' {
					return strings.TrimSpace(xff[:i])
				}
			}
			return xff
		}

		// X-Real-IP is set by some proxies
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitMiddleware returns an HTTP middleware that limits requests per IP.
// When the limit is exceeded, it returns 429 Too Many Requests with a
// Retry-After header.
func rateLimitMiddleware(rl *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)

		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded, 10 req/min per IP","retry_after":60}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
