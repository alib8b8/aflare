// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌​​‌​​​‌‌​‌​​‌‌​​‌​​​​​​‌‌​​​‌​‌​​​​‌‌‌​​‌‌‌​​‌​​​​​​​​​​​​​​​​​​​‌‌​‌‌​‌​‌‌​​​‌⁠
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
	"strings"
	"sync"

	"github.com/alib8b8/aflare/internal/logger"
)

// LLM cost attribution (inspired by PenguinHarness's "how much does an Agent
// cost" observability). The plumbing already carried a CostUSD field end-to-end
// (LLMCallTelemetry → LLMStepTrace → recordWorkflowMetrics), but nothing ever
// computed a value — it stayed 0. This file adds the missing piece: a model →
// price table and a compute function that fills CostUSD from token usage.
//
// Design choices:
//   - Centralised in the workflow package (applied in projectLLMTelemetry)
//     rather than per-LLM-node, so a single price table covers every provider
//     node and the OpenAICompatibleNode base without touching each one. A
//     router/caller that pre-computes CostUSD on LLMCallTelemetry still wins
//     (projectLLMTelemetry only fills when it is 0).
//   - Prices are USD per 1,000,000 tokens (the industry-standard unit), stored
//     as the per-1M rate to avoid float drift from "per 1K" rounding.
//   - Matching is case-insensitive longest-prefix: model strings often carry
//     date/region suffixes (e.g. "claude-3-haiku-20240307", "gpt-4o-2024-08-06")
//     that should not defeat pricing. Exact match is tried first; if it misses,
//     the longest key that is a prefix of the lowercased model wins.
//   - Unknown models yield 0.0 — we never fabricate a cost. Operators who need
//     coverage for an unlisted model set AFLARE_PRICING_FILE to a JSON file
//     mapping model → {input_per_1m, output_per_1m}; entries merge over the
//     defaults and also override default keys.
//   - The table is a snapshot read once (sync.Once) from env + defaults.
//     Pricing changes infrequently and a process restart picks them up; a
//     hot-reload would add locking cost for no operational benefit.

// ModelPricing is the USD price per 1,000,000 tokens for one model.
type ModelPricing struct {
	InputPer1M  float64 `json:"input_per_1m"`
	OutputPer1M float64 `json:"output_per_1m"`
}

// pricingEnvVar names the optional JSON file whose contents override/extend
// the built-in price table. Format: {"model-name": {"input_per_1m": 2.5,
// "output_per_1m": 10.0}, ...}. Model names are matched case-insensitively
// and by longest-prefix (see computeLLMCost), so a key like "gpt-4o" covers
// "gpt-4o-2024-08-06" without a separate entry.
const pricingEnvVar = "AFLARE_PRICING_FILE"

