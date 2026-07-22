// Package provider provides LLM provider management with quota-aware fallback.
package provider

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// HealthStatus represents the health state of a provider.
type HealthStatus int

const (
	HealthHealthy HealthStatus = iota
	HealthDegraded
	HealthUnhealthy
)

func (h HealthStatus) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// Provider represents an LLM provider configuration.
type Provider struct {
	Name           string        `json:"name"`
	BaseURL        string        `json:"base_url"`
	Model          string        `json:"model"`
	APIKey         string        `json:"api_key,omitempty"`
	Priority       int           `json:"priority"`        // Higher = preferred
	QuotaLimit     int           `json:"quota_limit"`     // Requests per minute
	QuotaUsed      int           `json:"quota_used"`
	CostMultiplier float64       `json:"cost_multiplier"` // 1.0 = base price
	Health         HealthStatus  `json:"health"`
	LastError      string        `json:"last_error,omitempty"`
	LastSuccess    time.Time     `json:"last_success"`
	FailCount      int           `json:"fail_count"`
	mu             sync.RWMutex
}

// CanAccept checks if the provider can accept a new request.
func (p *Provider) CanAccept() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check quota
	if p.QuotaLimit > 0 && p.QuotaUsed >= p.QuotaLimit {
		return false
	}

	// Check health
	if p.Health == HealthUnhealthy {
		return false
	}

	return true
}

// RecordSuccess records a successful request.
func (p *Provider) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.LastSuccess = time.Now()
	p.FailCount = 0
	p.Health = HealthHealthy
	p.LastError = ""
}

// RecordFailure records a failed request.
func (p *Provider) RecordFailure(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.FailCount++
	p.LastError = err

	// Update health based on failure count
	if p.FailCount >= 5 {
		p.Health = HealthUnhealthy
	} else if p.FailCount >= 2 {
		p.Health = HealthDegraded
	}
}

// ResetQuota resets the quota counter (call every minute).
func (p *Provider) ResetQuota() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.QuotaUsed = 0
}

// ProviderManager manages multiple LLM providers with fallback.
type ProviderManager struct {
	mu           sync.RWMutex
	providers    []*Provider
	healthCheck  map[string]time.Time
	rand         *rand.Rand
	randMu       sync.Mutex
	onFallback   func(from, to string)
}

// NewProviderManager creates a new provider manager.
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers:   make([]*Provider, 0),
		healthCheck: make(map[string]time.Time),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddProvider adds a provider to the manager.
func (pm *ProviderManager) AddProvider(p *Provider) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.providers = append(pm.providers, p)
}

// GetProvider returns the best available provider.
// It considers: health status, quota availability, priority, and cost.
func (pm *ProviderManager) GetProvider() (*Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Filter healthy providers with available quota
	var candidates []*Provider
	for _, p := range pm.providers {
		if p.CanAccept() {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("all providers are at quota limit or unhealthy")
	}

	// Sort by priority (higher first), then by health (healthy first)
	// Return the best candidate
	best := candidates[0]
	for _, p := range candidates[1:] {
		if p.Priority > best.Priority {
			best = p
		} else if p.Priority == best.Priority && p.Health < best.Health {
			best = p
		}
	}

	return best, nil
}

// GetProviderWithFallback returns a provider, falling back to alternatives if primary fails.
func (pm *ProviderManager) GetProviderWithFallback(preferred string) (*Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Try preferred provider first
	if preferred != "" {
		for _, p := range pm.providers {
			if p.Name == preferred && p.CanAccept() {
				return p, nil
			}
		}
	}

	// Fallback to any available provider
	return pm.GetProvider()
}

// ExecuteWithFallback executes a request with automatic fallback on failure.
func (pm *ProviderManager) ExecuteWithFallback(
	ctx context.Context,
	preferred string,
	execute func(ctx context.Context, provider *Provider) error,
) error {
	var lastErr error
	triedProviders := make(map[string]bool)

	for {
		// Get next available provider (excluding already tried)
		provider, err := pm.getNextProvider(preferred, triedProviders)
		if err != nil {
			if lastErr != nil {
				return fmt.Errorf("all providers failed, last error: %v", lastErr)
			}
			return err
		}

		// Try execution
		err = execute(ctx, provider)
		if err == nil {
			provider.RecordSuccess()
			return nil
		}

		// Record failure and try next provider
		provider.RecordFailure(err.Error())
		triedProviders[provider.Name] = true
		lastErr = err

		// Notify fallback
		if pm.onFallback != nil {
			nextProvider, _ := pm.peekNextProvider(triedProviders)
			if nextProvider != nil {
				pm.onFallback(provider.Name, nextProvider.Name)
			}
		}
	}
}

// getNextProvider returns the next provider that hasn't been tried yet.
func (pm *ProviderManager) getNextProvider(preferred string, tried map[string]bool) (*Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Try preferred first if not tried
	if preferred != "" && !tried[preferred] {
		for _, p := range pm.providers {
			if p.Name == preferred && p.CanAccept() {
				return p, nil
			}
		}
	}

	// Find any available provider not yet tried
	for _, p := range pm.providers {
		if !tried[p.Name] && p.CanAccept() {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no available providers")
}

// peekNextProvider returns the next provider without marking it as tried.
func (pm *ProviderManager) peekNextProvider(tried map[string]bool) (*Provider, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, p := range pm.providers {
		if !tried[p.Name] && p.CanAccept() {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no available providers")
}

// SetFallbackCallback sets a callback for fallback events.
func (pm *ProviderManager) SetFallbackCallback(callback func(from, to string)) {
	pm.onFallback = callback
}

// StartHealthMonitor starts a background health check routine.
func (pm *ProviderManager) StartHealthMonitor(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Also reset quotas every minute
		quotaTicker := time.NewTicker(time.Minute)
		defer quotaTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pm.checkHealth(ctx)
			case <-quotaTicker.C:
				pm.resetQuotas()
			}
		}
	}()
}

// checkHealth performs health checks on all providers.
func (pm *ProviderManager) checkHealth(ctx context.Context) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, p := range pm.providers {
		// If provider has been unhealthy for > 5 minutes, reset health to try again
		if p.Health == HealthUnhealthy && time.Since(p.LastSuccess) > 5*time.Minute {
			p.mu.Lock()
			p.Health = HealthDegraded // Give it another chance
			p.FailCount = 0
			p.mu.Unlock()
		}
	}
}

// resetQuotas resets quota counters for all providers.
func (pm *ProviderManager) resetQuotas() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, p := range pm.providers {
		p.ResetQuota()
	}
}

