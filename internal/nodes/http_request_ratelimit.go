// Copyright (c) 2026 llm-box Contributors
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

package nodes

import (
	"context"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"

	"golang.org/x/time/rate"
)

// httpRateLimiters is a process-wide pool of token-bucket limiters keyed by
// URL host (the net/url Host field, which is "host:port"). Requests that
// target the same host share one bucket so that, e.g., all calls to a given
// exchange market-data endpoint are throttled together regardless of which
// workflow step issues them. Different hosts get independent buckets and do
// not starve each other.
//
// Limiters are created lazily on first use with the rate/burst supplied by
// that first request; subsequent requests to the same host reuse the existing
// limiter (the per-host contract is "first config wins"). This is intentional:
// rate limiting is a property of the *destination*, not the caller, and a
// shared bucket is the only way to actually cap aggregate RPS to an external
// API from a fan-out workflow.
var httpRateLimiters sync.Map // map[string]*rate.Limiter

// httpRateLimitConfig captures the per-step rate-limit and retry policy
// parsed from http_request node params. The zero value means "no rate
// limit, no retry", preserving the original single-shot behavior.
type httpRateLimitConfig struct {
	rps         float64       // max requests per second; 0 = unlimited
	burst       int           // token-bucket burst size (>=1 when rps>0)
	maxRetries  int           // max retry attempts after the initial try
	backoff     time.Duration // initial backoff for the first retry
	maxBackoff  time.Duration // backoff cap
	retryStatus map[int]bool  // status codes that trigger a retry
}

// defaultRetryStatuses is the set of HTTP status codes that are retried by
// default when retry_on_status is not specified. These are the transient
// failures typical of external APIs under load (rate limited, gateway
// errors, service unavailable). 4xx other than 429 are not retried because
// they represent client errors that will not succeed on repeat.
var defaultRetryStatuses = []int{429, 500, 502, 503, 504}

// parseRateLimitConfig reads the optional rate-limit/retry params from the
// http_request node params. All fields are optional; missing fields yield
// the documented defaults. A zero rps disables rate limiting entirely
// (backward compatible), and a zero maxRetries disables retries.
func parseRateLimitConfig(params map[string]string) httpRateLimitConfig {
	cfg := httpRateLimitConfig{
		backoff:    100 * time.Millisecond,
		maxBackoff: 5 * time.Second,
	}

	// rate_limit_rps: float64, 0 = unlimited.
	cfg.rps = paramFloat(params, "rate_limit_rps", 0, 0, 100000)

	// rate_limit_burst: defaults to ceil(rps) (at least 1 when limiting).
	cfg.burst = paramInt(params, "rate_limit_burst", 0, 0, 1<<30)
	if cfg.rps > 0 {
		if cfg.burst <= 0 {
			cfg.burst = int(math.Ceil(cfg.rps))
			if cfg.burst < 1 {
				cfg.burst = 1
			}
		}
	}

	cfg.maxRetries = paramInt(params, "max_retries", 0, 0, 100)

	if v := paramInt(params, "retry_backoff_ms", 0, 0, 60000); v > 0 {
		cfg.backoff = time.Duration(v) * time.Millisecond
	}
	if v := paramInt(params, "retry_max_backoff_ms", 0, 0, 60000); v > 0 {
		cfg.maxBackoff = time.Duration(v) * time.Millisecond
	}
	if cfg.backoff > cfg.maxBackoff {
		cfg.backoff = cfg.maxBackoff
	}

	// retry_on_status: comma-separated status codes. Defaults to the
	// standard transient set when max_retries > 0.
	cfg.retryStatus = parseRetryStatuses(params["retry_on_status"])

	return cfg
}

// parseRetryStatuses parses a comma-separated list of HTTP status codes
// into a set. An empty input yields the default transient set so that
// retries are meaningful out of the box.
func parseRetryStatuses(raw string) map[int]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		out := make(map[int]bool, len(defaultRetryStatuses))
		for _, c := range defaultRetryStatuses {
			out[c] = true
		}
		return out
	}
	out := make(map[int]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		code, err := strconv.Atoi(part)
		if err != nil || code < 100 || code > 599 {
			continue
		}
		out[code] = true
	}
	return out
}

// getHTTPRateLimiter returns the shared *rate.Limiter for the given URL host
// (host:port). On first access for a host a new limiter is created with the
// supplied rps/burst and stored; later callers reuse it. A zero rps returns
// nil, signalling "no rate limiting" to the caller.
//
// The loadOrStore pattern means the *first* request to a host establishes
// the bucket's rate; if two steps target the same host with different rps,
// the second config is ignored for that host. This is the intended
// per-host sharing semantics (see httpRateLimiters docs).
func getHTTPRateLimiter(reqURL *url.URL, rps float64, burst int) *rate.Limiter {
	if rps <= 0 {
		return nil
	}
	host := reqURL.Host
	if host == "" {
		host = reqURL.String()
	}
	if actual, ok := httpRateLimiters.Load(host); ok {
		return actual.(*rate.Limiter)
	}
	lim := rate.NewLimiter(rate.Limit(rps), burst)
	actual, _ := httpRateLimiters.LoadOrStore(host, lim)
	return actual.(*rate.Limiter)
}

// backoffForAttempt returns the exponential backoff delay for the given
// attempt (1-indexed: 1 = first retry). The delay is base * 2^(attempt-1),
// capped at max. Overflow beyond max collapses to max.
func (c httpRateLimitConfig) backoffForAttempt(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if c.backoff <= 0 {
		return 0
	}
	delay := c.backoff
	for i := 1; i < attempt; i++ {
		if delay > c.maxBackoff/2 {
			return c.maxBackoff
		}
		delay *= 2
		if delay >= c.maxBackoff {
			return c.maxBackoff
		}
	}
	return delay
}

// shouldRetryStatus reports whether the given HTTP status code is in the
// retry set.
func (c httpRateLimitConfig) shouldRetryStatus(code int) bool {
	return c.retryStatus[code]
}

// resetHTTPRateLimitersForTest clears the limiter pool. It is intended only
// for use by tests that need a clean per-host bucket state; production code
// must not call it.
func resetHTTPRateLimitersForTest() {
	httpRateLimiters.Range(func(k, _ any) bool {
		httpRateLimiters.Delete(k)
		return true
	})
}

// rateLimitedWait blocks until the limiter allows one token, honoring ctx
// cancellation (so a cancelled workflow returns immediately rather than
// waiting for a token). A nil limiter is a no-op. The wait is logged at
// debug level so rate-limited steps are traceable without being noisy.
func rateLimitedWait(ctx context.Context, lim *rate.Limiter, host string) error {
	if lim == nil {
		return nil
	}
	logger.Debug("HTTP rate limited", "host", host, "limit", lim.Limit(), "burst", lim.Burst())
	return lim.Wait(ctx)
}