// defaultPricing is the built-in price table, sourced from each provider's
// published list pricing (USD per 1M tokens). Local/self-hosted models
// (ollama/*, qwen2-* on device) are priced 0 — their cost is amortised
// hardware, not per-call. Prices are indicative for cost awareness and
// budget alerts, NOT for billing: a provider's invoice is authoritative.
// When a provider's prices change, update here or override via
// AFLARE_PRICING_FILE without a code change.
var defaultPricing = map[string]ModelPricing{
	// OpenAI
	"gpt-4o":        {2.50, 10.00},
	"gpt-4o-mini":   {0.15, 0.60},
	"gpt-4-turbo":   {10.00, 30.00},
	"gpt-4":         {30.00, 60.00},
	"gpt-3.5-turbo": {0.50, 1.50},
	"o1":            {15.00, 60.00},
	"o1-mini":       {3.00, 12.00},
	"o1-preview":    {15.00, 60.00},
	"o3-mini":       {1.10, 4.40},
	// Anthropic (current generation, list prices as of 2026-09;
	// longest-prefix matching makes each key cover its dated variants,
	// e.g. "claude-opus-4-6" also prices "claude-opus-4-6-20260101")
	"claude-fable-5":    {10.00, 50.00}, // Fable 5.1 / Mythos 5(.1) share the price
	"claude-mythos-5":   {10.00, 50.00},
	"claude-opus-5":     {5.00, 25.00}, // Opus 4.5–4.8 share the price
	"claude-opus-4-8":   {5.00, 25.00},
	"claude-opus-4-7":   {5.00, 25.00},
	"claude-opus-4-6":   {5.00, 25.00},
	"claude-opus-4-5":   {5.00, 25.00},
	"claude-sonnet-5":   {2.00, 10.00}, // $2/$10 intro pricing made standard 2026-09-01
	"claude-sonnet-4-6": {3.00, 15.00},
	"claude-sonnet-4-5": {3.00, 15.00},
	"claude-haiku-4-5":  {1.00, 5.00},
	// Anthropic (retired generation — still served on Bedrock/Google
	// Cloud at these prices, kept for cost attribution on those routes)
	"claude-opus-4":     {15.00, 75.00}, // covers bare opus-4 and opus-4-1
	"claude-sonnet-4":   {3.00, 15.00},  // covers bare sonnet-4 and sonnet-4-1
	"claude-3-5-sonnet": {3.00, 15.00},
	"claude-3-5-haiku":  {0.80, 4.00},
	"claude-3-opus":     {15.00, 75.00},
	"claude-3-sonnet":   {3.00, 15.00},
	"claude-3-haiku":    {0.25, 1.25},
	// DeepSeek
	"deepseek-chat":     {0.27, 1.10}, // DeepSeek-V3
	"deepseek-reasoner": {0.55, 2.19}, // DeepSeek-R1
	// Zhipu GLM
	"glm-4":       {0.50, 0.50},
	"glm-4-flash": {0.00, 0.00}, // free tier
	"glm-4.5":     {0.55, 2.19},
	"glm-4-plus":  {0.70, 0.70},
	"glm-4-air":   {0.10, 0.10},
	// Moonshot / Kimi
	"moonshot-v1":    {1.20, 1.20},
	"kimi-k2":        {0.60, 2.50},
	"moonshot-v1-8k": {1.20, 1.20},
	// Alibaba Qwen
	"qwen-plus":  {0.40, 1.20},
	"qwen-turbo": {0.05, 0.20},
	"qwen-max":   {1.60, 6.40},
	// Google Gemini
	"gemini-1.5-pro":   {1.25, 5.00},
	"gemini-1.5-flash": {0.075, 0.30},
	"gemini-2.0-flash": {0.10, 0.40},
	// Mistral
	"mistral-large": {2.00, 6.00},
	"mistral-small": {0.20, 0.60},
	// xAI Grok
	"grok-4": {3.00, 15.00},
	// Perplexity
	"sonar":           {1.00, 1.00},
	"sonar-pro":       {3.00, 15.00},
	"sonar-reasoning": {2.00, 8.00},
	// Groq
	"llama-3.3-70b-versatile": {0.59, 0.79},
	// Together AI
	"meta-llama/llama-3.3-70b-instruct-turbo": {0.88, 0.88},
	// Local / self-hosted — cost is amortised hardware, not per-call.
	"ollama":  {0.00, 0.00},
	"llama":   {0.00, 0.00},
	"qwen2":   {0.00, 0.00},
	"qwen2.5": {0.00, 0.00},
}

var (
	// pricingOnce is a *sync.Once (not a value) so tests can atomically swap
	// in a fresh one to re-trigger resolution after flipping AFLARE_PRICING_FILE
	// via t.Setenv. In production the env is set once at startup and never
	// changes, so the once-only semantics hold; the pointer is solely a
	// testability hook, mirroring how sharedLLMCacheOnce could be reset.
	pricingOnce     = &sync.Once{}
	pricingResolved map[string]ModelPricing
)

