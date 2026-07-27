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

package distributed

import (
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker()
	if cb == nil {
		t.Fatal("expected non-nil circuit breaker")
	}
	if cb.State() != BreakerClosed {
		t.Errorf("expected initial state Closed, got %v", cb.State())
	}
	if !cb.AllowRequest() {
		t.Error("expected AllowRequest to return true in Closed state")
	}
}

func TestCircuitBreaker_NilSafety(t *testing.T) {
	var cb *CircuitBreaker
	if !cb.AllowRequest() {
		t.Error("nil breaker should allow requests")
	}
	cb.RecordSuccess()
	if cb.RecordFailure() {
		t.Error("nil breaker should not trip")
	}
	if cb.State() != BreakerClosed {
		t.Error("nil breaker state should be Closed")
	}
	stats := cb.Stats()
	if stats.State != BreakerClosed {
		t.Error("nil breaker stats state should be Closed")
	}
}

func TestCircuitBreaker_TripOnFailures(t *testing.T) {
	cb := NewCircuitBreaker()

	for i := 0; i < 4; i++ {
		if tripped := cb.RecordFailure(); tripped {
			t.Errorf("should not trip after %d failures", i+1)
		}
		if cb.State() != BreakerClosed {
			t.Errorf("should still be Closed after %d failures", i+1)
		}
	}

	if tripped := cb.RecordFailure(); !tripped {
		t.Error("should trip after 5 failures")
	}
	if cb.State() != BreakerOpen {
		t.Errorf("expected Open state after trip, got %v", cb.State())
	}
	if cb.AllowRequest() {
		t.Error("should not allow requests in Open state")
	}
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker()

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	stats := cb.Stats()
	if stats.Failures != 0 {
		t.Errorf("expected failures reset to 0 after success, got %d", stats.Failures)
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.cooldown = 10 * time.Millisecond

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	if cb.State() != BreakerOpen {
		t.Fatal("expected Open state")
	}

	time.Sleep(20 * time.Millisecond)

	if !cb.AllowRequest() {
		t.Error("should allow probe request after cooldown")
	}
	if cb.State() != BreakerHalfOpen {
		t.Errorf("expected HalfOpen state, got %v", cb.State())
	}

	if cb.AllowRequest() {
		t.Error("should not allow second probe while one is in flight")
	}

	cb.RecordSuccess()
	if cb.State() != BreakerHalfOpen {
		t.Errorf("expected still HalfOpen after 1 success, got %v", cb.State())
	}

	cb.RecordSuccess()
	if cb.State() != BreakerClosed {
		t.Errorf("expected Closed after 2 successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.cooldown = 10 * time.Millisecond

	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	time.Sleep(20 * time.Millisecond)
	cb.AllowRequest()

	if tripped := cb.RecordFailure(); !tripped {
		t.Error("should re-trip on HalfOpen failure")
	}
	if cb.State() != BreakerOpen {
		t.Errorf("expected Open after HalfOpen failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker()

	stats := cb.Stats()
	if stats.IsTripped {
		t.Error("should not be tripped initially")
	}
	if stats.Failures != 0 {
		t.Errorf("expected 0 failures, got %d", stats.Failures)
	}

	cb.RecordFailure()
	stats = cb.Stats()
	if stats.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", stats.Failures)
	}
	if stats.LastFailure == nil {
		t.Error("expected LastFailure to be set")
	}

	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	stats = cb.Stats()
	if !stats.IsTripped {
		t.Error("should be tripped after 5 failures")
	}
	if stats.TrippedAt == nil {
		t.Error("expected TrippedAt to be set")
	}
}

func TestBreakerRegistry_Basic(t *testing.T) {
	reg := NewBreakerRegistry()
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}

	if !reg.AllowRequest("node1") {
		t.Error("new node should allow requests")
	}

	reg.RecordSuccess("node1")

	for i := 0; i < 5; i++ {
		reg.RecordFailure("node1")
	}

	if reg.AllowRequest("node1") {
		t.Error("node1 should be blocked after 5 failures")
	}

	stats := reg.StatsAll()
	if len(stats) != 1 {
		t.Errorf("expected 1 breaker, got %d", len(stats))
	}
	if !stats["node1"].IsTripped {
		t.Error("node1 should be tripped")
	}

	reg.Remove("node1")
	stats = reg.StatsAll()
	if len(stats) != 0 {
		t.Errorf("expected 0 breakers after Remove, got %d", len(stats))
	}
}

func TestBreakerRegistry_NilSafety(t *testing.T) {
	var reg *BreakerRegistry
	if !reg.AllowRequest("any") {
		t.Error("nil registry should allow requests")
	}
	reg.RecordSuccess("any")
	if reg.RecordFailure("any") {
		t.Error("nil registry should not trip")
	}
	reg.Remove("any")
	if reg.StatsAll() != nil {
		t.Error("nil registry StatsAll should return nil")
	}
}
