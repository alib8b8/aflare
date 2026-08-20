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

package agent

import (
	"strings"
	"testing"
)

// TestContextBudget_ProviderDefaults verifies that the effective budget is
// selected per provider (P1-4): known providers get their window-derived
// default, unknown providers and ollama keep the legacy MaxContextChars.
func TestContextBudget_ProviderDefaults(t *testing.T) {
	cases := []struct {
		provider string
		want     int
	}{
		{"openai", 32000},
		{"deepseek", 32000},
		{"glm", 32000},
		{"qwen", 32000},
		{"kimi", 48000},
		{"mistral", 16000},
		{"yi", 16000},
		{"ollama", MaxContextChars},
		{"", MaxContextChars},               // provider never set
		{"custom-gateway", MaxContextChars}, // unknown provider falls back
	}
	for _, tc := range cases {
		cm := NewContextManager()
		if tc.provider != "" {
			cm.SetProvider(tc.provider)
		}
		if got := cm.Budget(); got != tc.want {
			t.Errorf("Budget() with provider %q = %d, want %d", tc.provider, got, tc.want)
		}
	}
}

// TestContextBudget_OverrideBeatsProviderDefault verifies that an explicit
// SetBudget wins over the provider default regardless of call order, and
// that a zero/negative SetBudget clears the override.
func TestContextBudget_OverrideBeatsProviderDefault(t *testing.T) {
	cm := NewContextManager()
	cm.SetBudget(5000)
	cm.SetProvider("openai") // provider default is 32000; override must win
	if got := cm.Budget(); got != 5000 {
		t.Errorf("Budget() = %d, want 5000 (override must beat provider default)", got)
	}

	// Reverse order: provider first, then override.
	cm2 := NewContextManager()
	cm2.SetProvider("kimi") // 48000
	cm2.SetBudget(2000)
	if got := cm2.Budget(); got != 2000 {
		t.Errorf("Budget() = %d, want 2000", got)
	}

	// Clearing the override restores the provider default.
	cm2.SetBudget(0)
	if got := cm2.Budget(); got != 48000 {
		t.Errorf("Budget() after clearing override = %d, want 48000 (kimi default)", got)
	}
}

// TestContextBudget_OverrideFloor verifies that degenerate overrides are
// clamped up to minContextBudget.
func TestContextBudget_OverrideFloor(t *testing.T) {
	cm := NewContextManager()
	cm.SetBudget(10)
	if got := cm.Budget(); got != minContextBudget {
		t.Errorf("Budget() = %d, want %d (floor clamp)", got, minContextBudget)
	}
}

// TestContextBudget_CompressionUsesProviderBudget verifies the behavioural
// effect: the same ~20000-token history is over the legacy budget (8000)
// but under the openai default (32000). With provider=openai no compression
// should occur; the same history with the budget overridden back to the
// legacy value must compress.
func TestContextBudget_CompressionUsesProviderBudget(t *testing.T) {
	fill := func(cm *ContextManager) {
		for i := 0; i < 20; i++ {
			// ~2000 latin chars ≈ 500 tokens/message (openai heuristic)
			// → 40 messages ≈ 20000 tokens total.
			cm.AddUser("question " + strings.Repeat("q", 2000))
			cm.AddAssistant("answer " + strings.Repeat("a", 2000))
		}
	}

	// openai budget (32000): 20000 tokens fit → no compression.
	cmOpenAI := NewContextManager()
	cmOpenAI.SetProvider("openai")
	fill(cmOpenAI)
	if before, after := cmOpenAI.CompressIfNeeded(); before != 0 || after != 0 {
		t.Errorf("openai: CompressIfNeeded() = (%d, %d), want (0, 0) — history fits the provider budget", before, after)
	}

	// Same history under the legacy budget (via explicit override) must
	// compress: ~30000 tokens > 8000.
	cmLegacy := NewContextManager()
	cmLegacy.SetProvider("openai")
	cmLegacy.SetBudget(MaxContextChars)
	fill(cmLegacy)
	before, after := cmLegacy.CompressIfNeeded()
	if before == 0 || after == 0 {
		t.Fatalf("legacy budget: CompressIfNeeded() = (%d, %d), want compression to occur", before, after)
	}
	if after >= before {
		t.Errorf("legacy budget: after=%d should be < before=%d", after, before)
	}
}

// TestContextBudget_ContextUsageLimitMatchesBudget verifies that the usage
// indicator and Summary report the effective budget, not the legacy
// constant.
func TestContextBudget_ContextUsageLimitMatchesBudget(t *testing.T) {
	cm := NewContextManager()
	cm.SetProvider("kimi")
	_, limit, _ := cm.ContextUsage()
	if limit != 48000 {
		t.Errorf("ContextUsage() limit = %d, want 48000", limit)
	}

	cm.SetBudget(2000)
	_, limit, _ = cm.ContextUsage()
	if limit != 2000 {
		t.Errorf("ContextUsage() limit after override = %d, want 2000", limit)
	}

	summary := cm.Summary()
	if !strings.Contains(summary, "limit: 2000") {
		t.Errorf("Summary() = %q, want it to mention \"limit: 2000\"", summary)
	}
}

// TestContextBudget_AgentLoopPlumbsOverride verifies that NewAgentLoop
// applies Config.ContextBudget to the loop's context manager.
func TestContextBudget_AgentLoopPlumbsOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Provider = "ollama"
	cfg.ContextBudget = 12000
	loop := NewAgentLoop(cfg)
	if got := loop.Context().Budget(); got != 12000 {
		t.Errorf("loop context budget = %d, want 12000 (Config.ContextBudget)", got)
	}

	// No override: the loop uses the provider default.
	cfg2 := DefaultConfig()
	cfg2.Provider = "deepseek"
	loop2 := NewAgentLoop(cfg2)
	if got := loop2.Context().Budget(); got != 32000 {
		t.Errorf("loop2 context budget = %d, want 32000 (deepseek default)", got)
	}
}
