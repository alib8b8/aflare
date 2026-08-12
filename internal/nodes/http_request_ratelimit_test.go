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

package nodes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// allowLoopback sets AFLARE_ALLOW_LOOPBACK so the http_request node's SSRF
// guard permits httptest servers on 127.0.0.1. Every test in this file hits
// a local mock server, so every test needs it.
func allowLoopback(t *testing.T) {
	t.Helper()
	t.Setenv("AFLARE_ALLOW_LOOPBACK", "1")
}

// TestHTTPRequest_RateLimit verifies that a token-bucket limiter throttles
// bursts beyond `burst`: with rate_limit_rps=2 (burst defaults to 2), the
// first 2 requests return immediately and the remaining 3 of 5 must wait
// for token refill (~0.5s each, ~1.5s total).
func TestHTTPRequest_RateLimit(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	node := &HTTPRequestNode{}
	params := map[string]string{
		"url":            srv.URL,
		"rate_limit_rps": "2",
		// rate_limit_burst omitted -> defaults to ceil(rps)=2
	}

	start := time.Now()
	for i := 0; i < 5; i++ {
		out, err := node.Execute(context.Background(), "", params)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		if !strings.Contains(out, "HTTP 200") {
			t.Fatalf("request %d unexpected output: %q", i+1, out)
		}
	}
	elapsed := time.Since(start)

	if got := atomic.LoadInt32(&hits); got != 5 {
		t.Fatalf("expected 5 server hits, got %d", got)
	}

	// 5 requests @ rps=2 burst=2: first 2 immediate, next 3 wait ~0.5s each
	// => lower bound ~1.4s. Upper bound is generous to tolerate scheduler
	// jitter on loaded CI.
	if elapsed < 1300*time.Millisecond {
		t.Fatalf("rate limit did not throttle: elapsed=%v (expected >=1.3s)", elapsed)
	}
	if elapsed > 6*time.Second {
		t.Fatalf("rate limit over-throttled: elapsed=%v", elapsed)
	}
}

// TestHTTPRequest_RetryOn5xx verifies that a transient 503 is retried and
// eventually succeeds when the mock server recovers on the 3rd call.
func TestHTTPRequest_RetryOn5xx(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, "unavailable")
			return
		}
		_, _ = fmt.Fprint(w, "recovered")
	}))
	defer srv.Close()

	node := &HTTPRequestNode{}
	params := map[string]string{
		"url":              srv.URL,
		"max_retries":      "3",
		"retry_backoff_ms": "5",
	}

	out, err := node.Execute(context.Background(), "", params)
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if !strings.Contains(out, "HTTP 200") {
		t.Fatalf("unexpected output: %q", out)
	}
	if !strings.Contains(out, "recovered") {
		t.Fatalf("expected recovered body, got: %q", out)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 server calls (2x503 + 1x200), got %d", got)
	}
}

// TestHTTPRequest_RetryExhausted verifies that after max_retries+1 attempts
// all returning 503, the node gives up with an error mentioning the status
// and the retry count.
func TestHTTPRequest_RetryExhausted(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, "unavailable")
	}))
	defer srv.Close()

	node := &HTTPRequestNode{}
	params := map[string]string{
		"url":              srv.URL,
		"max_retries":      "2",
		"retry_backoff_ms": "5",
	}

	out, err := node.Execute(context.Background(), "", params)
	if err == nil {
		t.Fatalf("expected error after retries exhausted, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("error should mention status 503, got: %v", err)
	}
	if !strings.Contains(err.Error(), "after 2 retries") {
		t.Fatalf("error should mention retry count, got: %v", err)
	}
	// max_retries=2 => initial attempt + 2 retries = 3 total calls.
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 server calls, got %d", got)
	}
}

// TestHTTPRequest_RetrySkips4xx verifies that a 400 (not in the default
// retry_on_status set) is not retried: the node returns immediately after a
// single attempt.
func TestHTTPRequest_RetrySkips4xx(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, "bad request")
	}))
	defer srv.Close()

	node := &HTTPRequestNode{}
	params := map[string]string{
		"url":              srv.URL,
		"max_retries":      "3",
		"retry_backoff_ms": "5",
	}

	out, err := node.Execute(context.Background(), "", params)
	if err == nil {
		t.Fatalf("expected error for 400, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("error should mention status 400, got: %v", err)
	}
	// 4xx is non-retryable: exactly one attempt, no retries.
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 server call (no retry on 4xx), got %d", got)
	}
}

