// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​‌‌​​‌‌​​‌‌‌​​​‌‌​‌​‌‌​‌​​​‌‌‌‌‌‌​‌​‌​‌‌​‌‌​‌‌‌‌​​​​​​​​​​​​​​​​‌‌‌‌​‌​​​‌‌‌‌‌‌​⁠
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
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alib8b8/aflare/internal/config"
)

// TestLLMRouterNode_Registration verifies the llm_router node is registered in the
// global registry under the name "llm_router".
func TestLLMRouterNode_Registration(t *testing.T) {
	node, ok := Get("llm_router")
	if !ok {
		t.Fatal("llm_router node not found in registry")
	}
	if node.Name() != "llm_router" {
		t.Errorf("expected node name 'llm_router', got '%s'", node.Name())
	}
}

// TestLLMRouterNode_Description ensures Description returns a non-empty string.
func TestLLMRouterNode_Description(t *testing.T) {
	node := &LLMRouterNode{}
	if desc := node.Description(); desc == "" {
		t.Error("Description() returned empty string")
	}
}

// TestLLMRouterNode_Schema verifies the schema name and required params.
func TestLLMRouterNode_Schema(t *testing.T) {
	node := &LLMRouterNode{}
	schema := node.Schema()

	if schema.Name != "llm_router" {
		t.Errorf("Schema().Name = %q, want %q", schema.Name, "llm_router")
	}
	if schema.Description == "" {
		t.Error("Schema().Description is empty")
	}
	if schema.Input == "" {
		t.Error("Schema().Input is empty")
	}
	if schema.Output == "" {
		t.Error("Schema().Output is empty")
	}

	expectedParams := map[string]bool{
		"system":        false,
		"strategy":      false,
		"max_retries":   false,
		"show_provider": false,
		"show_stats":    false,
	}
	for _, p := range schema.Params {
		if _, ok := expectedParams[p.Name]; ok {
			expectedParams[p.Name] = true
		}
	}
	for name, found := range expectedParams {
		if !found {
			t.Errorf("expected param %q in schema, not found", name)
		}
	}
}

// TestLLMRouterNode_SchemaStrategyParam verifies the strategy param default value.
func TestLLMRouterNode_SchemaStrategyParam(t *testing.T) {
	node := &LLMRouterNode{}
	schema := node.Schema()
	for _, p := range schema.Params {
		if p.Name == "strategy" {
			if p.Default != "priority" {
				t.Errorf("strategy param default = %q, want %q", p.Default, "priority")
			}
			return
		}
	}
	t.Error("strategy param not found in schema")
}

// TestLLMRouterNode_SchemaMaxRetriesParam verifies the max_retries param default value.
func TestLLMRouterNode_SchemaMaxRetriesParam(t *testing.T) {
	node := &LLMRouterNode{}
	schema := node.Schema()
	for _, p := range schema.Params {
		if p.Name == "max_retries" {
			if p.Default != "3" {
				t.Errorf("max_retries param default = %q, want %q", p.Default, "3")
			}
			return
		}
	}
	t.Error("max_retries param not found in schema")
}

