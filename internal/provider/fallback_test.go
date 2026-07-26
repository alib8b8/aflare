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

package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHealthStatusString(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{HealthHealthy, "healthy"},
		{HealthDegraded, "degraded"},
		{HealthUnhealthy, "unhealthy"},
		{HealthStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("HealthStatus(%d).String() = %q, want %q", tt.status, got, tt.expected)
		}
	}
}

func TestProviderCanAccept(t *testing.T) {
	p := &Provider{
		Name:       "test",
		QuotaLimit: 5,
		QuotaUsed:  0,
		Health:     HealthHealthy,
	}

	if !p.CanAccept() {
		t.Error("Provider with quota and healthy should accept")
	}

	p.QuotaUsed = 5
	if p.CanAccept() {
		t.Error("Provider at quota limit should not accept")
	}

	p.QuotaUsed = 0
	p.Health = HealthUnhealthy
	if p.CanAccept() {
		t.Error("Unhealthy provider should not accept")
	}
}

func TestProviderRecordSuccess(t *testing.T) {
	p := &Provider{
		Name:      "test",
		FailCount: 3,
		Health:    HealthDegraded,
		LastError: "some error",
	}

	p.RecordSuccess()

	if p.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", p.FailCount)
	}
	if p.Health != HealthHealthy {
		t.Errorf("Health = %v, want %v", p.Health, HealthHealthy)
	}
	if p.LastError != "" {
		t.Errorf("LastError = %q, want empty", p.LastError)
	}
}

func TestProviderRecordFailure(t *testing.T) {
	p := &Provider{
		Name:   "test",
		Health: HealthHealthy,
	}

	p.RecordFailure("error 1")
	if p.FailCount != 1 {
		t.Errorf("After 1 failure: FailCount = %d, want 1", p.FailCount)
	}
	if p.Health != HealthHealthy {
		t.Errorf("After 1 failure: Health = %v, want %v", p.Health, HealthHealthy)
	}

	p.RecordFailure("error 2")
	if p.FailCount != 2 {
		t.Errorf("After 2 failures: FailCount = %d, want 2", p.FailCount)
	}
	if p.Health != HealthDegraded {
		t.Errorf("After 2 failures: Health = %v, want %v", p.Health, HealthDegraded)
	}

	p.RecordFailure("error 3")
	p.RecordFailure("error 4")
	p.RecordFailure("error 5")
	if p.FailCount != 5 {
		t.Errorf("After 5 failures: FailCount = %d, want 5", p.FailCount)
	}
	if p.Health != HealthUnhealthy {
		t.Errorf("After 5 failures: Health = %v, want %v", p.Health, HealthUnhealthy)
	}
}

func TestProviderResetQuota(t *testing.T) {
	p := &Provider{
		Name:      "test",
		QuotaUsed: 10,
	}

	p.ResetQuota()
	if p.QuotaUsed != 0 {
		t.Errorf("QuotaUsed = %d, want 0 after reset", p.QuotaUsed)
	}
}

func TestNewProviderManager(t *testing.T) {
	pm := NewProviderManager()
	if pm == nil {
		t.Error("NewProviderManager returned nil")
	}

	status := pm.GetStatus()
	if len(status) != 0 {
		t.Errorf("New manager should have 0 providers, got %d", len(status))
	}
}

func TestGetProviderEmpty(t *testing.T) {
	pm := NewProviderManager()
	_, err := pm.GetProvider()
	if err == nil {
		t.Error("Expected error when no providers available")
	}
}

func TestGetProviderPriority(t *testing.T) {
	pm := NewProviderManager()

	pm.AddProvider(&Provider{
		Name:     "low",
		Priority: 10,
		Health:   HealthHealthy,
	})
	pm.AddProvider(&Provider{
		Name:     "high",
		Priority: 100,
		Health:   HealthHealthy,
	})
	pm.AddProvider(&Provider{
		Name:     "medium",
		Priority: 50,
		Health:   HealthHealthy,
	})

	p, err := pm.GetProvider()
	if err != nil {
		t.Fatalf("GetProvider error: %v", err)
	}
	if p.Name != "high" {
		t.Errorf("Got provider %q, want %q", p.Name, "high")
	}
}

func TestGetProviderAllUnhealthy(t *testing.T) {
	pm := NewProviderManager()

	pm.AddProvider(&Provider{
		Name:     "p1",
		Priority: 100,
		Health:   HealthUnhealthy,
	})
	pm.AddProvider(&Provider{
		Name:     "p2",
		Priority: 50,
		Health:   HealthUnhealthy,
	})

	_, err := pm.GetProvider()
	if err == nil {
		t.Error("Expected error when all providers are unhealthy")
	}
}

func TestGetProviderWithFallback(t *testing.T) {
	pm := NewProviderManager()

	preferred := &Provider{
		Name:     "preferred",
		Priority: 50,
		Health:   HealthHealthy,
	}
	other := &Provider{
		Name:     "other",
		Priority: 100,
		Health:   HealthHealthy,
	}

	pm.AddProvider(preferred)
	pm.AddProvider(other)

	p, err := pm.GetProviderWithFallback("preferred")
	if err != nil {
		t.Fatalf("GetProviderWithFallback error: %v", err)
	}
	if p.Name != "preferred" {
		t.Errorf("Got provider %q, want %q", p.Name, "preferred")
	}
}