// resolvePricing returns the effective price table: the built-in defaults
// merged with any entries from the AFLARE_PRICING_FILE override file. The
// result is computed once per process (sync.Once); env / file changes after
// the first call are not observed without a restart. This matches the
// existing pattern for AFLARE_LLM_CACHE (also sync.Once) and keeps the hot
// path (computeLLMCost, called per LLM call) lock-free.
func resolvePricing() map[string]ModelPricing {
	pricingOnce.Do(func() {
		// Copy defaults so an override merge never mutates the package var.
		pricingResolved = make(map[string]ModelPricing, len(defaultPricing))
		for k, v := range defaultPricing {
			pricingResolved[strings.ToLower(k)] = v
		}
		path := os.Getenv(pricingEnvVar)
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("failed to read LLM pricing override file, using defaults only",
				"file", path, "error", err)
			return
		}
		var overrides map[string]ModelPricing
		if err := json.Unmarshal(data, &overrides); err != nil {
			logger.Warn("failed to parse LLM pricing override file, using defaults only",
				"file", path, "error", err)
			return
		}
		for k, v := range overrides {
			pricingResolved[strings.ToLower(k)] = v
		}
		logger.Info("loaded LLM pricing overrides",
			"file", path, "override_count", len(overrides),
			"total_models", len(pricingResolved))
	})
	return pricingResolved
}

// pricingForModel looks up the price for model. It tries an exact
// case-insensitive match first, then falls back to the longest key that is a
// prefix of the lowercased model name. Returns the pricing and true on a hit;
// ModelPricing{} and false when no entry matches (the caller treats a miss as
// zero cost rather than guessing — fabricating a cost would corrupt budget
// alerts and any downstream billing reconciliation).
func pricingForModel(model string) (ModelPricing, bool) {
	if model == "" {
		return ModelPricing{}, false
	}
	table := resolvePricing()
	lower := strings.ToLower(model)
	// Exact match (common case: the model string is a known key).
	if p, ok := table[lower]; ok {
		return p, true
	}
	// Longest-prefix match: handles dated/region-suffixed variants like
	// "gpt-4o-2024-08-06" → "gpt-4o" and "claude-3-haiku-20240307" →
	// "claude-3-haiku". Iterate rather than sort because the table is small
	// (~40 entries) and this runs at most once per LLM call.
	var best ModelPricing
	bestLen := -1
	for k, p := range table {
		if strings.HasPrefix(lower, k) && len(k) > bestLen {
			best = p
			bestLen = len(k)
		}
	}
	if bestLen >= 0 {
		return best, true
	}
	return ModelPricing{}, false
}

// computeLLMCost returns the estimated USD cost of a single LLM call given its
// model and token usage. Returns 0.0 when the model is unknown (so totals
// never include fabricated cost) or when usage is nil/zero. The formula is
// the standard provider billing model: (prompt_tokens × input_rate +
// completion_tokens × output_rate) / 1_000_000, where rates are per-1M-token.
//
// This is a COST ESTIMATE for observability and budget alerts, not a billing
// figure: actual invoices depend on provider-specific rounding, cache discounts,
// tier pricing, and tax. For financial billing reconciliation, trust the
// provider invoice; use this figure for "did this run blow the budget?" alerts.
func computeLLMCost(model string, promptTokens, completionTokens int) float64 {
	p, ok := pricingForModel(model)
	if !ok {
		return 0
	}
	if promptTokens == 0 && completionTokens == 0 {
		return 0
	}
	return (float64(promptTokens)*p.InputPer1M + float64(completionTokens)*p.OutputPer1M) / 1_000_000
}

// aggregateLLMCosts sums the cost and token totals across every LLM call in a
// trace. Used by WorkflowTrace.finish to populate TotalCostUSD / TotalTokens
// so a single run's cost is queryable without re-iterating the step tree.
// Returns zeros when trace is nil or has no LLM calls.
func aggregateLLMCosts(trace *WorkflowTrace) (totalCost float64, totalTokens int) {
	if trace == nil {
		return 0, 0
	}
	for _, step := range trace.Steps {
		for _, call := range step.LLM {
			totalCost += call.CostUSD
			totalTokens += call.PromptTokens + call.CompletionTokens
		}
	}
	return totalCost, totalTokens
}
