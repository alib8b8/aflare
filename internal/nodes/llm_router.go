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

package nodes

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alib8b8/llm-box/internal/config"
	"github.com/alib8b8/llm-box/internal/logger"
)

type RouterProvider struct {
	Name         string
	Model        string
	Endpoint     string
	APIKey       string
	Priority     int
	CostPer1K    float64
	QuotaDaily   int64
	Enabled      bool
	AvgLatencyMs int64
	SuccessRate  float64
}

type ProviderStats struct {
	TotalCalls   int64
	SuccessCalls int64
	FailedCalls  int64
	// ConsecutiveFailures is reset to 0 on any successful call. It
	// drives the cooldown backoff so a healthy provider with occasional
	// failures isn't punished for its lifetime cumulative failure count.
	ConsecutiveFailures int64
	TotalLatency        int64
	TokensUsed          int64
	DailyUsage          int64
	LastUsed            time.Time
	LastResetDate       string
	CooldownUntil       time.Time
}

type LLMRouter struct {
	providers []RouterProvider
	stats     map[string]*ProviderStats
	statsMu   sync.RWMutex
	rrCounter uint64
	strategy  string
	maxRetry  int
}

var (
	globalRouter *LLMRouter
	routerOnce   sync.Once
	routerMu     sync.Mutex
)

func GetGlobalRouter() *LLMRouter {
	routerOnce.Do(func() {
		globalRouter = NewLLMRouterFromConfig()
	})
	return globalRouter
}

func NewLLMRouterFromConfig() *LLMRouter {
	rcfg := config.GetRouterConfig()

	providers := buildProviderList(rcfg)

	strategy := rcfg.Strategy
	if strategy == "" {
		strategy = config.RouterStrategyPriority
	}

	maxRetry := rcfg.MaxRetries
	if maxRetry <= 0 {
		maxRetry = 3
	}

	return &LLMRouter{
		providers: providers,
		stats:     make(map[string]*ProviderStats),
		strategy:  strategy,
		maxRetry:  maxRetry,
	}
}

func buildProviderList(rcfg config.RouterConfig) []RouterProvider {
	if len(rcfg.FallbackOrder) > 0 {
		var result []RouterProvider
		for i, entry := range rcfg.FallbackOrder {
			if entry.Enabled == false {
				continue
			}
			priority := entry.Priority
			if priority == 0 {
				priority = len(rcfg.FallbackOrder) - i
			}

			providerName := strings.ToLower(entry.Name)
			pcfg := config.GetProviderConfig(providerName)

			model := entry.Model
			if model == "" {
				model = pcfg.Model
			}
			if model == "" {
				model = defaultModelFor(providerName)
			}

			endpoint := pcfg.Endpoint
			if endpoint == "" {
				endpoint = defaultEndpointFor(providerName)
			}

			apiKey := pcfg.APIKey
			if apiKey == "" {
				apiKey = config.GetAPIKey(providerName, strings.ToUpper(providerName)+"_API_KEY")
			}

			result = append(result, RouterProvider{
				Name:       providerName,
				Model:      model,
				Endpoint:   endpoint,
				APIKey:     apiKey,
				Priority:   priority,
				CostPer1K:  entry.CostPer1K,
				QuotaDaily: entry.Quota,
				Enabled:    entry.Enabled != false,
			})
		}
		if len(result) > 0 {
			return result
		}
	}

	return detectAvailableProviders()
}

func detectAvailableProviders() []RouterProvider {
	candidates := []string{
		"openai", "anthropic", "gemini", "deepseek", "qwen",
		"kimi", "glm", "yi", "mistral", "ollama",
	}

	var result []RouterProvider
	priority := len(candidates)

	for _, name := range candidates {
		apiKey := config.GetAPIKey(name, strings.ToUpper(name)+"_API_KEY")
		if name == "ollama" || apiKey != "" {
			pcfg := config.GetProviderConfig(name)
			model := pcfg.Model
			if model == "" {
				model = defaultModelFor(name)
			}
			endpoint := pcfg.Endpoint
			if endpoint == "" {
				endpoint = defaultEndpointFor(name)
			}

			result = append(result, RouterProvider{
				Name:        name,
				Model:       model,
				Endpoint:    endpoint,
				APIKey:      apiKey,
				Priority:    priority,
				Enabled:     true,
				SuccessRate: 1.0,
			})
		}
		priority--
	}

	return result
}