// TestHTTPRequest_NoConfigBackwardCompat verifies that when no rate-limit or
// retry params are supplied, the node behaves exactly as before: a 200 is
// returned as "HTTP 200\n<body>", and a 500 surfaces the original
// "HTTP request failed with status 500" error with no retry suffix.
func TestHTTPRequest_NoConfigBackwardCompat(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "hello")
		}))
		defer srv.Close()

		node := &HTTPRequestNode{}
		out, err := node.Execute(context.Background(), "", map[string]string{"url": srv.URL})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "HTTP 200\nhello"
		if out != want {
			t.Fatalf("output mismatch:\n got: %q\nwant: %q", out, want)
		}
	})

	t.Run("error_500_no_retry", func(t *testing.T) {
		var calls int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		node := &HTTPRequestNode{}
		_, err := node.Execute(context.Background(), "", map[string]string{"url": srv.URL})
		if err == nil {
			t.Fatal("expected error for 500")
		}
		// Original message format, no "after N retries" suffix.
		want := "HTTP request failed with status 500"
		if err.Error() != want {
			t.Fatalf("error mismatch:\n got: %q\nwant: %q", err.Error(), want)
		}
		if got := atomic.LoadInt32(&calls); got != 1 {
			t.Fatalf("expected exactly 1 call (no retries by default), got %d", got)
		}
	})
}

// TestHTTPRequest_PerHostLimiter verifies that rate limiters are isolated per
// host:port. Two different mock servers (different ports => different URL
// hosts) each get their own bucket, so 2 requests to A and 2 to B all return
// immediately instead of sharing a single rps=2/burst=2 bucket (which would
// throttle the 3rd/4th).
func TestHTTPRequest_PerHostLimiter(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "A")
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "B")
	}))
	defer srvB.Close()

	// Sanity: the two servers must be on different host:port so they map to
	// different limiter keys. (httptest always picks a free port, so this
	// holds unless something is very wrong.)
	if srvA.URL == srvB.URL {
		t.Fatalf("test setup error: both mock servers share URL %q", srvA.URL)
	}

	node := &HTTPRequestNode{}
	paramsA := map[string]string{"url": srvA.URL, "rate_limit_rps": "2"}
	paramsB := map[string]string{"url": srvB.URL, "rate_limit_rps": "2"}

	start := time.Now()
	// 2 requests to A (burst=2) + 2 to B (burst=2): all immediate if buckets
	// are per-host. If they wrongly shared one bucket, 4 requests at rps=2
	// burst=2 would block ~1s on the 3rd/4th.
	for i := 0; i < 2; i++ {
		if out, err := node.Execute(context.Background(), "", paramsA); err != nil || !strings.Contains(out, "HTTP 200") {
			t.Fatalf("srvA request %d failed: out=%q err=%v", i+1, out, err)
		}
		if out, err := node.Execute(context.Background(), "", paramsB); err != nil || !strings.Contains(out, "HTTP 200") {
			t.Fatalf("srvB request %d failed: out=%q err=%v", i+1, out, err)
		}
	}
	elapsed := time.Since(start)

	// All 4 served from burst capacity => should be well under the ~1s a
	// shared bucket would impose.
	if elapsed > 600*time.Millisecond {
		t.Fatalf("per-host isolation failed: 4 burst requests took %v (expected <600ms)", elapsed)
	}
}

