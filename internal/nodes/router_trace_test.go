// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​‌​‌‌​​​​​‌‌‌​‌​​‌​​​​‌​​‌​​‌​​​‌​‌‌​‌​​‌‌​‌​​‌​​​​​​​​​​​​​​​​​‌​​​‌‌​‌​​‌‌‌‌​⁠
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
	"strings"
	"sync"
	"testing"

	"github.com/alib8b8/aflare/internal/config"
)

// recordingRouterSink is a thread-safe RouterDecisionSink used in tests to
// capture every decision published by the router.
type recordingRouterSink struct {
	mu        sync.Mutex
	decisions []RouterDecision
}

func (s *recordingRouterSink) RecordRouterDecision(d RouterDecision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions = append(s.decisions, d)
}

func (s *recordingRouterSink) snapshot() []RouterDecision {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]RouterDecision(nil), s.decisions...)
}

// TestRouterDecisionSink_NoopDefault verifies that a context without an
// explicit sink returns a non-nil no-op sink, so router code can call
// RecordRouterDecision unconditionally without nil checks.
func TestRouterDecisionSink_NoopDefault(t *testing.T) {
	s := RouterDecisionSinkFrom(context.Background())
	if s == nil {
		t.Fatal("RouterDecisionSinkFrom returned nil for bare context")
	}
	// Must not panic.
	s.RecordRouterDecision(RouterDecision{Strategy: "priority"})
}

// TestRouterDecisionSink_RoundTrip verifies that a sink attached via
// WithRouterDecisionSink is the same one returned by RouterDecisionSinkFrom.
func TestRouterDecisionSink_RoundTrip(t *testing.T) {
	sink := &recordingRouterSink{}
	ctx := WithRouterDecisionSink(context.Background(), sink)
	if got := RouterDecisionSinkFrom(ctx); got != sink {
		t.Error("RouterDecisionSinkFrom did not return the attached sink")
	}
}

// TestRouterDecisionSink_NilRestoresNoop verifies that passing nil to
// WithRouterDecisionSink installs the no-op default.
func TestRouterDecisionSink_NilRestoresNoop(t *testing.T) {
	ctx := WithRouterDecisionSink(context.Background(), nil)
	s := RouterDecisionSinkFrom(ctx)
	if s == nil {
		t.Fatal("nil sink produced nil default")
	}
	// Should not panic.
	s.RecordRouterDecision(RouterDecision{Strategy: "priority"})
}

// TestLLMRouter_PublishesDecisionOnNoProviders verifies the B-3 contract:
// when the router has no active providers it still publishes a decision
// (with empty candidates and a FinalError) so the workflow trace records
// that the router was invoked.
func TestLLMRouter_PublishesDecisionOnNoProviders(t *testing.T) {
	r := &LLMRouter{
		providers: []RouterProvider{},
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	sink := &recordingRouterSink{}
	ctx := WithRouterDecisionSink(context.Background(), sink)

	_, _, err := r.Execute(ctx, "hello", nil)
	if err == nil {
		t.Fatal("expected error when no providers, got nil")
	}

	decisions := sink.snapshot()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	d := decisions[0]
	if d.Strategy != config.RouterStrategyPriority {
		t.Errorf("Strategy=%q want %q", d.Strategy, config.RouterStrategyPriority)
	}
	if len(d.Candidates) != 0 {
		t.Errorf("Candidates=%v want empty", d.Candidates)
	}
	if d.Selected != "" {
		t.Errorf("Selected=%q want empty", d.Selected)
	}
	if !strings.Contains(d.FinalError, "no active LLM providers") {
		t.Errorf("FinalError=%q should mention no active providers", d.FinalError)
	}
}

// TestLLMRouter_PublishesDecisionOnAllFailures verifies the B-3 contract:
// when every provider fails (here because none have API keys), the router
// publishes a decision listing all candidates, no Selected, the per-provider
// attempts, and a FinalError summarizing the outcome.
func TestLLMRouter_PublishesDecisionOnAllFailures(t *testing.T) {
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, APIKey: "", Priority: 3},
		{Name: "anthropic", Enabled: true, APIKey: "", Priority: 2},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	sink := &recordingRouterSink{}
	ctx := WithRouterDecisionSink(context.Background(), sink)

	_, _, err := r.Execute(ctx, "input", nil)
	if err == nil {
		t.Fatal("expected error when all providers fail, got nil")
	}

	decisions := sink.snapshot()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	d := decisions[0]
	if d.Strategy != config.RouterStrategyPriority {
		t.Errorf("Strategy=%q want priority", d.Strategy)
	}
	if len(d.Candidates) != 2 {
		t.Fatalf("Candidates len=%d want 2", len(d.Candidates))
	}
	// Priority strategy orders by descending priority: openai (3) first.
	if d.Candidates[0] != "openai" || d.Candidates[1] != "anthropic" {
		t.Errorf("Candidates order=%v want [openai anthropic]", d.Candidates)
	}
	if d.Selected != "" {
		t.Errorf("Selected=%q want empty on all-failed", d.Selected)
	}
	if len(d.Attempts) != 2 {
		t.Fatalf("Attempts len=%d want 2", len(d.Attempts))
	}
	for i, a := range d.Attempts {
		if a.Success {
			t.Errorf("Attempt %d: Success=true want false", i)
		}
		if !strings.Contains(a.Error, "no API key") {
			t.Errorf("Attempt %d Error=%q should mention no API key", i, a.Error)
		}
	}
	if d.FinalError == "" {
		t.Error("FinalError should be non-empty on all-failed")
	}
}
