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
	"strconv"
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
	// Retained for backward compatibility / stats display; routing now
	// relies on the circuit breaker (Breaker) instead.
	ConsecutiveFailures int64
	TotalLatency        int64
	TokensUsed          int64
	DailyUsage          int64
	LastUsed            time.Time
	LastResetDate       string
	CooldownUntil       time.Time
	// EwmaLatency is the EWMA latency predictor used for routing decisions.
	// It replaces the arithmetic AvgLatencyMs mean which never decays old
	// observations. Nil-safe: callers must guard with a nil check.
	EwmaLatency *EWMAPredictor
	// Breaker is the per-provider circuit breaker (CLOSED/OPEN/HALF_OPEN).
	// Replaces the old ConsecutiveFailures>=5 cooldown logic. Nil-safe.
	Breaker *CircuitBreaker
}

// ProviderMultiError is returned by LLMRouter.Execute when every candidate
// provider failed. It wraps every per-provider error so callers can use
// errors.Is / errors.As to detect a specific failure mode across the whole
// batch — most importantly, whether context cancellation was the real cause
// (a caller whose deadline expired should not be told "all providers
// failed" when the truthful diagnosis is "you cancelled").
//
// Unwrap() []error is the multi-unwrap protocol introduced in Go 1.20:
// errors.Is and errors.As traverse every error in the returned slice, so
// e.g. errors.Is(multiErr, context.Canceled) is true if ANY provider
// attempt returned context.Canceled. This matters because the router
// tries providers sequentially; without multi-unwrap, only the LAST
// provider's error would be inspectable, and a cancellation that
// happened on an earlier provider (before a later provider produced a
// different error) would be silently hidden.
type ProviderMultiError struct {
	// Providers is the ordered list of provider names that were tried,
	// matching the order of Errors.
	Providers []string
	// Errors is the per-provider error, parallel to Providers.
	Errors []error
}

func (e *ProviderMultiError) Error() string {
	if e == nil {
		return "all LLM providers failed"
	}
	var b strings.Builder
	b.WriteString("all LLM providers failed (tried: ")
	b.WriteString(strings.Join(e.Providers, ", "))
	b.WriteString(")")
	// Annotate with each provider's error so the message is actionable
	// without the caller having to Unwrap. We cap the per-error text so
	// a single verbose provider error can't dominate the log line.
	for i, p := range e.Providers {
		if i >= len(e.Errors) {
			break
		}
		msg := e.Errors[i].Error()
		if len(msg) > 200 {
			msg = msg[:197] + "..."
		}
		fmt.Fprintf(&b, "\n  - %s: %s", p, msg)
	}
	return b.String()
}

// Unwrap returns the per-provider errors so errors.Is / errors.As traverse
// all of them. Returns nil when the multi-error carries no wrapped errors
// (should not happen in practice — Execute only constructs a multi-error
// after at least one failure — but defending against it keeps Unwrap's
// contract honest).
func (e *ProviderMultiError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return e.Errors
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
)

// GetGlobalRouter lazily initializes and returns the process-wide LLM router.
//
// sync.Once establishes the happens-before edge between the write to
// globalRouter inside Do and every subsequent read, so concurrent callers
// always observe a fully-initialized *LLMRouter. There is intentionally no
// ResetGlobalRouter: tearing down a shared singleton under concurrent access
// cannot be done safely without an atomic-pointer swap (a plain
// `globalRouter = nil; routerOnce = sync.Once{}` sequence opens a nil-deref
// window between the two assignments and races with unlocked readers). If
// test isolation is ever needed, build a fresh *LLMRouter via
// NewLLMRouterFromConfig and inject it instead of mutating the global.
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

// SelectProviders orders the router's active providers according to its
// configured strategy. It is preserved for callers (and tests) that select
// against the router's default strategy; per-call strategy overrides should
// go through Execute, which resolves the strategy from params without
// mutating router state.
func (r *LLMRouter) SelectProviders(ctx context.Context) []RouterProvider {
	return r.selectProviders(r.strategy)
}

