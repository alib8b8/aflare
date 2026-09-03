// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌​​​​​​​​‌‌‌​​‌​‌‌‌​‌‌​‌‌‌‌​​​‌‌​​‌‌​​​​‌‌‌​‌​‌​​​​​​​​​​​​​​​​​​‌‌‌‌​‌‌​‌​​‌‌‌‌​⁠
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

package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestComputeLLMCost_KnownModel verifies the core cost formula against a
// hand-computed expected value for a model in the default price table. This is
// the regression anchor: if the formula or the gpt-4o price changes, this test
// fails loudly. gpt-4o is 2.50/1M input + 10.00/1M output, so 1000 prompt +
// 500 completion tokens = (1000*2.5 + 500*10) / 1e6 = 0.0075 USD.
func TestComputeLLMCost_KnownModel(t *testing.T) {
	got := computeLLMCost("gpt-4o", 1000, 500)
	want := (1000*2.5 + 500*10.0) / 1_000_000
	if !almostEqual(got, want, 1e-12) {
		t.Errorf("computeLLMCost(gpt-4o, 1000, 500) = %.10f, want %.10f", got, want)
	}
}

// TestComputeLLMCost_PrefixMatch verifies that dated/suffixed model variants
// resolve via longest-prefix matching. "gpt-4o-2024-08-06" must price the same
// as "gpt-4o", and "claude-3-haiku-20240307" the same as "claude-3-haiku".
// Without prefix matching, every dated variant would silently report $0 cost,
// defeating the whole point of cost attribution.
func TestComputeLLMCost_PrefixMatch(t *testing.T) {
	cases := []struct {
		model    string
		baseCost float64 // expected == computeLLMCost(baseModel, 1000, 0)
		base     string
	}{
		{"gpt-4o-2024-08-06", computeLLMCost("gpt-4o", 1000, 0), "gpt-4o"},
		{"GPT-4O-MINI-2024-07-18", computeLLMCost("gpt-4o-mini", 1000, 0), "gpt-4o-mini"},
		{"claude-3-haiku-20240307", computeLLMCost("claude-3-haiku", 1000, 0), "claude-3-haiku"},
		{"deepseek-chat-1234", computeLLMCost("deepseek-chat", 1000, 0), "deepseek-chat"},
		// Current Claude generation: dated variants must price against
		// their exact base, not a shorter (wrong) prefix — the table has
		// both "claude-sonnet-4-6" ($3/$15) and "claude-sonnet-5"
		// ($2/$10), so a suffix on either must not cross-match.
		{"claude-sonnet-5-20260630", computeLLMCost("claude-sonnet-5", 1000, 0), "claude-sonnet-5"},
		{"claude-opus-4-6-20260101", computeLLMCost("claude-opus-4-6", 1000, 0), "claude-opus-4-6"},
		{"claude-haiku-4-5-20251001", computeLLMCost("claude-haiku-4-5", 1000, 0), "claude-haiku-4-5"},
	}
	for _, c := range cases {
		got := computeLLMCost(c.model, 1000, 0)
		if !almostEqual(got, c.baseCost, 1e-12) {
			t.Errorf("prefix match failed: computeLLMCost(%q) = %.10f, want %.10f (same as %q)",
				c.model, got, c.baseCost, c.base)
		}
	}
}

// TestComputeLLMCost_ClaudeCurrentGeneration pins the official list prices of
// the current Claude generation (checked against Anthropic's pricing page,
// 2026-09). Claude cost attribution was silently $0 for every model shipped
// after the claude-3 family retired — these anchors keep the table honest and
// catch accidental cross-generation prefix drift (e.g. sonnet-5 accidentally
// priced at the sonnet-4-6 rate).
func TestComputeLLMCost_ClaudeCurrentGeneration(t *testing.T) {
	cases := []struct {
		model       string
		inputPer1M  float64
		outputPer1M float64
	}{
		{"claude-sonnet-5", 2.00, 10.00},
		{"claude-opus-5", 5.00, 25.00},
		{"claude-haiku-4-5", 1.00, 5.00},
		{"claude-fable-5", 10.00, 50.00},
		{"claude-fable-5-1", 10.00, 50.00}, // prefix: fable-5 covers 5.1
		{"claude-sonnet-4-5", 3.00, 15.00},
		{"claude-3-5-sonnet-latest", 3.00, 15.00}, // retired gen, Bedrock route
	}
	for _, c := range cases {
		got := computeLLMCost(c.model, 1_000_000, 1_000_000)
		want := c.inputPer1M + c.outputPer1M
		if !almostEqual(got, want, 1e-9) {
			t.Errorf("computeLLMCost(%q, 1M in + 1M out) = %.4f, want %.4f ($%.2f/$%.2f per MTok)",
				c.model, got, want, c.inputPer1M, c.outputPer1M)
		}
	}
}

