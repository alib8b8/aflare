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

package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alib8b8/aflare/internal/cache"
)

// newSlowCountingMockLLM is newCountingMockLLM plus a per-request delay so
// that concurrent callers reliably overlap inside the delay window — the
// precondition for observing singleflight dedup.
func newSlowCountingMockLLM(t *testing.T, response string, delay time.Duration) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.ReadAll(r.Body)
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + response + `"}}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// TestLLMSingleflight_ConcurrentIdenticalRequestsShareOneUpstreamCall is
// the core P0-3 scenario: with the response cache active, N concurrent
// identical non-streaming requests must collapse into exactly one upstream
// call, and every caller must receive the leader's response.
func TestLLMSingleflight_ConcurrentIdenticalRequestsShareOneUpstreamCall(t *testing.T) {
	srv, calls := newSlowCountingMockLLM(t, "shared-answer", 150*time.Millisecond)
	n := newCacheTestNode(t, srv.URL, nil)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	const callers = 5
	start := make(chan struct{})
	outs := make([]string, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release all callers at once so they overlap
			outs[i], errs[i] = n.Execute(context.Background(), "hello", params)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := 0; i < callers; i++ {
		if errs[i] != nil {
			t.Fatalf("caller %d failed: %v", i, errs[i])
		}
		if outs[i] != "shared-answer" {
			t.Errorf("caller %d output = %q, want shared-answer", i, outs[i])
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1 (identical requests must be deduplicated)", got)
	}
}

// TestLLMSingleflight_DifferentRequestsNotDeduped verifies the flip side:
// concurrent requests with different prompts are different flights and
// each reaches upstream.
func TestLLMSingleflight_DifferentRequestsNotDeduped(t *testing.T) {
	srv, calls := newSlowCountingMockLLM(t, "r", 80*time.Millisecond)
	n := newCacheTestNode(t, srv.URL, nil)

	prompts := []string{"alpha", "beta", "gamma"}
	var wg sync.WaitGroup
	for _, p := range prompts {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if _, err := n.Execute(context.Background(), p, map[string]string{"api_key": "sk-test", "endpoint": srv.URL}); err != nil {
				t.Errorf("Execute(%q) failed: %v", p, err)
			}
		}(p)
	}
	wg.Wait()

	if got := calls.Load(); got != int64(len(prompts)) {
		t.Errorf("upstream calls = %d, want %d (different requests must not be deduplicated)", got, len(prompts))
	}
}

// TestLLMSingleflight_InactiveWithoutCache pins the gating: when the LLM
// response cache is off (the default), concurrent identical requests each
// hit upstream. Deployments that did not opt into "identical request →
// shared response" semantics — e.g. a best-of-N fan-out relying on
// independent samples of the same prompt — keep their behaviour.
func TestLLMSingleflight_InactiveWithoutCache(t *testing.T) {
	t.Setenv("AFLARE_LLM_CACHE", "")

	srv, calls := newSlowCountingMockLLM(t, "solo", 60*time.Millisecond)
	n := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            "dedupoff",
		DefaultModel:    "test-model",
		DefaultEndpoint: srv.URL,
		EnvAPIKey:       "AFLARE_TEST_UNIQUE_API_KEY_NEVER_SET",
		ProviderName:    "TestProvider",
	})
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	const callers = 4
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := n.Execute(context.Background(), "hello", params); err != nil {
				t.Errorf("Execute failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := calls.Load(); got != callers {
		t.Errorf("upstream calls = %d, want %d (dedup must be inactive without the cache)", got, callers)
	}
}

// TestLLMSingleflight_FollowerCancellationReturnsContextError verifies
// that a caller whose context is cancelled while waiting for the shared
// flight returns promptly with the context error, while the flight itself
// completes for the remaining callers (and caches its response).
func TestLLMSingleflight_FollowerCancellationReturnsContextError(t *testing.T) {
	srv, calls := newSlowCountingMockLLM(t, "leader-answer", 300*time.Millisecond)
	c := cache.New(cache.Config{Enabled: true, MaxEntries: 100, TTL: time.Hour})
	n := newCacheTestNode(t, srv.URL, c)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	leaderDone := make(chan error, 1)
	go func() {
		out, err := n.Execute(context.Background(), "hello", params)
		if err == nil && out != "leader-answer" {
			err = errors.New("unexpected leader output: " + out)
		}
		leaderDone <- err
	}()

	// Give the leader time to register its flight before the follower
	// arrives, so the follower joins (rather than starts) the flight.
	time.Sleep(50 * time.Millisecond)

	followerCtx, cancel := context.WithCancel(context.Background())
	followerDone := make(chan error, 1)
	go func() {
		_, err := n.Execute(followerCtx, "hello", params)
		followerDone <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-followerDone:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("follower error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follower did not return promptly after its context was cancelled")
	}

	// The leader must be unaffected by the follower's cancellation.
	select {
	case err := <-leaderDone:
		if err != nil {
			t.Fatalf("leader failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("leader did not complete")
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1", got)
	}

	// The completed (and now cached) response is served to the next
	// identical request from the cache, without another upstream call.
	out, err := n.Execute(context.Background(), "hello", params)
	if err != nil || out != "leader-answer" {
		t.Errorf("post-flight Execute = (%q, %v), want (leader-answer, nil)", out, err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls after cache hit = %d, want 1", got)
	}
}

// TestLLMSingleflight_EachCallerRecordsTelemetry verifies that every
// deduplicated caller still publishes its own telemetry record with the
// shared flight's outcome — a workflow fanning out N identical calls sees
// N trace entries, exactly as it did before dedup existed.
func TestLLMSingleflight_EachCallerRecordsTelemetry(t *testing.T) {
	srv, calls := newSlowCountingMockLLM(t, "tel-answer", 100*time.Millisecond)
	n := newCacheTestNode(t, srv.URL, nil)
	sink := &recordingSink{}
	ctx := WithLLMCallSink(context.Background(), sink)
	params := map[string]string{"api_key": "sk-test", "endpoint": srv.URL}

	const callers = 3
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := n.Execute(ctx, "hello", params); err != nil {
				t.Errorf("Execute failed: %v", err)
			}
		}()
	}
	wg.Wait()

	records := sink.snapshot()
	if len(records) != callers {
		t.Fatalf("telemetry records = %d, want %d (one per caller)", len(records), callers)
	}
	for i, c := range records {
		if c.Response != "tel-answer" {
			t.Errorf("record %d Response = %q, want tel-answer", i, c.Response)
		}
		if c.StatusCode != http.StatusOK {
			t.Errorf("record %d StatusCode = %d, want 200", i, c.StatusCode)
		}
		if c.ErrText != "" {
			t.Errorf("record %d ErrText = %q, want empty", i, c.ErrText)
		}
		if c.Latency <= 0 {
			t.Errorf("record %d Latency = %v, want > 0", i, c.Latency)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1", got)
	}
}