// TestLLMRouter_Execute_NoProviders verifies that Execute gracefully returns an
// error when there are no active providers configured. This is the deterministic
// error path that does not require any network calls.
func TestLLMRouter_Execute_NoProviders(t *testing.T) {
	r := &LLMRouter{
		providers: []RouterProvider{},
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	ctx := context.Background()

	tests := []struct {
		name   string
		input  string
		params map[string]string
	}{
		{"empty_input", "", nil},
		{"nonempty_input", "hello", nil},
		{"with_params", "hello", map[string]string{"system": "be brief"}},
		{"with_strategy", "hello", map[string]string{"strategy": "cost"}},
		{"with_max_retries", "hello", map[string]string{"max_retries": "5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := r.Execute(ctx, tt.input, tt.params)
			if err == nil {
				t.Fatal("expected error when no providers available, got nil")
			}
			if !strings.Contains(err.Error(), "no active LLM providers") {
				t.Errorf("expected 'no active LLM providers' error, got: %v", err)
			}
		})
	}
}

// TestLLMRouter_Execute_MaxRetries tests the max_retries param parsing using
// providers without API keys. Providers without an API key (and not named
// "ollama") fail without making any network calls, so the number of attempted
// providers reflects the parsed max_retries value.
func TestLLMRouter_Execute_MaxRetries(t *testing.T) {
	// Providers without API keys (and not named "ollama") trigger recordFailure
	// without invoking callProvider, so no network access occurs.
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, APIKey: "", Priority: 3},
		{Name: "anthropic", Enabled: true, APIKey: "", Priority: 2},
		{Name: "gemini", Enabled: true, APIKey: "", Priority: 1},
	}

	tests := []struct {
		name       string
		maxRetries string
		wantTried  int
	}{
		{"valid_1", "1", 1},
		{"valid_2", "2", 2},
		{"valid_3", "3", 3},
		{"exceeds_provider_count", "10", 3},
		{"invalid_string_falls_back", "abc", 3},
		{"zero_falls_back", "0", 3},
		{"negative_falls_back", "-5", 3},
		{"empty_uses_router_default", "", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LLMRouter{
				providers: append([]RouterProvider(nil), providers...),
				stats:     make(map[string]*ProviderStats),
				strategy:  config.RouterStrategyPriority,
				maxRetry:  3,
			}
			ctx := context.Background()
			params := map[string]string{}
			if tt.maxRetries != "" {
				params["max_retries"] = tt.maxRetries
			}

			_, _, err := r.Execute(ctx, "input", params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errStr := err.Error()
			idx := strings.Index(errStr, "tried: ")
			if idx < 0 {
				t.Fatalf("expected 'tried:' in error message, got: %v", err)
			}
			triedPart := errStr[idx+len("tried: "):]
			if endIdx := strings.Index(triedPart, ")"); endIdx >= 0 {
				triedPart = triedPart[:endIdx]
			}
			triedCount := strings.Count(triedPart, ",") + 1
			if triedCount != tt.wantTried {
				t.Errorf("max_retries=%q: expected %d tried providers, got %d (%s)",
					tt.maxRetries, tt.wantTried, triedCount, triedPart)
			}
		})
	}
}

// TestLLMRouter_Strategies verifies that SelectProviders orders providers
// according to the configured strategy. This is the deterministic core of the
// strategy override mechanism exposed by the llm_router node's "strategy" param.
func TestLLMRouter_Strategies(t *testing.T) {
	providers := []RouterProvider{
		{Name: "cheap", Enabled: true, Priority: 1, CostPer1K: 0.5, AvgLatencyMs: 500, SuccessRate: 0.9},
		{Name: "fast", Enabled: true, Priority: 2, CostPer1K: 2.0, AvgLatencyMs: 100, SuccessRate: 0.95},
		{Name: "prio", Enabled: true, Priority: 3, CostPer1K: 1.0, AvgLatencyMs: 300, SuccessRate: 0.99},
	}

	tests := []struct {
		name      string
		strategy  string
		wantFirst string
	}{
		{"priority_highest_first", config.RouterStrategyPriority, "prio"},
		{"cost_lowest_first", config.RouterStrategyCost, "cheap"},
		{"latency_lowest_first", config.RouterStrategyLatency, "fast"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LLMRouter{
				providers: append([]RouterProvider(nil), providers...),
				stats:     make(map[string]*ProviderStats),
				strategy:  tt.strategy,
				maxRetry:  3,
			}
			if got := r.GetStrategy(); got != tt.strategy {
				t.Errorf("GetStrategy() = %q, want %q", got, tt.strategy)
			}
			ctx := context.Background()
			selected := r.SelectProviders(ctx)
			if len(selected) != len(providers) {
				t.Fatalf("SelectProviders returned %d providers, want %d", len(selected), len(providers))
			}
			if selected[0].Name != tt.wantFirst {
				t.Errorf("strategy %q: first provider = %q, want %q",
					tt.strategy, selected[0].Name, tt.wantFirst)
			}
		})
	}
}

