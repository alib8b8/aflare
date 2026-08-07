// Copyright (c) 2026 aflare Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
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
	"sync"
	"testing"
	"time"
)

// newTestBreaker builds a breaker with short timeouts so the Open->HalfOpen
// transition can be exercised without slowing down the test suite.
func newTestBreaker() *CircuitBreaker {
	return NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold:    3,
		OpenTimeout:         20 * time.Millisecond,
		HalfOpenMaxRequests: 2,
		WindowSize:          time.Minute,
	})
}

// TestCircuitBreaker_ClosedToOpen verifies that FailureThreshold failures
// transition the breaker from Closed to Open, and that AllowRequest rejects
// once Open.
func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := newTestBreaker()
	if cb.State() != CircuitClosed {
		t.Fatalf("initial state = %s, want closed", cb.State())
	}

	// Two failures: below threshold, stays closed.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Errorf("state after 2 failures = %s, want closed", cb.State())
	}

	// Third failure trips the breaker.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("state after 3 failures = %s, want open", cb.State())
	}
}

// TestCircuitBreaker_AllowRequestRejectsOpen verifies that AllowRequest
// returns false while the breaker is Open (before OpenTimeout elapses).
func TestCircuitBreaker_AllowRequestRejectsOpen(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("state = %s, want open", cb.State())
	}
	if cb.AllowRequest() {
		t.Error("AllowRequest() = true on Open breaker, want false")
	}
}

// TestCircuitBreaker_OpenToHalfOpen verifies that after OpenTimeout elapses,
// AllowRequest returns true and transitions the breaker to HalfOpen.
func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Fatalf("state = %s, want open", cb.State())
	}

	// Wait for OpenTimeout to elapse.
	time.Sleep(30 * time.Millisecond)

	// First AllowRequest after timeout transitions to HalfOpen and is allowed.
	if !cb.AllowRequest() {
		t.Error("AllowRequest() after OpenTimeout = false, want true")
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("state after post-timeout AllowRequest = %s, want half-open", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenToClosed verifies that HalfOpenMaxRequests
// successful probes transition the breaker back to Closed.
func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	time.Sleep(30 * time.Millisecond)

	// Allow the configured number of probe requests (2), each succeeding.
	for i := 0; i < 2; i++ {
		if !cb.AllowRequest() {
			t.Fatalf("probe %d: AllowRequest() = false, want true", i)
		}
		cb.RecordSuccess()
	}
	if cb.State() != CircuitClosed {
		t.Errorf("state after %d successful probes = %s, want closed", 2, cb.State())
	}
}

// TestCircuitBreaker_HalfOpenToOpen verifies that a single failure during
// HalfOpen re-opens the breaker immediately.
func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	time.Sleep(30 * time.Millisecond)

	if !cb.AllowRequest() {
		t.Fatal("AllowRequest() after timeout = false, want true")
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("state = %s, want half-open", cb.State())
	}

	// A probe failure re-opens the circuit.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("state after HalfOpen failure = %s, want open", cb.State())
	}
}

// TestCircuitBreaker_SlidingWindow verifies that failures older than the
// window are forgotten and do not count toward tripping the breaker.
func TestCircuitBreaker_SlidingWindow(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold:    3,
		OpenTimeout:         30 * time.Second,
		HalfOpenMaxRequests: 2,
		WindowSize:          30 * time.Millisecond,
	})

	// Two failures, then wait for them to age out of the window.
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(40 * time.Millisecond)

	// The aged failures should have been pruned, so two more failures (4
	// total but only 2 in-window) must not trip the breaker.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Errorf("state after window-expired failures = %s, want closed", cb.State())
	}
	// FailureCount reflects only in-window failures.
	if got := cb.FailureCount(); got != 2 {
		t.Errorf("FailureCount() = %v, want 2 (only in-window failures)", got)
	}

	// One more in-window failure trips it.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("state after 3rd in-window failure = %s, want open", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenMaxRequests verifies that HalfOpen caps the
// number of concurrent probe requests allowed.
func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	time.Sleep(30 * time.Millisecond)

	// HalfOpenMaxRequests is 2: the first two probes are allowed, the third
	// is rejected until an in-flight probe completes.
	if !cb.AllowRequest() {
		t.Error("probe 1: AllowRequest() = false, want true")
	}
	if !cb.AllowRequest() {
		t.Error("probe 2: AllowRequest() = false, want true")
	}
	if cb.AllowRequest() {
		t.Error("probe 3: AllowRequest() = true, want false (max probes in flight)")
	}

	// Completing one probe frees a slot.
	cb.RecordSuccess()
	if !cb.AllowRequest() {
		t.Error("probe after success: AllowRequest() = false, want true (slot freed)")
	}
}

// TestCircuitBreaker_SuccessClearsFailures verifies that a success in Closed
// state clears the failure history (so a provider with intermittent failures
// is not penalized cumulatively).
func TestCircuitBreaker_SuccessClearsFailures(t *testing.T) {
	cb := newTestBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	if got := cb.FailureCount(); got != 2 {
		t.Fatalf("FailureCount() = %v, want 2", got)
	}
	cb.RecordSuccess()
	if got := cb.FailureCount(); got != 0 {
		t.Errorf("FailureCount() after success = %v, want 0 (success clears failures)", got)
	}
	// After clearing, threshold failures are needed to trip again.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Errorf("state after clear+2 failures = %s, want closed (threshold not reached)", cb.State())
	}
}

// TestCircuitBreaker_Concurrent verifies the breaker is safe under concurrent
// access (run under -race).
func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := newTestBreaker()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cb.AllowRequest()
			if n%2 == 0 {
				cb.RecordSuccess()
			} else {
				cb.RecordFailure()
			}
			_ = cb.State()
		}(i)
	}
	wg.Wait()
	// No data race panic is the success criterion; state is non-deterministic.
}

// TestCircuitBreaker_DefaultConfig verifies that DefaultCircuitBreakerConfig
// produces a working breaker with sensible defaults.
func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %v, want 5", cfg.FailureThreshold)
	}
	if cfg.OpenTimeout != 30*time.Second {
		t.Errorf("OpenTimeout = %v, want 30s", cfg.OpenTimeout)
	}
	if cfg.HalfOpenMaxRequests != 3 {
		t.Errorf("HalfOpenMaxRequests = %v, want 3", cfg.HalfOpenMaxRequests)
	}

	cb := NewCircuitBreaker(cfg)
	if cb.State() != CircuitClosed {
		t.Errorf("state = %s, want closed", cb.State())
	}
	// 4 failures: below default threshold of 5, stays closed.
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitClosed {
		t.Errorf("state after 4 failures = %s, want closed", cb.State())
	}
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("state after 5 failures = %s, want open", cb.State())
	}
}

// TestCircuitState_String verifies the string representation of each state.
func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
