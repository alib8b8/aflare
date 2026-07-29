// Copyright (c) 2026 llm-box Contributors
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
	"math"
	"sync"
)

// EWMA predictor tracks exponentially-weighted moving average of latency.
// Recent observations weigh more than old ones, so the predictor adapts
// quickly to performance changes (e.g. a provider degradation).
//
// The decay factor alpha controls the half-life: alpha=0.3 means a new
// observation contributes 30% to the average, and the effective half-life
// is ~log(0.5)/log(1-0.3) ≈ 1.94 observations.
type EWMAPredictor struct {
	mu          sync.Mutex
	alpha       float64 // smoothing factor (0,1]
	ewma        float64 // current EWMA value (ms)
	ewmaVar     float64 // EWMA of squared deviation (for stddev)
	count       int64   // total observations
	initialized bool    // whether ewma has been set
}

// NewEWMAPredictor creates a predictor with the given smoothing factor.
// alpha=0.3 is a good default (responds quickly but not jittery).
func NewEWMAPredictor(alpha float64) *EWMAPredictor {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.3
	}
	return &EWMAPredictor{alpha: alpha}
}

// Observe records a new latency observation (in milliseconds).
func (p *EWMAPredictor) Observe(latencyMs float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.initialized {
		p.ewma = latencyMs
		p.ewmaVar = 0
		p.initialized = true
	} else {
		delta := latencyMs - p.ewma
		p.ewma += p.alpha * delta
		p.ewmaVar = (1 - p.alpha) * (p.ewmaVar + p.alpha*delta*delta)
	}
	p.count++
}

// Predict returns the current EWMA latency prediction (in ms).
// Returns 0 if no observations have been made yet.
func (p *EWMAPredictor) Predict() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ewma
}

// StdDev returns the EWMA standard deviation of latency.
func (p *EWMAPredictor) StdDev() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return math.Sqrt(p.ewmaVar)
}

// Count returns the total number of observations.
func (p *EWMAPredictor) Count() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

// P95Estimate returns an approximate p95 latency using the EWMA and stddev,
// assuming a normal distribution: p95 ≈ ewma + 1.645 * stddev.
// This is an approximation (real latency distributions are log-normal), but
// it's far better than the arithmetic mean for routing decisions.
func (p *EWMAPredictor) P95Estimate() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.initialized {
		return 0
	}
	return p.ewma + 1.645*math.Sqrt(p.ewmaVar)
}

// Reset clears all observations.
func (p *EWMAPredictor) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ewma = 0
	p.ewmaVar = 0
	p.count = 0
	p.initialized = false
}
