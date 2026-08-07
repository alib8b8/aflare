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
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // normal operation, requests pass through
	CircuitOpen                         // tripped, requests are rejected immediately
	CircuitHalfOpen                     // allowing a limited number of probe requests
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures a provider circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures (within the
	// window) that trips the breaker from Closed to Open.
	FailureThreshold int
	// OpenTimeout is how long the breaker stays Open before transitioning
	// to HalfOpen (allowing probe requests).
	OpenTimeout time.Duration
	// HalfOpenMaxRequests is the number of probe requests allowed in
	// HalfOpen state. If all succeed, the breaker closes. If any fails,
	// it re-opens.
	HalfOpenMaxRequests int
	// WindowSize is the sliding window size for failure counting.
	// Failures older than this window are forgotten.
	WindowSize time.Duration
}

// DefaultCircuitBreakerConfig returns production-ready defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		OpenTimeout:         30 * time.Second,
		HalfOpenMaxRequests: 3,
		WindowSize:          2 * time.Minute,
	}
}

// CircuitBreaker is a per-provider circuit breaker with CLOSED/OPEN/HALF_OPEN
// states. It uses a sliding window for failure counting and supports probe
// requests in HalfOpen state.
//
// State transitions:
//
//	Closed   --(failures >= threshold)-->     Open
//	Open     --(OpenTimeout elapsed)-->       HalfOpen
//	HalfOpen --(probe success)-->             Closed
//	HalfOpen --(probe failure)-->             Open
type CircuitBreaker struct {
	mu               sync.Mutex
	config           CircuitBreakerConfig
	state            CircuitState
	failures         []time.Time // timestamps of recent failures (sliding window)
	openedAt         time.Time   // when the breaker transitioned to Open
	halfOpenInFlight int         // probe requests currently in flight (HalfOpen)
	halfOpenSuccess  int         // successful probes in current HalfOpen window
}

// NewCircuitBreaker creates a breaker with the given config.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.OpenTimeout <= 0 {
		config.OpenTimeout = 30 * time.Second
	}
	if config.HalfOpenMaxRequests <= 0 {
		config.HalfOpenMaxRequests = 3
	}
	if config.WindowSize <= 0 {
		config.WindowSize = 2 * time.Minute
	}
	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
	}
}

// AllowRequest checks whether a request should be allowed through.
// Returns true if the request may proceed, false if the breaker is Open.
// In HalfOpen state, allows up to HalfOpenMaxRequests concurrent probes.
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.openedAt) >= cb.config.OpenTimeout {
			// Transition to HalfOpen
			cb.state = CircuitHalfOpen
			cb.halfOpenInFlight = 0
			cb.halfOpenSuccess = 0
			cb.halfOpenInFlight++
			return true
		}
		return false
	case CircuitHalfOpen:
		if cb.halfOpenInFlight < cb.config.HalfOpenMaxRequests {
			cb.halfOpenInFlight++
			return true
		}
		return false
	}
	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		// Clear failures on any success (sliding window also handles expiry)
		cb.failures = cb.failures[:0]
	case CircuitHalfOpen:
		cb.halfOpenSuccess++
		cb.halfOpenInFlight--
		if cb.halfOpenSuccess >= cb.config.HalfOpenMaxRequests {
			// All probes succeeded, close the circuit
			cb.state = CircuitClosed
			cb.failures = cb.failures[:0]
			cb.halfOpenInFlight = 0
			cb.halfOpenSuccess = 0
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failures = append(cb.failures, now)
		cb.pruneFailures(now)
		if len(cb.failures) >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
			cb.openedAt = now
		}
	case CircuitHalfOpen:
		// Any failure in HalfOpen re-opens the circuit
		cb.state = CircuitOpen
		cb.openedAt = now
		cb.halfOpenInFlight = 0
		cb.halfOpenSuccess = 0
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// pruneFailures removes failures older than the sliding window.
func (cb *CircuitBreaker) pruneFailures(now time.Time) {
	cutoff := now.Add(-cb.config.WindowSize)
	// failures are in chronological order, find the first one within window
	idx := 0
	for idx < len(cb.failures) && cb.failures[idx].Before(cutoff) {
		idx++
	}
	if idx > 0 {
		cb.failures = cb.failures[idx:]
	}
}

// FailureCount returns the number of failures in the current sliding window.
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.pruneFailures(time.Now())
	return len(cb.failures)
}