func defaultModelFor(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o-mini"
	case "anthropic":
		return "claude-3-haiku-20240307"
	case "gemini":
		return "gemini-1.5-flash"
	case "deepseek":
		return "deepseek-chat"
	case "qwen":
		return "qwen-plus"
	case "kimi":
		return "moonshot-v1-8k"
	case "glm":
		return "glm-4-flash"
	case "yi":
		return "yi-lightning"
	case "mistral":
		return "mistral-small-latest"
	case "ollama":
		return "llama3"
	default:
		return "gpt-4o-mini"
	}
}

func (r *LLMRouter) SelectProviders(ctx context.Context) []RouterProvider {
	providers := r.getActiveProviders()

	if len(providers) == 0 {
		return providers
	}

	switch r.strategy {
	case config.RouterStrategyCost:
		return r.sortByCost(providers)
	case config.RouterStrategyLatency:
		return r.sortByLatency(providers)
	case config.RouterStrategyRoundRobin:
		return r.roundRobin(providers)
	case config.RouterStrategyRandom:
		return r.randomOrder(providers)
	case config.RouterStrategyPriority:
		fallthrough
	default:
		return r.sortByPriority(providers)
	}
}

func (r *LLMRouter) getActiveProviders() []RouterProvider {
	now := time.Now()
	today := now.Format("2006-01-02")

	r.statsMu.Lock()
	defer r.statsMu.Unlock()

	var active []RouterProvider
	for _, p := range r.providers {
		if !p.Enabled {
			continue
		}

		stats := r.stats[p.Name]
		if stats == nil {
			stats = &ProviderStats{LastResetDate: today}
			r.stats[p.Name] = stats
		}

		if stats.LastResetDate != today {
			stats.DailyUsage = 0
			stats.LastResetDate = today
		}

		if p.QuotaDaily > 0 && stats.DailyUsage >= p.QuotaDaily {
			continue
		}

		if now.Before(stats.CooldownUntil) {
			continue
		}

		active = append(active, p)
	}

	return active
}

func (r *LLMRouter) sortByPriority(providers []RouterProvider) []RouterProvider {
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Priority != providers[j].Priority {
			return providers[i].Priority > providers[j].Priority
		}
		return providers[i].SuccessRate > providers[j].SuccessRate
	})
	return providers
}

func (r *LLMRouter) sortByCost(providers []RouterProvider) []RouterProvider {
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].CostPer1K != providers[j].CostPer1K {
			return providers[i].CostPer1K < providers[j].CostPer1K
		}
		return providers[i].SuccessRate > providers[j].SuccessRate
	})
	return providers
}

func (r *LLMRouter) sortByLatency(providers []RouterProvider) []RouterProvider {
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].AvgLatencyMs != providers[j].AvgLatencyMs {
			return providers[i].AvgLatencyMs < providers[j].AvgLatencyMs
		}
		return providers[i].SuccessRate > providers[j].SuccessRate
	})
	return providers
}

func (r *LLMRouter) roundRobin(providers []RouterProvider) []RouterProvider {
	n := uint64(len(providers))
	if n == 0 {
		return providers
	}
	start := atomic.AddUint64(&r.rrCounter, 1) % n
	result := make([]RouterProvider, n)
	for i := uint64(0); i < n; i++ {
		result[i] = providers[(start+i)%n]
	}
	return result
}

func (r *LLMRouter) randomOrder(providers []RouterProvider) []RouterProvider {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]RouterProvider, len(providers))
	copy(result, providers)
	rng.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})
	return result
}