// TestComputeLLMCost_UnknownModelIsZero asserts that an unlisted model yields
// exactly 0.0 — never a fabricated estimate. Fabricating cost would corrupt
// budget alerts and any downstream billing reconciliation, so the safe default
// is "no data" rather than a guess.
func TestComputeLLMCost_UnknownModelIsZero(t *testing.T) {
	got := computeLLMCost("some-future-model-2099", 1000, 1000)
	if got != 0 {
		t.Errorf("computeLLMCost(unknown model) = %.10f, want 0 (never fabricate cost)", got)
	}
}

// TestComputeLLMCost_ZeroUsageIsZero confirms that a known model with zero
// token usage reports $0 (no division-by-zero, no negative cost).
func TestComputeLLMCost_ZeroUsageIsZero(t *testing.T) {
	if got := computeLLMCost("gpt-4o", 0, 0); got != 0 {
		t.Errorf("computeLLMCost(gpt-4o, 0, 0) = %.10f, want 0", got)
	}
}

// TestComputeLLMCost_EmptyModelIsZero guards the empty-string path.
func TestComputeLLMCost_EmptyModelIsZero(t *testing.T) {
	if got := computeLLMCost("", 1000, 1000); got != 0 {
		t.Errorf("computeLLMCost(\"\", ...) = %.10f, want 0", got)
	}
}

// TestComputeLLMCost_OnlyCompletionTokens verifies output-only pricing works
// (e.g. a cached-prompt call that still generates completion tokens).
func TestComputeLLMCost_OnlyCompletionTokens(t *testing.T) {
	got := computeLLMCost("gpt-4o", 0, 1000)
	want := 1000 * 10.0 / 1_000_000
	if !almostEqual(got, want, 1e-12) {
		t.Errorf("computeLLMCost(gpt-4o, 0, 1000) = %.10f, want %.10f", got, want)
	}
}

// TestPricingOverride_File loads a JSON override via AFLARE_PRICING_FILE and
// confirms it both ADDS a new model and OVERRIDES an existing default. This is
// the operator escape hatch for unlisted/renegotiated prices without a code
// change. Uses t.Setenv so the process-wide sync.Once snapshot is taken with
// the override active; because pricingOnce is package-global and already
// triggered by earlier tests in this package, this test relies on the override
// having been loaded at first-resolve time. To keep it deterministic we run it
// FIRST (TestMain ordering) — see the build-tag note below. In practice the
// override file is set at process start before any workflow runs, matching
// this test's setup.
func TestPricingOverride_File(t *testing.T) {
	// Build an override file: add a brand-new model and override gpt-4o's
	// price to something unmistakably different.
	overrides := map[string]ModelPricing{
		"custom-internal-model": {1.00, 5.00},
		"gpt-4o":                {99.0, 99.0}, // override default
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "pricing.json")
	b, err := json.Marshal(overrides)
	if err != nil {
		t.Fatalf("marshal overrides: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write override file: %v", err)
	}

	t.Setenv(pricingEnvVar, path)
	// Force a fresh resolve by swapping in a new sync.Once and clearing the
	// cached map. pricingOnce is a *sync.Once precisely so tests can do this;
	// production never flips the env after start, so the once-only semantics
	// hold there. Tests in this package run sequentially (no t.Parallel), so
	// the swap is race-free.
	pricingOnce = &sync.Once{}
	pricingResolved = nil
	// t.Setenv restores AFLARE_PRICING_FILE to "" on exit; reset the cache
	// again so any later test re-resolves against the (now-empty) env and
	// picks up defaults, rather than inheriting this test's override.
	t.Cleanup(func() {
		pricingOnce = &sync.Once{}
		pricingResolved = nil
	})

	// custom-internal-model should now be priced.
	got := computeLLMCost("custom-internal-model", 1_000_000, 0)
	if !almostEqual(got, 1.0, 1e-9) {
		t.Errorf("override should price custom-internal-model input at $1.0/1M; got %.6f", got)
	}
	// gpt-4o should reflect the override ($99/1M), NOT the default ($2.5/1M).
	got = computeLLMCost("gpt-4o", 1_000_000, 0)
	if !almostEqual(got, 99.0, 1e-9) {
		t.Errorf("override should change gpt-4o input to $99/1M; got %.6f (default is 2.5)", got)
	}
}