// TestHTTPRequest_RateLimitKeyMergesHosts (M-9) verifies that an explicit
// rate_limit_key OVERRIDES the default URL.Host bucketing: two DIFFERENT
// hosts (different mock servers) that share the same rate_limit_key MUST
// share one token bucket, so 4 requests at rps=2/burst=2 throttle the 3rd
// and 4th (taking ~1s). Without rate_limit_key the per-host test above
// proves they would NOT throttle — so this test directly exercises the M-9
// override.
//
// This is the M-9 use case: api.example.com and api2.example.com both
// resolve to the same backend IP; without an explicit key they would each
// get their own bucket and double the effective RPS against the backend.
// rate_limit_key lets the operator merge them into one bucket.
func TestHTTPRequest_RateLimitKeyMergesHosts(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "A")
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "B")
	}))
	defer srvB.Close()
	if srvA.URL == srvB.URL {
		t.Fatalf("test setup error: both mock servers share URL %q", srvA.URL)
	}

	node := &HTTPRequestNode{}
	// Both targets share rate_limit_key="shared-backend" so they collapse
	// into one bucket regardless of their distinct URL.Host.
	paramsA := map[string]string{
		"url":            srvA.URL,
		"rate_limit_rps": "2",
		"rate_limit_key": "shared-backend",
	}
	paramsB := map[string]string{
		"url":            srvB.URL,
		"rate_limit_rps": "2",
		"rate_limit_key": "shared-backend",
	}

	start := time.Now()
	// 4 sequential requests against a SHARED bucket with rps=2/burst=2: the
	// first 2 consume the burst immediately; the 3rd waits ~0.5s for one
	// token and the 4th waits ~0.5s more, so the limiter GUARANTEES a minimum
	// of ~1.0s (2 * 0.5s of mandatory Wait() blocking, sequential because each
	// Execute calls limiter.Wait before its HTTP call). This is the opposite of
	// TestHTTPRequest_PerHostLimiter, which asserts <600ms with per-host
	// buckets (all 4 served from burst capacity).
	var hits int32
	for i := 0; i < 2; i++ {
		if out, err := node.Execute(context.Background(), "", paramsA); err != nil || !strings.Contains(out, "HTTP 200") {
			t.Fatalf("srvA request %d failed: out=%q err=%v", i+1, out, err)
		}
		atomic.AddInt32(&hits, 1)
		if out, err := node.Execute(context.Background(), "", paramsB); err != nil || !strings.Contains(out, "HTTP 200") {
			t.Fatalf("srvB request %d failed: out=%q err=%v", i+1, out, err)
		}
		atomic.AddInt32(&hits, 1)
	}
	elapsed := time.Since(start)

	if got := atomic.LoadInt32(&hits); got != 4 {
		t.Fatalf("expected 4 successful requests, got %d", got)
	}
	// The limiter guarantees ~1.0s of blocking; assert >=0.9s to tolerate
	// clock/scheduler jitter while staying clearly above the per-host test's
	// <600ms bound (so a regression that fails to merge buckets would fall
	// under 0.6s and trip this assertion).
	if elapsed < 900*time.Millisecond {
		t.Fatalf("rate_limit_key did not merge hosts into one bucket: elapsed=%v (expected >=0.9s for shared rps=2/burst=2 over 4 requests)", elapsed)
	}
	if elapsed > 6*time.Second {
		t.Fatalf("rate_limit_key over-throttled: elapsed=%v", elapsed)
	}
}

// TestHTTPRequest_RateLimitKeyFallsBackToHost (M-9) verifies that when
// rate_limit_key is NOT set, the legacy URL.Host bucketing is preserved
// (per-host isolation). This is a regression guard for the M-9 change: the
// new keyOverride parameter must not change behaviour when empty.
func TestHTTPRequest_RateLimitKeyFallsBackToHost(t *testing.T) {
	allowLoopback(t)
	resetHTTPRateLimitersForTest()

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "A")
	}))
	defer srvA.Close()
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "B")
	}))
	defer srvB.Close()
	if srvA.URL == srvB.URL {
		t.Fatalf("test setup error: both mock servers share URL %q", srvA.URL)
	}

	node := &HTTPRequestNode{}
	// rate_limit_key intentionally NOT set: behaviour must match the legacy
	// per-host bucketing (each host gets its own bucket).
	paramsA := map[string]string{"url": srvA.URL, "rate_limit_rps": "2"}
	paramsB := map[string]string{"url": srvB.URL, "rate_limit_rps": "2"}

	start := time.Now()
	for i := 0; i < 2; i++ {
		if out, err := node.Execute(context.Background(), "", paramsA); err != nil || !strings.Contains(out, "HTTP 200") {
			t.Fatalf("srvA request %d failed: out=%q err=%v", i+1, out, err)
		}
		if out, err := node.Execute(context.Background(), "", paramsB); err != nil || !strings.Contains(out, "HTTP 200") {
			t.Fatalf("srvB request %d failed: out=%q err=%v", i+1, out, err)
		}
	}
	elapsed := time.Since(start)
	// Per-host buckets => all 4 served from burst, well under 1s.
	if elapsed > 600*time.Millisecond {
		t.Fatalf("empty rate_limit_key should fall back to per-host bucketing: 4 burst requests took %v (expected <600ms)", elapsed)
	}
}