// selectProviders applies the given strategy to the active provider set.
// Splitting this out from SelectProviders lets Execute honor a per-call
// strategy override without touching the shared router.strategy field
// (which would leak across concurrent workflows sharing the global router).
func (r *LLMRouter) selectProviders(strategy string) []RouterProvider {
	providers := r.getActiveProviders()

	if len(providers) == 0 {
		return providers
	}

	switch strategy {
	case config.RouterStrategyCost:
		return r.sortByCost(providers)
	case config.RouterStrategyLatency:
		return r.sortByLatency(providers)
	case config.RouterStrategyPareto:
		return sortByPareto(providers, r.stats)
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

// resolveStrategy returns the effective strategy for a call: the per-call
// override from params when valid, otherwise the router's configured default.
// Mirrors the max_retries override handling so neither field mutates shared
// router state.
func (r *LLMRouter) resolveStrategy(params map[string]string) string {
	if override := getParam(params, "strategy", ""); override != "" {
		switch override {
		case config.RouterStrategyPriority,
			config.RouterStrategyCost,
			config.RouterStrategyLatency,
			config.RouterStrategyPareto,
			config.RouterStrategyRoundRobin,
			config.RouterStrategyRandom:
			return override
		}
	}
	return r.strategy
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
			stats = newProviderStats()
			r.stats[p.Name] = stats
		}

		if stats.LastResetDate != today {
			stats.DailyUsage = 0
			stats.LastResetDate = today
		}

		if p.QuotaDaily > 0 && stats.DailyUsage >= p.QuotaDaily {
			continue
		}

		// Circuit breaker gates admission: an Open breaker excludes the
		// provider; HalfOpen allows a limited number of probe requests.
		// Replaces the old CooldownUntil time-based check.
		if stats.Breaker != nil && !stats.Breaker.AllowRequest() {
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
	// Snapshot the EWMA predictor pointer for each provider under the read
	// lock. Predict() has its own mutex, so we call it outside the statsMu
	// critical section to avoid nested locks.
	predictors := make(map[string]*EWMAPredictor, len(providers))
	r.statsMu.RLock()
	for _, p := range providers {
		if s := r.stats[p.Name]; s != nil {
			predictors[p.Name] = s.EwmaLatency
		}
	}
	r.statsMu.RUnlock()

	// latencyFor resolves the effective latency for a provider: the EWMA
	// prediction when available (and non-zero), falling back to the
	// arithmetic AvgLatencyMs so fresh providers without observations are
	// still rankable.
	latencyFor := func(p RouterProvider) float64 {
		if ep := predictors[p.Name]; ep != nil {
			if pred := ep.Predict(); pred > 0 {
				return pred
			}
		}
		return float64(p.AvgLatencyMs)
	}

	sort.SliceStable(providers, func(i, j int) bool {
		li := latencyFor(providers[i])
		lj := latencyFor(providers[j])
		if li != lj {
			return li < lj
		}
		return providers[i].SuccessRate > providers[j].SuccessRate
	})
	return providers
}

// sortByPareto orders providers by cost-quality Pareto efficiency.
// A provider is Pareto-optimal if no other provider is both cheaper AND
// faster. Non-optimal providers are ranked after optimal ones.
// This balances cost and latency better than sorting by a single dimension.
func sortByPareto(providers []RouterProvider, stats map[string]*ProviderStats) []RouterProvider {
	// Compute (cost, latency) for each provider
	type provCost struct {
		idx     int
		cost    float64
		latency float64
		optimal bool
	}
	costs := make([]provCost, len(providers))
	for i := range providers {
		name := providers[i].Name
		lat := float64(providers[i].AvgLatencyMs)
		if s, ok := stats[name]; ok && s != nil && s.EwmaLatency != nil {
			if p := s.EwmaLatency.Predict(); p > 0 {
				lat = p
			}
		}
		costs[i] = provCost{
			idx:     i,
			cost:    providers[i].CostPer1K,
			latency: lat,
		}
	}
	// Mark Pareto-optimal: a provider is optimal if no other is both
	// cheaper AND faster.
	for i := range costs {
		costs[i].optimal = true
		for j := range costs {
			if i == j {
				continue
			}
			if costs[j].cost < costs[i].cost && costs[j].latency < costs[i].latency {
				costs[i].optimal = false
				break
			}
		}
	}
	// Sort: optimal first (sorted by cost), then non-optimal (sorted by cost)
	sort.SliceStable(costs, func(i, j int) bool {
		if costs[i].optimal != costs[j].optimal {
			return costs[i].optimal // optimal providers first
		}
		// Within same optimality tier, sort by cost
		if costs[i].cost != costs[j].cost {
			return costs[i].cost < costs[j].cost
		}
		return costs[i].latency < costs[j].latency
	})
	result := make([]RouterProvider, len(providers))
	for i, c := range costs {
		result[i] = providers[c.idx]
	}
	return result
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
	// Resolve strategy per-call from params so a workflow's strategy
	// override never leaks into the shared global router.strategy field
	// (which would affect concurrent workflows).
	strategy := r.resolveStrategy(params)
	providers := r.selectProviders(strategy)
	if len(providers) == 0 {
		// B-3: publish the (empty) decision so the trace records that the
		// router was invoked but had no candidates.
		RouterDecisionSinkFrom(ctx).RecordRouterDecision(RouterDecision{
			Strategy:   strategy,
			Candidates: nil,
			Selected:   "",
			FinalError: "no active LLM providers available",
		})
		return "", "", fmt.Errorf("no active LLM providers available. Configure at least one provider via config or environment variables")
	}

	systemPrompt := getParam(params, "system", "")
	maxRetries := r.maxRetry
	if override := getParam(params, "max_retries", ""); override != "" {
		// Parse the override: a positive int is used verbatim; any parse
		// error or non-positive value falls back to the router default.
		if parsed, err := strconv.Atoi(strings.TrimSpace(override)); err == nil && parsed > 0 {
			maxRetries = parsed
		}
	}

	// B-3: build the candidate list and accumulate attempts for the
	// decision record published at the end (success or all-failed).
	candidateNames := make([]string, 0, len(providers))
	for _, p := range providers {
		candidateNames = append(candidateNames, p.Name)
	}
	var attempts []RouterAttempt
	// Accumulate EVERY per-provider error (not just the last) so the
	// final ProviderMultiError can expose all of them via Unwrap() []error.
	// This lets errors.Is(err, context.Canceled) succeed when cancellation
	// happened on any provider attempt, not just the final one — the
	// previous code wrapped only lastErr with %w, hiding earlier
	// cancellations behind a later, unrelated provider error.
	var triedProviders []string
	var providerErrors []error

	for attempt := 0; attempt < maxRetries && attempt < len(providers); attempt++ {
		// Honor cancellation between provider attempts. Without this, a
		// caller whose deadline already expired would still pay for N more
		// provider round-trips, and the eventual "all providers failed"
		// error would mask the real cause (context cancellation). The
		// non-blocking select keeps the happy path zero-overhead.
		select {
		case <-ctx.Done():
			cancelErr := ctx.Err()
			RouterDecisionSinkFrom(ctx).RecordRouterDecision(RouterDecision{
				Strategy:   strategy,
				Candidates: candidateNames,
				Selected:   "",
				Attempts:   attempts,
				FinalError: cancelErr.Error(),
			})
			return "", "", cancelErr
		default:
		}

		provider := providers[attempt]
		triedProviders = append(triedProviders, provider.Name)

		if provider.APIKey == "" && provider.Name != "ollama" {
			err := fmt.Errorf("provider %s has no API key configured", provider.Name)
			providerErrors = append(providerErrors, err)
			r.recordFailure(provider.Name, 0)
			attempts = append(attempts, RouterAttempt{
				Provider: provider.Name,
				Success:  false,
				Error:    err.Error(),
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
				Strategy:   strategy,
				Candidates: candidateNames,
				Selected:   provider.Name,
				Attempts:   attempts,
			})
			return result, provider.Name, nil
		}

		providerErrors = append(providerErrors, err)
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

	finalErr := &ProviderMultiError{
		Providers: triedProviders,
		Errors:    providerErrors,
	}
	RouterDecisionSinkFrom(ctx).RecordRouterDecision(RouterDecision{
		Strategy:   strategy,
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

	// EWMA latency prediction: recent observations weigh more than old
	// ones, so the predictor adapts quickly to performance changes.
	if stats.EwmaLatency != nil {
		stats.EwmaLatency.Observe(float64(latencyMs))
	}
	// Circuit breaker: a success may close the circuit from HalfOpen.
	if stats.Breaker != nil {
		stats.Breaker.RecordSuccess()
	}

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

	// Circuit breaker: a failure may trip Closed->Open or re-open from
	// HalfOpen. This replaces the old ConsecutiveFailures>=5 cooldown
	// block, which lacked a HalfOpen probe state.
	if stats.Breaker != nil {
		prevState := stats.Breaker.State()
		stats.Breaker.RecordFailure()
		if prevState != CircuitOpen && stats.Breaker.State() == CircuitOpen {
			logger.Warn("Provider circuit breaker opened",
				"provider", name,
				"failures", stats.Breaker.FailureCount(),
			)
		}
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
		stats = newProviderStats()
		r.stats[name] = stats
	}
	return stats
}

// newProviderStats constructs a fully-initialized ProviderStats with an EWMA
// latency predictor and a circuit breaker. All ProviderStats must be created
// through this constructor so the routing logic can assume non-nil
// EwmaLatency/Breaker fields.
func newProviderStats() *ProviderStats {
	return &ProviderStats{
		LastResetDate: time.Now().Format("2006-01-02"),
		EwmaLatency:   NewEWMAPredictor(0.3),
		Breaker:       NewCircuitBreaker(DefaultCircuitBreakerConfig()),
	}
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
