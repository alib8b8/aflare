// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​‌‌​​‌​‌​​‌‌​​​‌​​‌​​​​​‌​‌​‌‌​‌‌‌​​​‌​​​‌‌‌​​‌‌‌​‌‌‌​‌​​‌‌​​​‌‌​‌‌​​‌​​​​​​​​​​​​​​​​​​​‌‌‌‌​‌​​​‌​​​⁠
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

package agentx

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Per-agent circuit breaker for external delegations.
//
// One dead endpoint must not eat the full delegation timeout on every
// round of a 20-agent plan: after a few consecutive failures the agent's
// circuit opens and further delegations fail fast with a "circuit-open"
// error, until a cool-down elapses and a cheap health check (A2A: agent
// card fetch) proves the agent is back.
//
// State machine (per agent name, in-memory, resets with the process):
//
//	closed --threshold consecutive failures--> open
//	open  --cool-down elapsed--> half-open
//	half-open --probe/delegation success--> closed
//	half-open --probe/delegation failure--> open
//
// The breaker intentionally lives in agentx (not nodes) so it protects
// every delegation path: supervisor fan-out and the cli_agent/a2a_agent
// nodes alike.

const (
	// defaultBreakerThreshold is how many consecutive delegation
	// failures trip an agent's circuit.
	defaultBreakerThreshold = 3

	// defaultBreakerOpenTime is how long an open circuit stays open
	// before a half-open probe is allowed.
	defaultBreakerOpenTime = 60 * time.Second

	// breakerProbeTimeout bounds the half-open A2A agent-card probe.
	breakerProbeTimeout = 10 * time.Second
)

type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

// CircuitOpenError marks a delegation rejected by the breaker. The
// supervisor surfaces it like any other delegation failure, so failure
// policies (fail_on) and the results envelope keep working unchanged.
type CircuitOpenError struct {
	Agent     string
	Failures  int
	LastError string
	ProbeErr  error // half-open probe failure, when applicable
}

func (e *CircuitOpenError) Error() string {
	detail := fmt.Sprintf("agent %q: circuit-open after %d consecutive failures (last error: %s)", e.Agent, e.Failures, e.LastError)
	if e.ProbeErr != nil {
		detail = fmt.Sprintf("agent %q: circuit-open (half-open health check failed: %v)", e.Agent, e.ProbeErr)
	}
	return detail
}

// agentBreakerEntry is one agent's breaker state.
type agentBreakerEntry struct {
	state    breakerState
	failures int       // consecutive failures while closed
	lastErr  string    // most recent failure, for the fast-fail message
	openedAt time.Time // when the circuit (re)opened
}

// agentBreakerSet is the process-wide set of per-agent breakers.
type agentBreakerSet struct {
	mu        sync.Mutex
	threshold int
	openFor   time.Duration
	entries   map[string]*agentBreakerEntry
}

func newAgentBreakerSet(threshold int, openFor time.Duration) *agentBreakerSet {
	if threshold < 1 {
		threshold = 1
	}
	return &agentBreakerSet{
		threshold: threshold,
		openFor:   openFor,
		entries:   make(map[string]*agentBreakerEntry),
	}
}

// globalAgentBreakers guards every delegation issued by this process.
var globalAgentBreakers = newAgentBreakerSet(defaultBreakerThreshold, defaultBreakerOpenTime)

// allow reports whether a delegation to def may proceed. A non-nil error
// means the circuit is open: the caller must fail fast instead of
// dispatching. When the cool-down has elapsed the breaker goes half-open:
// A2A agents are probed with a cheap agent-card fetch (a real delegation
// is too expensive for a health check), CLI agents let the delegation
// itself act as the probe.
func (b *agentBreakerSet) allow(ctx context.Context, def AgentDef) error {
	b.mu.Lock()
	entry, ok := b.entries[def.Name]
	if !ok || entry.state == breakerClosed {
		if !ok {
			entry = &agentBreakerEntry{state: breakerClosed}
			b.entries[def.Name] = entry
		}
		b.mu.Unlock()
		return nil
	}
	if entry.state == breakerOpen {
		if time.Since(entry.openedAt) < b.openFor {
			err := &CircuitOpenError{Agent: def.Name, Failures: entry.failures, LastError: entry.lastErr}
			b.mu.Unlock()
			return err
		}
		entry.state = breakerHalfOpen
		b.mu.Unlock()
		// Half-open A2A agents get a cheap liveness probe first; the
		// probe result closes or re-opens the circuit.
		if def.Driver == DriverA2A {
			probeCtx, cancel := context.WithTimeout(ctx, breakerProbeTimeout)
			defer cancel()
			if _, perr := FetchAgentCard(probeCtx, def); perr != nil {
				b.mu.Lock()
				entry.state = breakerOpen
				entry.openedAt = time.Now()
				entry.lastErr = perr.Error()
				b.mu.Unlock()
				return &CircuitOpenError{Agent: def.Name, Failures: entry.failures, ProbeErr: perr}
			}
			b.record(def.Name, nil) // healthy again: close the circuit
		}
		return nil
	}
	// Half-open: allow the delegation through; record() decides whether
	// the circuit closes or re-opens.
	b.mu.Unlock()
	return nil
}

// record notes one delegation outcome. Success closes the circuit and
// resets the failure streak; a failure while closed increments the
// streak (tripping open at the threshold) and a failure while half-open
// re-opens immediately.
func (b *agentBreakerSet) record(name string, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.entries[name]
	if !ok {
		entry = &agentBreakerEntry{state: breakerClosed}
		b.entries[name] = entry
	}
	if err == nil {
		entry.state = breakerClosed
		entry.failures = 0
		entry.lastErr = ""
		return
	}
	switch entry.state {
	case breakerClosed:
		entry.failures++
		entry.lastErr = err.Error()
		if entry.failures >= b.threshold {
			entry.state = breakerOpen
			entry.openedAt = time.Now()
		}
	case breakerHalfOpen:
		entry.state = breakerOpen
		entry.openedAt = time.Now()
		entry.lastErr = err.Error()
	case breakerOpen:
		// Stale outcome from a delegation allowed before the circuit
		// opened; the failure is already accounted for.
	}
}

// resetAgentBreakers clears all breaker state. It exists for tests that
// need a pristine breaker between cases.
func resetAgentBreakers() {
	globalAgentBreakers.mu.Lock()
	defer globalAgentBreakers.mu.Unlock()
	globalAgentBreakers.entries = make(map[string]*agentBreakerEntry)
}

// setAgentBreakerPolicyForTest overrides the global breaker policy
// (failure threshold and cool-down) so tests can exercise transitions
// without real waits. Test-only.
func setAgentBreakerPolicyForTest(threshold int, openFor time.Duration) {
	globalAgentBreakers.mu.Lock()
	defer globalAgentBreakers.mu.Unlock()
	globalAgentBreakers.threshold = threshold
	globalAgentBreakers.openFor = openFor
}