func (r *LLMRouter) Execute(ctx context.Context, input string, params map[string]string) (string, string, error) {
	providers := r.SelectProviders(ctx)
	if len(providers) == 0 {
		// B-3: publish the (empty) decision so the trace records that the
		// router was invoked but had no candidates.
		RouterDecisionSinkFrom(ctx).RecordRouterDecision(RouterDecision{
			Strategy:   r.strategy,
			Candidates: nil,
			Selected:   "",
			FinalError: "no active LLM providers available",
		})
		return "", "", fmt.Errorf("no active LLM providers available. Configure at least one provider via config or environment variables")
	}

	systemPrompt := getParam(params, "system", "")
	maxRetries := r.maxRetry
	if override := getParam(params, "max_retries", ""); override != "" {
		if n, err := fmt.Sscanf(override, "%d", &maxRetries); err == nil && n == 1 && maxRetries > 0 {
		} else {
			maxRetries = r.maxRetry
		}
	}

	// B-3: build the candidate list and accumulate attempts for the
	// decision record published at the end (success or all-failed).
	candidateNames := make([]string, 0, len(providers))
	for _, p := range providers {
		candidateNames = append(candidateNames, p.Name)
	}
	var attempts []RouterAttempt
	var lastErr error
	var triedProviders []string

	for attempt := 0; attempt < maxRetries && attempt < len(providers); attempt++ {
		provider := providers[attempt]
		triedProviders = append(triedProviders, provider.Name)

		if provider.APIKey == "" && provider.Name != "ollama" {
			lastErr = fmt.Errorf("provider %s has no API key configured", provider.Name)
			r.recordFailure(provider.Name, 0)
			attempts = append(attempts, RouterAttempt{
				Provider: provider.Name,
				Success:  false,
				Error:    lastErr.Error(),
			})
			continue
		}

		start := time.Now()
		result, err := r.callProvider(ctx, provider, input, systemPrompt, params)
		latency := time.Since(start).Milliseconds()

		if err == nil {
			tokensUsed := int64((len(input) + len(result)) / 4)
			r.recordSuccess(provider.Name, latency, tokensUsed)
			attempts = append(attempts, RouterAttempt{
				Provider:  provider.Name,
				Success:   true,
				LatencyMs: latency,
			})
			RouterDecisionSinkFrom(ctx).RecordRouterDecision(RouterDecision{
				Strategy:   r.strategy,
				Candidates: candidateNames,
				Selected:   provider.Name,
				Attempts:   attempts,
			})
			return result, provider.Name, nil
		}

		lastErr = err
		r.recordFailure(provider.Name, latency)
		attempts = append(attempts, RouterAttempt{
			Provider:  provider.Name,
			Success:   false,
			Error:     err.Error(),
			LatencyMs: latency,
		})
		logger.Warn("LLM provider failed, trying next",
			"provider", provider.Name,
			"error", err,
			"attempt", attempt+1,
			"total", len(providers),
		)
	}

	finalErr := fmt.Errorf("all LLM providers failed (tried: %s). Last error: %w",
		strings.Join(triedProviders, ", "), lastErr)
	RouterDecisionSinkFrom(ctx).RecordRouterDecision(RouterDecision{
		Strategy:   r.strategy,
		Candidates: candidateNames,
		Selected:   "",
		Attempts:   attempts,
		FinalError: finalErr.Error(),
	})
	return "", "", finalErr
}

func (r *LLMRouter) callProvider(ctx context.Context, p RouterProvider, input, systemPrompt string, params map[string]string) (string, error) {
	compatNode := NewOpenAICompatibleNode(LLMNodeConfig{
		Name:            p.Name,
		DefaultModel:    p.Model,
		DefaultEndpoint: p.Endpoint,
		EnvAPIKey:       strings.ToUpper(p.Name) + "_API_KEY",
		ProviderName:    p.Name,
	})

	callParams := make(map[string]string)
	for k, v := range params {
		callParams[k] = v
	}
	callParams["model"] = p.Model
	callParams["endpoint"] = p.Endpoint
	if p.APIKey != "" {
		callParams["api_key"] = p.APIKey
	}
	if systemPrompt != "" {
		callParams["system"] = systemPrompt
	}

	return compatNode.Execute(ctx, input, callParams)
}

