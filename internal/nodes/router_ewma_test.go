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
	"testing"
)

// TestEWMAPredictor_BasicConvergence verifies that a stream of identical
// observations converges to that value. With three observations of 100ms,
// the first seeds the EWMA to exactly 100, and subsequent observations
// (delta=0) leave it unchanged.
func TestEWMAPredictor_BasicConvergence(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	for i := 0; i < 3; i++ {
		p.Observe(100)
	}
	// First observation sets ewma=100; later ones have delta=0, so ewma
	// stays exactly 100.
	if got := p.Predict(); got != 100 {
		t.Errorf("Predict() = %v, want 100 (constant stream should converge exactly)", got)
	}
	if got := p.Count(); got != 3 {
		t.Errorf("Count() = %v, want 3", got)
	}
}

// TestEWMAPredictor_AdaptsToChange verifies that the EWMA shifts toward more
// recent observations. After [100,100,100,200,200,200] the prediction must
// be biased toward 200 (i.e. > 150, the midpoint), proving old observations
// are decayed rather than arithmetically averaged (which would give exactly
// 150).
func TestEWMAPredictor_AdaptsToChange(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	for _, v := range []float64{100, 100, 100, 200, 200, 200} {
		p.Observe(v)
	}
	got := p.Predict()
	// The arithmetic mean would be 150. EWMA with alpha=0.3 weights recent
	// observations more heavily, so the prediction must exceed 150.
	if got <= 150 {
		t.Errorf("Predict() = %v, want > 150 (EWMA should bias toward recent 200ms observations)", got)
	}
	// And it must stay below 200 (the most recent value) since older 100ms
	// observations still exert some pull.
	if got >= 200 {
		t.Errorf("Predict() = %v, want < 200 (older observations should still pull down)", got)
	}
}

// TestEWMAPredictor_P95Estimate verifies that P95 is strictly greater than
// the EWMA mean whenever there is variance in the observations.
func TestEWMAPredictor_P95Estimate(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	// Mixed observations produce non-zero variance, so p95 > ewma.
	for _, v := range []float64{50, 150, 80, 120} {
		p.Observe(v)
	}
	ewma := p.Predict()
	p95 := p.P95Estimate()
	if p95 <= ewma {
		t.Errorf("P95Estimate() = %v, want > EWMA %v (p95 must exceed mean when variance > 0)", p95, ewma)
	}
}

// TestEWMAPredictor_P95Estimate_NoData verifies that P95Estimate returns 0
// before any observation has been recorded.
func TestEWMAPredictor_P95Estimate_NoData(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	if got := p.P95Estimate(); got != 0 {
		t.Errorf("P95Estimate() with no data = %v, want 0", got)
	}
	if got := p.Predict(); got != 0 {
		t.Errorf("Predict() with no data = %v, want 0", got)
	}
}

// TestEWMAPredictor_Reset verifies that Reset clears all state: Predict
// returns 0 and Count returns 0.
func TestEWMAPredictor_Reset(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	p.Observe(100)
	p.Observe(200)
	if p.Count() != 2 {
		t.Fatalf("Count() before reset = %v, want 2", p.Count())
	}

	p.Reset()

	if got := p.Predict(); got != 0 {
		t.Errorf("Predict() after Reset = %v, want 0", got)
	}
	if got := p.Count(); got != 0 {
		t.Errorf("Count() after Reset = %v, want 0", got)
	}
	if got := p.P95Estimate(); got != 0 {
		t.Errorf("P95Estimate() after Reset = %v, want 0", got)
	}
	// After reset, the predictor should behave as fresh: a single
	// observation seeds the EWMA exactly.
	p.Observe(300)
	if got := p.Predict(); got != 300 {
		t.Errorf("Predict() after reset+observe(300) = %v, want 300 (first obs seeds exactly)", got)
	}
}

// TestEWMAPredictor_StdDev verifies StdDev is non-negative and grows with
// more dispersed observations.
func TestEWMAPredictor_StdDev(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	// Constant stream => zero variance/stddev.
	p.Observe(100)
	p.Observe(100)
	if got := p.StdDev(); got != 0 {
		t.Errorf("StdDev() for constant stream = %v, want 0", got)
	}

	// Dispersed stream => positive stddev.
	p2 := NewEWMAPredictor(0.3)
	for _, v := range []float64{50, 150, 50, 150} {
		p2.Observe(v)
	}
	if got := p2.StdDev(); got <= 0 {
		t.Errorf("StdDev() for dispersed stream = %v, want > 0", got)
	}
}

// TestEWMAPredictor_DefaultAlpha verifies that an invalid alpha falls back to
// the 0.3 default rather than producing a degenerate predictor.
func TestEWMAPredictor_DefaultAlpha(t *testing.T) {
	for _, alpha := range []float64{0, -1, 2, 100} {
		p := NewEWMAPredictor(alpha)
		p.Observe(100)
		p.Observe(200)
		got := p.Predict()
		// With alpha=0.3: first seeds 100, second => 100 + 0.3*(200-100) = 130.
		if got != 130 {
			t.Errorf("NewEWMAPredictor(%v): Predict() = %v, want 130 (default alpha=0.3)", alpha, got)
		}
	}
}

// TestEWMAPredictor_Concurrent verifies the predictor is safe under
// concurrent Observe/Predict calls (run under -race).
func TestEWMAPredictor_Concurrent(t *testing.T) {
	p := NewEWMAPredictor(0.3)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			p.Observe(float64(100 + i%10))
		}
	}()
	// Concurrent reads must not race with writes.
	for i := 0; i < 100; i++ {
		_ = p.Predict()
		_ = p.StdDev()
	}
	<-done
	if got := p.Count(); got != 100 {
		t.Errorf("Count() after concurrent observes = %v, want 100", got)
	}
}