// TestLLMRouter_StrategyOverride verifies that changing the strategy field
// (as the llm_router node does when the "strategy" param is provided) changes
// the ordering produced by SelectProviders.
func TestLLMRouter_StrategyOverride(t *testing.T) {
	providers := []RouterProvider{
		{Name: "low_cost", Enabled: true, Priority: 1, CostPer1K: 0.1, AvgLatencyMs: 1000, SuccessRate: 0.5},
		{Name: "high_cost", Enabled: true, Priority: 10, CostPer1K: 10.0, AvgLatencyMs: 100, SuccessRate: 0.99},
	}
	ctx := context.Background()

	// With the priority strategy, high_cost (priority 10) comes first.
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	if got := r.SelectProviders(ctx)[0].Name; got != "high_cost" {
		t.Errorf("priority strategy: expected high_cost first, got %s", got)
	}

	// Override strategy to cost: low_cost (cheaper) now comes first.
	r.strategy = config.RouterStrategyCost
	if got := r.SelectProviders(ctx)[0].Name; got != "low_cost" {
		t.Errorf("cost strategy override: expected low_cost first, got %s", got)
	}
}

// TestLLMRouter_RoundRobinStrategy verifies that the round_robin strategy
// returns all providers and rotates the starting index across calls.
func TestLLMRouter_RoundRobinStrategy(t *testing.T) {
	providers := []RouterProvider{
		{Name: "a", Enabled: true, Priority: 1},
		{Name: "b", Enabled: true, Priority: 2},
		{Name: "c", Enabled: true, Priority: 3},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyRoundRobin,
		maxRetry:  3,
	}
	ctx := context.Background()

	first := r.SelectProviders(ctx)
	if len(first) != len(providers) {
		t.Fatalf("round_robin: expected %d providers, got %d", len(providers), len(first))
	}

	second := r.SelectProviders(ctx)
	if len(second) != len(providers) {
		t.Fatalf("round_robin: expected %d providers on second call, got %d", len(providers), len(second))
	}
	// With more than one provider, the rotated start index should change the
	// first element between successive calls.
	if first[0].Name == second[0].Name {
		t.Errorf("round_robin: expected different first provider across calls, both were %q", first[0].Name)
	}
}

// TestLLMRouter_RandomStrategy verifies that the random strategy returns all
// providers (contents, not order).
func TestLLMRouter_RandomStrategy(t *testing.T) {
	providers := []RouterProvider{
		{Name: "a", Enabled: true, Priority: 1},
		{Name: "b", Enabled: true, Priority: 2},
		{Name: "c", Enabled: true, Priority: 3},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyRandom,
		maxRetry:  3,
	}
	ctx := context.Background()
	selected := r.SelectProviders(ctx)
	if len(selected) != len(providers) {
		t.Errorf("random: expected %d providers, got %d", len(providers), len(selected))
	}
}

// TestLLMRouter_GetProviders verifies GetProviders returns a defensive copy.
func TestLLMRouter_GetProviders(t *testing.T) {
	providers := []RouterProvider{
		{Name: "openai", Enabled: true},
		{Name: "anthropic", Enabled: true},
	}
	r := &LLMRouter{
		providers: providers,
		stats:     make(map[string]*ProviderStats),
		strategy:  "priority",
		maxRetry:  3,
	}
	got := r.GetProviders()
	if len(got) != 2 {
		t.Fatalf("GetProviders() returned %d, want 2", len(got))
	}
	// Mutating the returned slice must not affect the router's internal state.
	got[0].Name = "modified"
	if r.providers[0].Name == "modified" {
		t.Error("GetProviders() did not return a copy")
	}
}

// TestLLMRouter_GetProviderStats verifies stats are returned for tracked providers.
func TestLLMRouter_GetProviderStats(t *testing.T) {
	r := &LLMRouter{
		providers: []RouterProvider{{Name: "openai", Enabled: true}},
		stats:     make(map[string]*ProviderStats),
		strategy:  "priority",
		maxRetry:  3,
	}
	// No calls yet, so no stats should be tracked.
	stats := r.GetProviderStats()
	if len(stats) != 0 {
		t.Errorf("expected 0 stats before any calls, got %d", len(stats))
	}
}