func (r *LLMRouter) recordSuccess(name string, latencyMs int64, tokensUsed int64) {
	r.statsMu.Lock()
	defer r.statsMu.Unlock()

	stats := r.getOrCreateStatsLocked(name)

	today := time.Now().Format("2006-01-02")
	if stats.LastResetDate != today {
		stats.DailyUsage = 0
		stats.LastResetDate = today
	}

	stats.TotalCalls++
	stats.SuccessCalls++
	stats.TotalLatency += latencyMs
	stats.TokensUsed += tokensUsed
	stats.DailyUsage += tokensUsed
	stats.LastUsed = time.Now()
	// A successful call breaks the failure streak: reset the consecutive
	// counter so cooldowns reflect the CURRENT health of the provider.
	stats.ConsecutiveFailures = 0

	if stats.TotalCalls > 0 {
		for i, p := range r.providers {
			if p.Name == name {
				r.providers[i].AvgLatencyMs = stats.TotalLatency / stats.TotalCalls
				r.providers[i].SuccessRate = float64(stats.SuccessCalls) / float64(stats.TotalCalls)
				break
			}
		}
	}
}

func (r *LLMRouter) recordFailure(name string, latencyMs int64) {
	r.statsMu.Lock()
	defer r.statsMu.Unlock()

	stats := r.getOrCreateStatsLocked(name)
	stats.TotalCalls++
	stats.FailedCalls++
	stats.ConsecutiveFailures++
	if latencyMs > 0 {
		stats.TotalLatency += latencyMs
	}
	stats.LastUsed = time.Now()

	// Cooldown backoff is driven by the CONSECUTIVE failure count, not
	// the cumulative one. A provider that fails 5 times in a row then
	// recovers should not be cooled down forever after.
	if stats.ConsecutiveFailures >= 5 {
		cooldownSeconds := 30 * int(stats.ConsecutiveFailures)
		if cooldownSeconds > 300 {
			cooldownSeconds = 300
		}
		stats.CooldownUntil = time.Now().Add(time.Duration(cooldownSeconds) * time.Second)
		logger.Warn("Provider cooldown activated",
			"provider", name,
			"consecutive_failures", stats.ConsecutiveFailures,
			"cooldown_seconds", cooldownSeconds,
		)
	}

	if stats.TotalCalls > 0 {
		for i, p := range r.providers {
			if p.Name == name {
				if latencyMs > 0 {
					r.providers[i].AvgLatencyMs = stats.TotalLatency / stats.TotalCalls
				}
				r.providers[i].SuccessRate = float64(stats.SuccessCalls) / float64(stats.TotalCalls)
				break
			}
		}
	}
}

func (r *LLMRouter) getOrCreateStatsLocked(name string) *ProviderStats {
	stats, ok := r.stats[name]
	if !ok {
		today := time.Now().Format("2006-01-02")
		stats = &ProviderStats{
			LastResetDate: today,
		}
		r.stats[name] = stats
	}
	return stats
}

func (r *LLMRouter) GetProviderStats() map[string]ProviderStats {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()

	result := make(map[string]ProviderStats, len(r.stats))
	for k, v := range r.stats {
		result[k] = *v
	}
	return result
}

func (r *LLMRouter) GetProviders() []RouterProvider {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()
	// Copy under the read lock so concurrent recordSuccess/recordFailure
	// writers can't tear a struct field mid-copy.
	return append([]RouterProvider(nil), r.providers...)
}

func (r *LLMRouter) GetStrategy() string {
	return r.strategy
}

func ResetGlobalRouter() {
	routerMu.Lock()
	defer routerMu.Unlock()
	globalRouter = nil
	routerOnce = sync.Once{}
}