// GetStatus returns the status of all providers.
func (pm *ProviderManager) GetStatus() []ProviderStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := make([]ProviderStatus, len(pm.providers))
	for i, p := range pm.providers {
		p.mu.RLock()
		status[i] = ProviderStatus{
			Name:       p.Name,
			Health:     p.Health.String(),
			QuotaUsed:  p.QuotaUsed,
			QuotaLimit: p.QuotaLimit,
			FailCount:  p.FailCount,
			LastError:  p.LastError,
		}
		p.mu.RUnlock()
	}
	return status
}

// ProviderStatus represents the status of a single provider.
type ProviderStatus struct {
	Name       string `json:"name"`
	Health     string `json:"health"`
	QuotaUsed  int    `json:"quota_used"`
	QuotaLimit int    `json:"quota_limit"`
	FailCount  int    `json:"fail_count"`
	LastError  string `json:"last_error,omitempty"`
}

// FormatStatus returns a human-readable status report.
func (pm *ProviderManager) FormatStatus() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var report string
	report += "🔌 Provider Status\n"
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	for _, p := range pm.providers {
		p.mu.RLock()
		healthIcon := "✅"
		if p.Health == HealthDegraded {
			healthIcon = "⚠️"
		} else if p.Health == HealthUnhealthy {
			healthIcon = "❌"
		}

		report += fmt.Sprintf("%s %s (priority: %d)\n", healthIcon, p.Name, p.Priority)
		if p.QuotaLimit > 0 {
			report += fmt.Sprintf("   Quota: %d/%d requests/min\n", p.QuotaUsed, p.QuotaLimit)
		}
		if p.FailCount > 0 {
			report += fmt.Sprintf("   Failures: %d\n", p.FailCount)
		}
		if p.LastError != "" {
			report += fmt.Sprintf("   Last error: %s\n", p.LastError)
		}
		p.mu.RUnlock()
		report += "\n"
	}

	return report
}

// DefaultProviders returns a list of common providers with default settings.
func DefaultProviders() []*Provider {
	return []*Provider{
		{
			Name:           "openai-gpt4o",
			BaseURL:        "https://api.openai.com/v1",
			Model:          "gpt-4o",
			Priority:       100,
			QuotaLimit:     500,
			CostMultiplier: 1.0,
			Health:         HealthHealthy,
		},
		{
			Name:           "anthropic-claude",
			BaseURL:        "https://api.anthropic.com/v1",
			Model:          "claude-3-5-sonnet",
			Priority:       95,
			QuotaLimit:     500,
			CostMultiplier: 1.2,
			Health:         HealthHealthy,
		},
		{
			Name:           "deepseek",
			BaseURL:        "https://api.deepseek.com/v1",
			Model:          "deepseek-chat",
			Priority:       80,
			QuotaLimit:     1000,
			CostMultiplier: 0.1,
			Health:         HealthHealthy,
		},
		{
			Name:           "ollama-local",
			BaseURL:        "http://localhost:11434",
			Model:          "llama3",
			Priority:       50,
			QuotaLimit:     0, // Unlimited
			CostMultiplier: 0, // Free
			Health:         HealthHealthy,
		},
	}
}

// GlobalProviderManager is the default provider manager.
var GlobalProviderManager = NewProviderManager()

// InitDefaultProviders initializes the global provider manager with default providers.
func InitDefaultProviders() {
	for _, p := range DefaultProviders() {
		GlobalProviderManager.AddProvider(p)
	}
}