func TestGetProviderWithFallbackSkip(t *testing.T) {
	pm := NewProviderManager()

	preferred := &Provider{
		Name:     "preferred",
		Priority: 50,
		Health:   HealthUnhealthy,
	}
	other := &Provider{
		Name:     "other",
		Priority: 100,
		Health:   HealthHealthy,
	}

	pm.AddProvider(preferred)
	pm.AddProvider(other)

	p, err := pm.GetProviderWithFallback("preferred")
	if err != nil {
		t.Fatalf("GetProviderWithFallback error: %v", err)
	}
	if p.Name != "other" {
		t.Errorf("Got provider %q, want %q (fallback)", p.Name, "other")
	}
}

func TestExecuteWithFallback(t *testing.T) {
	pm := NewProviderManager()

	p1 := &Provider{
		Name:     "p1",
		Priority: 100,
		Health:   HealthHealthy,
	}
	pm.AddProvider(p1)

	err := pm.ExecuteWithFallback(context.Background(), "p1", func(ctx context.Context, p *Provider) error {
		return nil
	})
	if err != nil {
		t.Errorf("ExecuteWithFallback error: %v", err)
	}
}

func TestExecuteWithFallbackAllFail(t *testing.T) {
	pm := NewProviderManager()

	p1 := &Provider{
		Name:     "p1",
		Priority: 100,
		Health:   HealthHealthy,
	}
	p2 := &Provider{
		Name:     "p2",
		Priority: 50,
		Health:   HealthHealthy,
	}
	pm.AddProvider(p1)
	pm.AddProvider(p2)

	testErr := errors.New("execution failed")
	err := pm.ExecuteWithFallback(context.Background(), "", func(ctx context.Context, p *Provider) error {
		return testErr
	})
	if err == nil {
		t.Error("Expected error when all providers fail")
	}
}

func TestExecuteWithFallbackContextCancel(t *testing.T) {
	pm := NewProviderManager()

	p1 := &Provider{
		Name:     "p1",
		Priority: 100,
		Health:   HealthHealthy,
	}
	pm.AddProvider(p1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pm.ExecuteWithFallback(ctx, "p1", func(ctx context.Context, p *Provider) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

func TestGetStatus(t *testing.T) {
	pm := NewProviderManager()

	pm.AddProvider(&Provider{
		Name:       "p1",
		Priority:   100,
		Health:     HealthHealthy,
		QuotaUsed:  5,
		QuotaLimit: 100,
		FailCount:  0,
	})
	pm.AddProvider(&Provider{
		Name:       "p2",
		Priority:   50,
		Health:     HealthDegraded,
		QuotaUsed:  0,
		QuotaLimit: 50,
		FailCount:  2,
		LastError:  "timeout",
	})

	status := pm.GetStatus()
	if len(status) != 2 {
		t.Fatalf("GetStatus returned %d providers, want 2", len(status))
	}

	if status[0].Name != "p1" {
		t.Errorf("status[0].Name = %q, want %q", status[0].Name, "p1")
	}
	if status[0].Health != "healthy" {
		t.Errorf("status[0].Health = %q, want %q", status[0].Health, "healthy")
	}
	if status[1].Health != "degraded" {
		t.Errorf("status[1].Health = %q, want %q", status[1].Health, "degraded")
	}
	if status[1].FailCount != 2 {
		t.Errorf("status[1].FailCount = %d, want 2", status[1].FailCount)
	}
}

func TestFormatStatus(t *testing.T) {
	pm := NewProviderManager()

	pm.AddProvider(&Provider{
		Name:       "test-provider",
		Priority:   100,
		Health:     HealthHealthy,
		QuotaUsed:  3,
		QuotaLimit: 100,
	})

	report := pm.FormatStatus()
	if report == "" {
		t.Error("FormatStatus returned empty report")
	}
	if len(report) < 10 {
		t.Errorf("FormatStatus report too short: %d chars", len(report))
	}
}

func TestDefaultProviders(t *testing.T) {
	providers := DefaultProviders()
	if len(providers) != 4 {
		t.Fatalf("DefaultProviders returned %d providers, want 4", len(providers))
	}

	expectedNames := []string{"openai-gpt4o", "anthropic-claude", "deepseek", "ollama-local"}
	for i, name := range expectedNames {
		if providers[i].Name != name {
			t.Errorf("providers[%d].Name = %q, want %q", i, providers[i].Name, name)
		}
	}

	if providers[0].Priority != 100 {
		t.Errorf("openai-gpt4o priority = %d, want 100", providers[0].Priority)
	}
	if providers[3].QuotaLimit != 0 {
		t.Errorf("ollama-local QuotaLimit = %d, want 0 (unlimited)", providers[3].QuotaLimit)
	}
}