// TestAggregateLLMCosts_SumsAcrossSteps verifies the per-workflow aggregation
// that feeds WorkflowTrace.TotalCostUSD. A trace with two steps, each with one
// LLM call, must sum both calls' cost and tokens.
func TestAggregateLLMCosts_SumsAcrossSteps(t *testing.T) {
	trace := &WorkflowTrace{
		Steps: []StepTrace{
			{LLM: []LLMStepTrace{
				{CostUSD: 0.001, PromptTokens: 100, CompletionTokens: 50},
			}},
			{LLM: []LLMStepTrace{
				{CostUSD: 0.002, PromptTokens: 200, CompletionTokens: 100},
				{CostUSD: 0.003, PromptTokens: 300, CompletionTokens: 150},
			}},
		},
	}
	cost, tokens := aggregateLLMCosts(trace)
	if !almostEqual(cost, 0.006, 1e-12) {
		t.Errorf("total cost = %.6f, want 0.006", cost)
	}
	// 100+50 + 200+100 + 300+150 = 900
	if tokens != 900 {
		t.Errorf("total tokens = %d, want 900", tokens)
	}
}

// TestAggregateLLMCosts_NilTraceIsZero guards the nil-trace path (e.g. a
// panic-recovery audit record that never built a trace).
func TestAggregateLLMCosts_NilTraceIsZero(t *testing.T) {
	cost, tokens := aggregateLLMCosts(nil)
	if cost != 0 || tokens != 0 {
		t.Errorf("aggregateLLMCosts(nil) = (%v, %d), want (0, 0)", cost, tokens)
	}
}

// TestWorkflowTrace_Finish_PopulatesTotals confirms that finish() stamps the
// aggregated cost/token totals onto the trace, so callers reading
// trace.TotalCostUSD after execution get the right value without manually
// re-iterating steps.
func TestWorkflowTrace_Finish_PopulatesTotals(t *testing.T) {
	trace := &WorkflowTrace{
		StartedAt: time.Now(),
		Steps: []StepTrace{
			{LLM: []LLMStepTrace{
				{CostUSD: 0.0042, PromptTokens: 500, CompletionTokens: 250},
			}},
		},
	}
	trace.finish(trace.StartedAt.Add(1_000_000_000)) // +1s
	if !almostEqual(trace.TotalCostUSD, 0.0042, 1e-12) {
		t.Errorf("trace.TotalCostUSD = %.6f, want 0.0042", trace.TotalCostUSD)
	}
	if trace.TotalTokens != 750 {
		t.Errorf("trace.TotalTokens = %d, want 750", trace.TotalTokens)
	}
	if trace.Duration <= 0 {
		t.Errorf("trace.Duration = %v, want > 0", trace.Duration)
	}
}

// TestPricingForModel_CaseInsensitive confirms matching is case-insensitive
// (providers send mixed-case model strings; e.g. "GPT-4O" must price the same
// as "gpt-4o").
func TestPricingForModel_CaseInsensitive(t *testing.T) {
	lower, _ := pricingForModel("gpt-4o")
	upper, ok := pricingForModel("GPT-4O")
	if !ok {
		t.Fatal("GPT-4O should match case-insensitively")
	}
	if lower != upper {
		t.Errorf("case mismatch: gpt-4o=%+v, GPT-4O=%+v", lower, upper)
	}
}

// almostEqual reports whether a and b are within eps — used because floating
// point cost arithmetic can introduce tiny representation errors that a strict
// == comparison would flag.
func almostEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