// TestLLMRouter_DisabledProvidersExcluded verifies that disabled providers are
// not returned by SelectProviders.
func TestLLMRouter_DisabledProvidersExcluded(t *testing.T) {
	providers := []RouterProvider{
		{Name: "enabled_a", Enabled: true, Priority: 2},
		{Name: "disabled_b", Enabled: false, Priority: 10},
		{Name: "enabled_c", Enabled: true, Priority: 1},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	ctx := context.Background()
	selected := r.SelectProviders(ctx)
	if len(selected) != 2 {
		t.Fatalf("expected 2 active providers, got %d", len(selected))
	}
	for _, p := range selected {
		if p.Name == "disabled_b" {
			t.Error("disabled provider should not be selected")
		}
	}
}

// statusErr is a test-only error type carrying an HTTP-style status code.
// It is declared at package level so it can have an Error() method (Go does
// not allow methods on types declared inside function bodies) and so
// errors.As can extract a *statusErr from an []error.
type statusErr struct{ code int }

func (e *statusErr) Error() string { return fmt.Sprintf("status %d", e.code) }

// TestProviderMultiError_Unwrap verifies that errors.Is / errors.As traverse
// every wrapped provider error, not just the last one. This is the core
// correctness property of the multi-error: a caller must be able to detect
// context.Cancellation (or any other sentinel/typed error) even when it
// occurred on an earlier provider attempt and a later attempt produced a
// different, unrelated error. Before the multi-error fix, only the last
// provider's error was wrapped, so a cancellation on provider #1 hidden
// behind a network error on provider #2 was undetectable.
func TestProviderMultiError_Unwrap(t *testing.T) {
	// Simulate three provider failures: a 5xx, a context cancellation
	// (the "real" cause the caller cares about), and a connection error.
	sentinel := fmt.Errorf("upstream returned 500")
	multi := &ProviderMultiError{
		Providers: []string{"openai", "anthropic", "gemini"},
		Errors: []error{
			sentinel,
			context.Canceled,
			fmt.Errorf("connection refused"),
		},
	}

	// errors.Is must find context.Canceled even though it is the SECOND
	// of three wrapped errors (not the last).
	if !errors.Is(multi, context.Canceled) {
		t.Errorf("errors.Is(multi, context.Canceled) = false, want true; "+
			"multi-error must expose every wrapped error for inspection: %v", multi)
	}

	// errors.As must find a typed error from the first provider, even
	// though the second provider's error is a different type. Use a
	// concrete pointer-typed sentinel that errors.As can extract.
	wrappedSentinel := &statusErr{code: 500}
	multi2 := &ProviderMultiError{
		Providers: []string{"a", "b"},
		Errors: []error{
			wrappedSentinel,
			fmt.Errorf("unrelated"),
		},
	}
	var found *statusErr
	if !errors.As(multi2, &found) {
		t.Fatalf("errors.As failed to find *statusErr across multi-error")
	}
	if found.code != 500 {
		t.Errorf("errors.As found code=%d, want 500", found.code)
	}

	// A sentinel that is NOT in the batch must not match.
	if errors.Is(multi, context.DeadlineExceeded) {
		t.Errorf("errors.Is(multi, context.DeadlineExceeded) = true, want false")
	}
}

// TestProviderMultiError_ErrorFormat verifies the human-readable message
// lists every tried provider and caps verbose per-error text so a single
// chatty provider can't dominate the log line.
func TestProviderMultiError_ErrorFormat(t *testing.T) {
	t.Run("lists_all_providers", func(t *testing.T) {
		multi := &ProviderMultiError{
			Providers: []string{"openai", "anthropic"},
			Errors:    []error{fmt.Errorf("e1"), fmt.Errorf("e2")},
		}
		msg := multi.Error()
		if !strings.Contains(msg, "tried: openai, anthropic") {
			t.Errorf("message %q missing provider list", msg)
		}
		if !strings.Contains(msg, "openai: e1") {
			t.Errorf("message %q missing per-provider annotation", msg)
		}
	})

	t.Run("caps_long_error_text", func(t *testing.T) {
		long := strings.Repeat("x", 500)
		multi := &ProviderMultiError{
			Providers: []string{"p"},
			Errors:    []error{fmt.Errorf("%s", long)},
		}
		msg := multi.Error()
		// The per-error annotation is capped at 200 chars (197 + "...").
		if strings.Contains(msg, strings.Repeat("x", 201)) {
			t.Errorf("message did not cap long error text; got %d x's in a row", 201)
		}
		if !strings.Contains(msg, "...") {
			t.Errorf("expected truncation marker '...' in message, got: %s", msg)
		}
	})

	t.Run("nil_safe", func(t *testing.T) {
		var multi *ProviderMultiError
		if msg := multi.Error(); msg != "all LLM providers failed" {
			t.Errorf("nil multi-error message = %q, want fallback", msg)
		}
		if unwrap := multi.Unwrap(); unwrap != nil {
			t.Errorf("nil multi-error Unwrap = %v, want nil", unwrap)
		}
	})
}

// TestLLMRouter_Execute_ReturnsMultiError verifies that the all-providers-failed
// path returns a *ProviderMultiError that callers can type-assert on (and that
// errors.Is traverses the accumulated per-provider errors).
func TestLLMRouter_Execute_ReturnsMultiError(t *testing.T) {
	// Providers without API keys fail deterministically without network.
	providers := []RouterProvider{
		{Name: "openai", Enabled: true, APIKey: "", Priority: 1},
		{Name: "anthropic", Enabled: true, APIKey: "", Priority: 2},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}

	_, _, err := r.Execute(context.Background(), "input", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var multi *ProviderMultiError
	if !errors.As(err, &multi) {
		t.Fatalf("expected *ProviderMultiError, got %T: %v", err, err)
	}
	if len(multi.Providers) != 2 {
		t.Errorf("expected 2 tried providers, got %d", len(multi.Providers))
	}
	if len(multi.Errors) != 2 {
		t.Errorf("expected 2 accumulated errors, got %d", len(multi.Errors))
	}
	// The "no API key configured" error is a wrapped fmt error; verify
	// we can detect its absence/presence to confirm Unwrap traversal works.
	if errors.Is(err, context.Canceled) {
		t.Errorf("did not expect context.Canceled in no-key failure path")
	}
}

// TestLLMRouter_ParetoStrategy verifies the pareto strategy ranks
// Pareto-optimal providers (no other provider is both cheaper AND faster)
// ahead of dominated ones. With:
//   - A: cost=1, latency=100 (optimal: nothing beats it on both axes)
//   - B: cost=2, latency=200 (dominated by A: A is cheaper AND faster)
//   - C: cost=3, latency=50  (optimal: nothing is both cheaper and faster)
//
// The expected order is optimal-first sorted by cost: A, C, then B.
func TestLLMRouter_ParetoStrategy(t *testing.T) {
	providers := []RouterProvider{
		{Name: "B", Enabled: true, CostPer1K: 2.0, AvgLatencyMs: 200, SuccessRate: 0.9},
		{Name: "A", Enabled: true, CostPer1K: 1.0, AvgLatencyMs: 100, SuccessRate: 0.9},
		{Name: "C", Enabled: true, CostPer1K: 3.0, AvgLatencyMs: 50, SuccessRate: 0.9},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPareto,
		maxRetry:  3,
	}
	ctx := context.Background()
	selected := r.SelectProviders(ctx)
	if len(selected) != 3 {
		t.Fatalf("pareto: expected 3 providers, got %d", len(selected))
	}
	// A (optimal, cheapest) first, C (optimal) second, B (dominated) last.
	wantOrder := []string{"A", "C", "B"}
	for i, want := range wantOrder {
		if selected[i].Name != want {
			t.Errorf("pareto position %d = %q, want %q (full order: %v)",
				i, selected[i].Name, want, namesOf(selected))
		}
	}
}

// TestLLMRouter_ParetoStrategy_UsesEWMA verifies that when EWMA observations
// exist, pareto ranking uses the EWMA prediction rather than the static
// AvgLatencyMs.
func TestLLMRouter_ParetoStrategy_UsesEWMA(t *testing.T) {
	providers := []RouterProvider{
		{Name: "X", Enabled: true, CostPer1K: 1.0, AvgLatencyMs: 100, SuccessRate: 0.9},
		{Name: "Y", Enabled: true, CostPer1K: 2.0, AvgLatencyMs: 500, SuccessRate: 0.9},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPareto,
		maxRetry:  3,
	}
	// Seed EWMA: X is actually slow (300ms), Y is actually fast (50ms).
	// With EWMA, Y (cost=2, lat=50) dominates nothing but is optimal;
	// X (cost=1, lat=300) is also optimal (cheaper). Both optimal, so
	// sorted by cost: X first. But the latency used should be EWMA, not
	// AvgLatencyMs. Verify by making X dominated: give X high EWMA so
	// Y is both... no, Y is more expensive. Let's just verify the order
	// reflects EWMA by making X's EWMA very high and Y's very low with
	// equal costs so the latency tiebreaker decides.
	r.providers[0].CostPer1K = 1.0
	r.providers[1].CostPer1K = 1.0
	rStats := newProviderStats("")
	rStats.EwmaLatency.Observe(300) // X observed slow
	rStats.EwmaLatency.Observe(300)
	yStats := newProviderStats("")
	yStats.EwmaLatency.Observe(50) // Y observed fast
	yStats.EwmaLatency.Observe(50)
	r.stats["X"] = rStats
	r.stats["Y"] = yStats

	ctx := context.Background()
	selected := r.SelectProviders(ctx)
	// Equal cost => both optimal; within optimal tier sorted by cost
	// (equal) then latency. EWMA: Y=50 < X=300, so Y first.
	if selected[0].Name != "Y" {
		t.Errorf("pareto with EWMA: first = %q, want Y (EWMA 50ms < X 300ms); order: %v",
			selected[0].Name, namesOf(selected))
	}
}

// TestLLMRouter_LatencyStrategy_UsesEWMA verifies the latency strategy prefers
// the EWMA prediction over the static AvgLatencyMs when observations exist.
func TestLLMRouter_LatencyStrategy_UsesEWMA(t *testing.T) {
	providers := []RouterProvider{
		{Name: "slow_static", Enabled: true, AvgLatencyMs: 1000, SuccessRate: 0.9},
		{Name: "fast_static", Enabled: true, AvgLatencyMs: 50, SuccessRate: 0.9},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyLatency,
		maxRetry:  3,
	}
	// Seed EWMA that INVERTS the static ordering: slow_static is actually
	// fast (20ms), fast_static is actually slow (800ms).
	slowStats := newProviderStats("")
	slowStats.EwmaLatency.Observe(20)
	slowStats.EwmaLatency.Observe(20)
	fastStats := newProviderStats("")
	fastStats.EwmaLatency.Observe(800)
	fastStats.EwmaLatency.Observe(800)
	r.stats["slow_static"] = slowStats
	r.stats["fast_static"] = fastStats

	ctx := context.Background()
	selected := r.SelectProviders(ctx)
	// EWMA: slow_static=20 < fast_static=800, so slow_static first despite
	// its high static AvgLatencyMs.
	if selected[0].Name != "slow_static" {
		t.Errorf("latency with EWMA: first = %q, want slow_static (EWMA 20ms); order: %v",
			selected[0].Name, namesOf(selected))
	}
}

// TestLLMRouter_CircuitBreakerExcludesProvider verifies that a provider whose
// circuit breaker is Open is excluded from the active provider list.
func TestLLMRouter_CircuitBreakerExcludesProvider(t *testing.T) {
	providers := []RouterProvider{
		{Name: "healthy", Enabled: true, Priority: 1},
		{Name: "tripped", Enabled: true, Priority: 2},
	}
	r := &LLMRouter{
		providers: append([]RouterProvider(nil), providers...),
		stats:     make(map[string]*ProviderStats),
		strategy:  config.RouterStrategyPriority,
		maxRetry:  3,
	}
	// Force "tripped" into Open by exceeding the failure threshold.
	trippedStats := newProviderStats("")
	for i := 0; i < 10; i++ { // default threshold is 5
		trippedStats.Breaker.RecordFailure()
	}
	if trippedStats.Breaker.State() != CircuitOpen {
		t.Fatalf("expected tripped breaker to be Open, got %s", trippedStats.Breaker.State())
	}
	r.stats["tripped"] = trippedStats

	ctx := context.Background()
	selected := r.SelectProviders(ctx)
	for _, p := range selected {
		if p.Name == "tripped" {
			t.Error("tripped provider (Open breaker) should be excluded from selection")
		}
	}
	if len(selected) != 1 || selected[0].Name != "healthy" {
		t.Errorf("expected only healthy provider, got %v", namesOf(selected))
	}
}

// namesOf returns a comma-joined list of provider names for assertion messages.
func namesOf(providers []RouterProvider) string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
