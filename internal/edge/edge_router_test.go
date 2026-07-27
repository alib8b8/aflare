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

package edge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================
// Mock implementations
// ============================================================

type mockLocalModel struct {
	available bool
	modelName string
	metrics   LocalMetrics
	execFn    func(ctx context.Context, prompt string, opts map[string]string) (string, error)
	calls     int
	mu        sync.Mutex
}

func (m *mockLocalModel) IsAvailable() bool { return m.available }
func (m *mockLocalModel) GetModelName() string {
	if m.modelName != "" {
		return m.modelName
	}
	return "mock-local"
}
func (m *mockLocalModel) Execute(ctx context.Context, prompt string, opts map[string]string) (string, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.execFn != nil {
		return m.execFn(ctx, prompt, opts)
	}
	return "local-output: " + prompt, nil
}
func (m *mockLocalModel) GetMetrics() LocalMetrics { return m.metrics }

type mockCloudModel struct {
	provider string
	execFn   func(ctx context.Context, prompt string, opts map[string]string) (string, error)
	calls    int
	mu       sync.Mutex
}

func (m *mockCloudModel) GetProviderName() string { return m.provider }
func (m *mockCloudModel) Execute(ctx context.Context, prompt string, opts map[string]string) (string, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.execFn != nil {
		return m.execFn(ctx, prompt, opts)
	}
	return "cloud-output: " + prompt, nil
}
func (m *mockCloudModel) GetMetrics() CloudMetrics {
	return CloudMetrics{Provider: m.provider, Available: true}
}

func validConfig() EdgeConfig {
	return EdgeConfig{
		PrivacyLevel:   PrivacyBalanced,
		LocalThreshold: 5,
		MaxLatency:     1000,
		CloudModels: map[string]CloudModelConfig{
			"openai": {
				Endpoint: "https://api.openai.com/v1",
				Model:    "gpt-4",
				Priority: 10,
			},
			"anthropic": {
				Endpoint: "https://api.anthropic.com/v1",
				Model:    "claude-3",
				Priority: 20,
			},
		},
	}
}

// ============================================================
// NewEdgeRouter & validateEdgeConfig
// ============================================================

func TestNewEdgeRouter_Valid(t *testing.T) {
	cfg := validConfig()
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil router")
	}
	if r.config.PrivacyLevel != PrivacyBalanced {
		t.Errorf("expected privacy balanced, got %s", r.config.PrivacyLevel)
	}
	if len(r.cloudModels) != 0 {
		t.Errorf("expected empty cloudModels map initially, got %d", len(r.cloudModels))
	}
	if r.stats.LocalCalls != 0 || r.stats.CloudCalls != 0 {
		t.Error("expected zero stats")
	}
}

func TestNewEdgeRouter_InvalidConfigs(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(EdgeConfig) EdgeConfig
		wantErr string
	}{
		{
			name:    "threshold_negative",
			mutate:  func(c EdgeConfig) EdgeConfig { c.LocalThreshold = -1; return c },
			wantErr: "local_threshold must be between 0 and 10",
		},
		{
			name:    "threshold_too_high",
			mutate:  func(c EdgeConfig) EdgeConfig { c.LocalThreshold = 11; return c },
			wantErr: "local_threshold must be between 0 and 10",
		},
		{
			name:    "max_latency_negative",
			mutate:  func(c EdgeConfig) EdgeConfig { c.MaxLatency = -5; return c },
			wantErr: "max_latency cannot be negative",
		},
		{
			name:    "invalid_privacy_level",
			mutate:  func(c EdgeConfig) EdgeConfig { c.PrivacyLevel = "unknown"; return c },
			wantErr: "invalid privacy_level",
		},
		{
			name:    "empty_privacy_level",
			mutate:  func(c EdgeConfig) EdgeConfig { c.PrivacyLevel = ""; return c },
			wantErr: "invalid privacy_level",
		},
		{
			name: "missing_endpoint",
			mutate: func(c EdgeConfig) EdgeConfig {
				c.CloudModels["openai"] = CloudModelConfig{Model: "gpt-4"}
				return c
			},
			wantErr: "requires endpoint",
		},
		{
			name: "missing_model",
			mutate: func(c EdgeConfig) EdgeConfig {
				c.CloudModels["openai"] = CloudModelConfig{Endpoint: "https://api.openai.com/v1"}
				return c
			},
			wantErr: "requires model",
		},
		{
			name: "bad_scheme",
			mutate: func(c EdgeConfig) EdgeConfig {
				c.CloudModels["openai"] = CloudModelConfig{
					Endpoint: "ftp://example.com",
					Model:    "gpt-4",
				}
				return c
			},
			wantErr: "must use http or https",
		},
		{
			name: "credentials_in_url",
			mutate: func(c EdgeConfig) EdgeConfig {
				c.CloudModels["openai"] = CloudModelConfig{
					Endpoint: "https://user:pass@api.openai.com/v1",
					Model:    "gpt-4",
				}
				return c
			},
			wantErr: "cannot contain credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.mutate(validConfig())
			r, err := NewEdgeRouter(cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if r != nil {
				t.Errorf("expected nil router on error, got non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestNewEdgeRouter_ProviderNameTooLong(t *testing.T) {
	cfg := validConfig()
	longName := strings.Repeat("a", maxProviderNameLen+1)
	cfg.CloudModels[longName] = CloudModelConfig{
		Endpoint: "https://api.example.com",
		Model:    "model",
	}
	_, err := NewEdgeRouter(cfg)
	if err == nil {
		t.Fatal("expected error for overly long provider name")
	}
	if !strings.Contains(err.Error(), "provider name too long") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// validateEndpoint
// ============================================================

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		ep      string
		wantErr bool
	}{
		{"valid_http", "http://localhost:8080", false},
		{"valid_https", "https://api.openai.com/v1", false},
		{"bad_scheme_ftp", "ftp://example.com", true},
		{"bad_scheme_file", "file:///etc/passwd", true},
		{"no_scheme", "example.com", true},
		{"with_credentials", "https://user:pass@example.com", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoint(tt.ep)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.ep)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for %q, got %v", tt.ep, err)
			}
		})
	}
}

func TestValidateEndpoint_TooLong(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 2100)
	err := validateEndpoint(long)
	if err == nil {
		t.Fatal("expected error for overly long endpoint")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// Route
// ============================================================

func TestRoute_PrivacyStrict_AlwaysLocal(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyStrict
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	task := TaskRequest{ID: "t1", Prompt: "anything", ContainsSensitiveData: false}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dec.Target != TargetLocal {
		t.Errorf("expected local, got %s", dec.Target)
	}
	if dec.Reason != "privacy_strict_mode" {
		t.Errorf("unexpected reason: %s", dec.Reason)
	}
	if dec.TaskID != "t1" {
		t.Errorf("task id mismatch: %s", dec.TaskID)
	}
	if dec.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestRoute_BalancedSensitive_AlwaysLocal(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyBalanced
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Sensitive data + balanced should always go local regardless of complexity
	task := TaskRequest{
		ID:                    "t1",
		Prompt:                "深度分析这段数据", // complex keyword
		ContainsSensitiveData: true,
	}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dec.Target != TargetLocal {
		t.Errorf("expected local for sensitive+balanced, got %s", dec.Target)
	}
	if dec.Reason != "contains_sensitive_data" {
		t.Errorf("unexpected reason: %s", dec.Reason)
	}
}

func TestRoute_LowComplexity_LocalAvailable(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyPermissive
	cfg.LocalThreshold = 5
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.localModel = &mockLocalModel{available: true}
	// Simple keyword reduces complexity to 3, below threshold
	task := TaskRequest{ID: "t1", Prompt: "请总结这篇文章"}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dec.Target != TargetLocal {
		t.Errorf("expected local, got %s (reason: %s)", dec.Target, dec.Reason)
	}
	if !strings.HasPrefix(dec.Reason, "low_complexity_") {
		t.Errorf("expected low_complexity reason, got %s", dec.Reason)
	}
}

func TestRoute_HighComplexity_Cloud(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyPermissive
	cfg.LocalThreshold = 5
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Complex keyword bumps complexity above threshold
	task := TaskRequest{ID: "t1", Prompt: "请深度分析这段代码"}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dec.Target != TargetCloud {
		t.Errorf("expected cloud, got %s (reason: %s)", dec.Target, dec.Reason)
	}
	if !strings.HasPrefix(dec.Reason, "high_complexity_") {
		t.Errorf("expected high_complexity reason, got %s", dec.Reason)
	}
	if dec.Provider != "anthropic" {
		t.Errorf("expected anthropic (highest priority), got %s", dec.Provider)
	}
}

func TestRoute_Permissive_Sensitive_StillCloud(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyPermissive
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	task := TaskRequest{
		ID:                    "t1",
		Prompt:                "深度分析",
		ContainsSensitiveData: true,
	}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dec.Target != TargetCloud {
		t.Errorf("expected cloud for permissive, got %s", dec.Target)
	}
}

func TestRoute_BalancedFallback_NoLocalModel(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyBalanced
	cfg.LocalThreshold = 5
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// No local model, balanced privacy, non-sensitive + high complexity
	// Should fall through to cloud path (since !ContainsSensitiveData)
	task := TaskRequest{ID: "t1", Prompt: "深度分析"}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dec.Target != TargetCloud {
		t.Errorf("expected cloud, got %s (reason: %s)", dec.Target, dec.Reason)
	}
}

func TestRoute_BalancedSensitive_HighComplexityNoLocal_FallsToLocal(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyBalanced
	cfg.LocalThreshold = 0 // forces complexity > threshold (5 > 0)
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Sensitive + balanced + high complexity + no local model = privacy_balanced_fallback
	task := TaskRequest{
		ID:                    "t1",
		Prompt:                "深度分析",
		ContainsSensitiveData: true,
	}
	dec, err := r.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// With balanced+sensitive, the early-return at "contains_sensitive_data" fires first
	if dec.Target != TargetLocal {
		t.Errorf("expected local, got %s", dec.Target)
	}
}

func TestRoute_InvalidTask(t *testing.T) {
	r, err := NewEdgeRouter(validConfig())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty_id", func(t *testing.T) {
		_, err := r.Route(context.Background(), TaskRequest{Prompt: "x"})
		if err == nil || !strings.Contains(err.Error(), "task ID is required") {
			t.Errorf("expected ID required error, got %v", err)
		}
	})

	t.Run("empty_prompt", func(t *testing.T) {
		_, err := r.Route(context.Background(), TaskRequest{ID: "t1"})
		if err == nil || !strings.Contains(err.Error(), "prompt cannot be empty") {
			t.Errorf("expected empty prompt error, got %v", err)
		}
	})

	t.Run("id_too_long", func(t *testing.T) {
		longID := strings.Repeat("a", maxTaskIDLength+1)
		_, err := r.Route(context.Background(), TaskRequest{ID: longID, Prompt: "x"})
		if err == nil || !strings.Contains(err.Error(), "task ID too long") {
			t.Errorf("expected ID too long error, got %v", err)
		}
	})

	t.Run("prompt_too_long", func(t *testing.T) {
		longPrompt := strings.Repeat("a", maxPromptLength+1)
		_, err := r.Route(context.Background(), TaskRequest{ID: "t1", Prompt: longPrompt})
		if err == nil || !strings.Contains(err.Error(), "prompt too long") {
			t.Errorf("expected prompt too long error, got %v", err)
		}
	})
}

// ============================================================
// Execute
// ============================================================

func TestExecute_Local(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyStrict // forces local
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.localModel = &mockLocalModel{available: true}

	task := TaskRequest{ID: "t1", Prompt: "hello"}
	res, err := r.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.Success {
		t.Error("expected success")
	}
	if !strings.HasPrefix(res.Output, "local-output:") {
		t.Errorf("unexpected output: %s", res.Output)
	}
	if res.TaskID != "t1" {
		t.Errorf("task id mismatch: %s", res.TaskID)
	}
	if res.LatencyMs < 0 {
		t.Error("latency should be non-negative")
	}
	if res.Decision.Target != TargetLocal {
		t.Errorf("expected local decision, got %s", res.Decision.Target)
	}

	stats := r.GetStats()
	if stats.LocalCalls != 1 || stats.LocalSuccess != 1 {
		t.Errorf("expected 1 local call/success, got %+v", stats)
	}
	if stats.SavingsPct != 100 {
		t.Errorf("expected 100%% savings, got %f", stats.SavingsPct)
	}
}

func TestExecute_Cloud(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyPermissive
	cfg.LocalThreshold = 0 // high complexity -> cloud
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.cloudModels["anthropic"] = &mockCloudModel{provider: "anthropic"}

	task := TaskRequest{ID: "t1", Prompt: "深度分析"}
	res, err := r.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.Success {
		t.Error("expected success")
	}
	if !strings.HasPrefix(res.Output, "cloud-output:") {
		t.Errorf("unexpected output: %s", res.Output)
	}
	if res.Decision.Provider != "anthropic" {
		t.Errorf("expected anthropic provider, got %s", res.Decision.Provider)
	}

	stats := r.GetStats()
	if stats.CloudCalls != 1 || stats.CloudSuccess != 1 {
		t.Errorf("expected 1 cloud call/success, got %+v", stats)
	}
	if stats.SavingsPct != 0 {
		t.Errorf("expected 0%% savings, got %f", stats.SavingsPct)
	}
}

func TestExecute_Fallback_LocalToCloud(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyStrict // forces local
	cfg.EnableFallback = true
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// local model unavailable, but cloud mock configured for fallback
	r.cloudModels["anthropic"] = &mockCloudModel{provider: "anthropic"}

	task := TaskRequest{ID: "t1", Prompt: "hello"}
	res, err := r.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("expected no error from fallback, got %v", err)
	}
	if !res.Success {
		t.Error("expected success")
	}
	if !res.FallbackUsed {
		t.Error("expected fallback used")
	}
	if !strings.HasPrefix(res.Output, "cloud-output:") {
		t.Errorf("expected cloud output via fallback, got %s", res.Output)
	}

	stats := r.GetStats()
	// local call is counted even though it failed
	if stats.LocalCalls != 1 {
		t.Errorf("expected 1 local call, got %d", stats.LocalCalls)
	}
	if stats.LocalSuccess != 0 {
		t.Errorf("expected 0 local success (local failed), got %d", stats.LocalSuccess)
	}
	// fallback cloud call is counted in cloud stats (fixed: previously the
	// EnableFallback path bypassed the TargetCloud branch and never updated
	// CloudCalls/CloudSuccess, skewing SavingsPct).
	if stats.CloudCalls != 1 {
		t.Errorf("expected 1 cloud call (fallback updates cloud stats), got %d", stats.CloudCalls)
	}
	if stats.CloudSuccess != 1 {
		t.Errorf("expected 1 cloud success (fallback succeeded), got %d", stats.CloudSuccess)
	}
	// SavingsPct = LocalCalls / (LocalCalls + CloudCalls) * 100 = 1/2 * 100 = 50
	if stats.SavingsPct != 50 {
		t.Errorf("expected 50%% savings (1 local of 2 total calls), got %f", stats.SavingsPct)
	}
}

func TestExecute_UnknownCloudProvider(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyPermissive
	cfg.LocalThreshold = 0
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// No cloud models registered in r.cloudModels map
	// selectBestCloudProvider returns "openai" by default but it's not in the map
	task := TaskRequest{ID: "t1", Prompt: "深度分析"}
	_, err = r.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for unknown cloud provider")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecute_NoLocalModel_Strict(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyStrict
	cfg.EnableFallback = false
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	task := TaskRequest{ID: "t1", Prompt: "hello"}
	_, err = r.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for missing local model")
	}
	if !strings.Contains(err.Error(), "local model not available") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// analyzeComplexity
// ============================================================

func TestAnalyzeComplexity(t *testing.T) {
	r, err := NewEdgeRouter(validConfig())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		prompt string
		want   int
	}{
		{"default_short", "hello world", 5},
		{"complex_keyword_zh", "请分析这段内容", 7},
		{"complex_keyword_en", "please analyze this", 7},
		{"simple_keyword_zh", "请总结这段内容", 3},
		{"simple_keyword_en", "please summarize this", 3},
		{"both_keywords", "分析并总结", 5}, // +2 then -2
		{"medium_prompt", strings.Repeat("a", 600), 6},
		{"long_prompt", strings.Repeat("a", 1100), 7},
		{"long_prompt_with_complex", "analyze " + strings.Repeat("a", 1100), 9},
		{"long_prompt_with_simple", "summarize " + strings.Repeat("a", 1100), 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.analyzeComplexity(TaskRequest{Prompt: tt.prompt})
			if got != tt.want {
				t.Errorf("analyzeComplexity(%q) = %d, want %d", tt.prompt, got, tt.want)
			}
		})
	}
}

func TestAnalyzeComplexity_Clamps(t *testing.T) {
	r, err := NewEdgeRouter(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	// Construct a prompt with multiple complex keywords - but only first match adds +2 (break).
	// So max is 5 + 2 (complex) + 2 (length>1000) = 9, never reaching clamp of 10.
	// For lower bound: only simple keyword fires -2 once, so 5-2=3, never reaching clamp of 0.
	// Verify clamp boundaries indirectly: complexity is always within [0, 10].
	for _, prompt := range []string{"", "summarize", "analyze", strings.Repeat("a", 2000)} {
		got := r.analyzeComplexity(TaskRequest{Prompt: prompt})
		if got < 0 || got > 10 {
			t.Errorf("complexity out of [0,10] for %q: %d", prompt, got)
		}
	}
}

// ============================================================
// selectBestCloudProvider
// ============================================================

func TestSelectBestCloudProvider(t *testing.T) {
	t.Run("picks_highest_priority", func(t *testing.T) {
		cfg := validConfig()
		r, err := NewEdgeRouter(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got := r.selectBestCloudProvider()
		if got != "anthropic" {
			t.Errorf("expected anthropic (priority 20), got %s", got)
		}
	})

	t.Run("empty_returns_openai_default", func(t *testing.T) {
		cfg := validConfig()
		cfg.CloudModels = nil
		r, err := NewEdgeRouter(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got := r.selectBestCloudProvider()
		if got != "openai" {
			t.Errorf("expected openai default, got %s", got)
		}
	})

	t.Run("single_provider", func(t *testing.T) {
		cfg := validConfig()
		cfg.CloudModels = map[string]CloudModelConfig{
			"onlyone": {Endpoint: "https://example.com", Model: "m", Priority: 1},
		}
		r, err := NewEdgeRouter(cfg)
		if err != nil {
			t.Fatal(err)
		}
		got := r.selectBestCloudProvider()
		if got != "onlyone" {
			t.Errorf("expected onlyone, got %s", got)
		}
	})
}

// ============================================================
// GetStats
// ============================================================

func TestGetStats_SavingsCalculation(t *testing.T) {
	r, err := NewEdgeRouter(validConfig())
	if err != nil {
		t.Fatal(err)
	}

	// No calls -> 0% savings
	stats := r.GetStats()
	if stats.SavingsPct != 0 {
		t.Errorf("expected 0%% savings with no calls, got %f", stats.SavingsPct)
	}

	// 3 local, 1 cloud -> 75% savings
	r.mu.Lock()
	r.stats.LocalCalls = 3
	r.stats.CloudCalls = 1
	r.mu.Unlock()
	stats = r.GetStats()
	if stats.SavingsPct != 75 {
		t.Errorf("expected 75%% savings, got %f", stats.SavingsPct)
	}

	// All cloud -> 0%
	r.mu.Lock()
	r.stats.LocalCalls = 0
	r.stats.CloudCalls = 5
	r.mu.Unlock()
	stats = r.GetStats()
	if stats.SavingsPct != 0 {
		t.Errorf("expected 0%% savings for all cloud, got %f", stats.SavingsPct)
	}
}

// ============================================================
// OllamaModel
// ============================================================

func TestOllamaModel_Basic(t *testing.T) {
	m := NewOllamaModel("http://localhost:11434", "llama3:8b")
	if m == nil {
		t.Fatal("expected non-nil model")
	}
	if m.GetModelName() != "llama3:8b" {
		t.Errorf("expected llama3:8b, got %s", m.GetModelName())
	}
	if m.IsAvailable() {
		t.Error("expected unavailable by default")
	}
	out, err := m.Execute(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out, "llama3:8b") {
		t.Errorf("expected output to contain model name, got %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected output to contain prompt, got %s", out)
	}
	// GetMetrics returns zero-value struct (field never populated)
	metrics := m.GetMetrics()
	if metrics.Available {
		t.Error("expected metrics.Available=false since never set")
	}
}

func TestOllamaModel_Execute_LongPromptTruncated(t *testing.T) {
	m := NewOllamaModel("http://localhost:11434", "llama3")
	longPrompt := strings.Repeat("x", 200)
	out, err := m.Execute(context.Background(), longPrompt, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Output should only include first 50 chars of prompt
	if strings.Contains(out, strings.Repeat("x", 60)) {
		t.Error("expected prompt to be truncated to 50 chars in output")
	}
}

// ============================================================
// PrivacyAnalyzer
// ============================================================

func TestPrivacyAnalyzer(t *testing.T) {
	p := NewPrivacyAnalyzer()
	if p == nil {
		t.Fatal("expected non-nil analyzer")
	}

	tests := []struct {
		name string
		text string
		want bool
	}{
		{"clean_text", "the weather is nice today", false},
		{"password_en", "my password is 123456", true},
		// 中文敏感词 pattern 不使用 \b（Go RE2 的 \b 是 ASCII 词边界，
		// 在中文相邻时永不命中）。修复后纯中文文本能正确匹配。
		{"password_zh_matches", "我的密码是123456", true},
		{"id_card_zh_matches", "我的身份证号是", true},
		{"address_zh_matches", "我的地址是北京", true},
		{"secret", "this is a secret value", true},
		{"token", "bearer token here", true},
		{"email", "contact me at user@example.com", true},
		{"phone_us", "call 555-123-4567", true},
		{"phone_zh_11digit", "手机号 13812345678", true}, // matches `\d{11}`
		{"api_key_pattern", "sk-" + strings.Repeat("a", 25), true},
		{"github_token", "ghp_" + strings.Repeat("a", 25), true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Analyze(tt.text)
			if got != tt.want {
				t.Errorf("Analyze(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// ============================================================
// AgentInfo.Validate & AgentRegistry
// ============================================================

func TestAgentInfo_Validate(t *testing.T) {
	tests := []struct {
		name    string
		agent   *AgentInfo
		wantErr string
	}{
		{
			name:    "empty_did",
			agent:   &AgentInfo{DID: "", Endpoint: "https://example.com"},
			wantErr: "agent DID is required",
		},
		{
			name:    "did_too_long",
			agent:   &AgentInfo{DID: "did:" + strings.Repeat("a", 260), Endpoint: "https://example.com"},
			wantErr: "agent DID too long",
		},
		{
			name:    "did_no_prefix",
			agent:   &AgentInfo{DID: "just-an-id", Endpoint: "https://example.com"},
			wantErr: "invalid agent DID format",
		},
		{
			name:    "did_only_two_parts",
			agent:   &AgentInfo{DID: "did:only", Endpoint: "https://example.com"},
			wantErr: "invalid DID format",
		},
		{
			name:    "empty_endpoint",
			agent:   &AgentInfo{DID: "did:web:alice"},
			wantErr: "agent endpoint is required",
		},
		{
			name: "bad_endpoint_scheme",
			agent: &AgentInfo{
				DID:      "did:web:alice",
				Endpoint: "ftp://example.com",
			},
			wantErr: "invalid agent endpoint",
		},
		{
			name: "too_many_capabilities",
			agent: &AgentInfo{
				DID:          "did:web:alice",
				Endpoint:     "https://example.com",
				Capabilities: make([]string, maxAgentCapabilities+1),
			},
			wantErr: "too many capabilities",
		},
		{
			name: "capability_too_long",
			agent: &AgentInfo{
				DID:          "did:web:alice",
				Endpoint:     "https://example.com",
				Capabilities: []string{strings.Repeat("x", 101)},
			},
			wantErr: "capability name too long",
		},
		{
			name: "too_many_metadata_entries",
			agent: &AgentInfo{
				DID:      "did:web:alice",
				Endpoint: "https://example.com",
				Metadata: func() map[string]string {
					m := make(map[string]string)
					for i := 0; i < maxAgentMetadataEntries+1; i++ {
						m[fmt.Sprintf("k%d", i)] = "v"
					}
					return m
				}(),
			},
			wantErr: "too many metadata entries",
		},
		{
			name: "metadata_key_too_long",
			agent: &AgentInfo{
				DID:      "did:web:alice",
				Endpoint: "https://example.com",
				Metadata: map[string]string{strings.Repeat("k", maxAgentMetadataKeyLen+1): "v"},
			},
			wantErr: "metadata key too long",
		},
		{
			name: "metadata_value_too_long",
			agent: &AgentInfo{
				DID:      "did:web:alice",
				Endpoint: "https://example.com",
				Metadata: map[string]string{"k": strings.Repeat("v", maxAgentMetadataValueLen+1)},
			},
			wantErr: "metadata value too long",
		},
		{
			name: "valid",
			agent: &AgentInfo{
				DID:          "did:web:alice",
				Endpoint:     "https://example.com",
				Capabilities: []string{"translate", "summarize"},
				Metadata:     map[string]string{"version": "1.0"},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.agent.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestAgentRegistry_CRUD(t *testing.T) {
	reg := NewAgentRegistry()

	// Initially empty
	if agents := reg.ListAgents(); len(agents) != 0 {
		t.Errorf("expected empty registry, got %d", len(agents))
	}

	// Register
	agent1 := &AgentInfo{
		DID:          "did:web:alice",
		Endpoint:     "https://alice.example.com",
		Capabilities: []string{"translate", "summarize"},
		Trusted:      true,
	}
	if err := reg.RegisterAgent(agent1); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	if agent1.LastSeen.IsZero() {
		t.Error("expected LastSeen to be set on registration")
	}

	// Get
	got, ok := reg.GetAgent("did:web:alice")
	if !ok {
		t.Fatal("expected to find registered agent")
	}
	if got.DID != "did:web:alice" {
		t.Errorf("DID mismatch: %s", got.DID)
	}

	// Get missing
	_, ok = reg.GetAgent("did:web:unknown")
	if ok {
		t.Error("expected not found for unregistered DID")
	}

	// List
	if n := len(reg.ListAgents()); n != 1 {
		t.Errorf("expected 1 agent, got %d", n)
	}

	// Discover by capability
	matches := reg.DiscoverAgentsByCapability("translate")
	if len(matches) != 1 {
		t.Errorf("expected 1 match for translate, got %d", len(matches))
	}
	matches = reg.DiscoverAgentsByCapability("nonexistent")
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}

	// Register invalid agent
	invalid := &AgentInfo{DID: "invalid"}
	if err := reg.RegisterAgent(invalid); err == nil {
		t.Error("expected error for invalid agent")
	}

	// Remove
	reg.RemoveAgent("did:web:alice")
	if _, ok := reg.GetAgent("did:web:alice"); ok {
		t.Error("expected agent to be removed")
	}
}

func TestAgentRegistry_UpdateExisting(t *testing.T) {
	reg := NewAgentRegistry()
	agent := &AgentInfo{
		DID:      "did:web:bob",
		Endpoint: "https://bob.example.com",
		Trusted:  true,
	}
	if err := reg.RegisterAgent(agent); err != nil {
		t.Fatal(err)
	}
	originalLastSeen := agent.LastSeen
	time.Sleep(time.Millisecond)

	// Re-register with updated capabilities
	agent.Capabilities = []string{"new-cap"}
	if err := reg.RegisterAgent(agent); err != nil {
		t.Fatalf("re-registration failed: %v", err)
	}
	if !agent.LastSeen.After(originalLastSeen) {
		t.Error("expected LastSeen to update on re-registration")
	}
	if n := len(reg.ListAgents()); n != 1 {
		t.Errorf("expected still 1 agent after update, got %d", n)
	}
}

func TestAgentRegistry_Overflow(t *testing.T) {
	reg := NewAgentRegistry()
	for i := 0; i < maxRegistrySize; i++ {
		err := reg.RegisterAgent(&AgentInfo{
			DID:      fmt.Sprintf("did:web:agent%d", i),
			Endpoint: "https://example.com",
		})
		if err != nil {
			t.Fatalf("failed at %d: %v", i, err)
		}
	}
	// Next registration should fail since the registry is full (and DID is new)
	err := reg.RegisterAgent(&AgentInfo{
		DID:      "did:web:newone",
		Endpoint: "https://example.com",
	})
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !strings.Contains(err.Error(), "registry is full") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// CrossDomainMessenger
// ============================================================

func TestCrossDomainMessenger(t *testing.T) {
	reg := NewAgentRegistry()
	alice := &AgentInfo{
		DID:      "did:web:alice",
		Endpoint: "https://alice.example.com",
		Trusted:  true,
	}
	if err := reg.RegisterAgent(alice); err != nil {
		t.Fatal(err)
	}
	bob := &AgentInfo{
		DID:      "did:web:bob",
		Endpoint: "https://bob.example.com",
		Trusted:  false, // untrusted
	}
	if err := reg.RegisterAgent(bob); err != nil {
		t.Fatal(err)
	}
	m := NewCrossDomainMessenger(reg)

	t.Run("send_success", func(t *testing.T) {
		err := m.SendMessage("did:web:alice", "did:web:bob", "hello")
		// bob is untrusted -> error
		if err == nil {
			t.Error("expected error for untrusted receiver")
		}
		if !strings.Contains(err.Error(), "not trusted") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("send_to_trusted", func(t *testing.T) {
		err := m.SendMessage("did:web:bob", "did:web:alice", "hi")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		sent, errs := m.GetStats()
		if sent != 1 {
			t.Errorf("expected 1 sent, got %d", sent)
		}
		if errs != 0 {
			t.Errorf("expected 0 errors, got %d", errs)
		}
	})

	t.Run("missing_sender", func(t *testing.T) {
		err := m.SendMessage("", "did:web:alice", "hi")
		if err == nil || !strings.Contains(err.Error(), "sender and receiver") {
			t.Errorf("expected sender required error, got %v", err)
		}
	})

	t.Run("sender_equals_receiver", func(t *testing.T) {
		err := m.SendMessage("did:web:alice", "did:web:alice", "hi")
		if err == nil || !strings.Contains(err.Error(), "must be different") {
			t.Errorf("expected different error, got %v", err)
		}
	})

	t.Run("invalid_sender_format", func(t *testing.T) {
		err := m.SendMessage("invalid-did", "did:web:alice", "hi")
		if err == nil || !strings.Contains(err.Error(), "invalid sender DID") {
			t.Errorf("expected invalid sender DID error, got %v", err)
		}
	})

	t.Run("invalid_receiver_format", func(t *testing.T) {
		err := m.SendMessage("did:web:alice", "invalid-did", "hi")
		if err == nil || !strings.Contains(err.Error(), "invalid receiver DID") {
			t.Errorf("expected invalid receiver DID error, got %v", err)
		}
	})

	t.Run("receiver_not_found", func(t *testing.T) {
		err := m.SendMessage("did:web:alice", "did:web:unknown", "hi")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected not found error, got %v", err)
		}
	})

	t.Run("payload_too_long", func(t *testing.T) {
		longPayload := strings.Repeat("x", maxPayloadLength+1)
		err := m.SendMessage("did:web:bob", "did:web:alice", longPayload)
		if err == nil || !strings.Contains(err.Error(), "payload too long") {
			t.Errorf("expected payload too long error, got %v", err)
		}
	})

	t.Run("resolve_endpoint", func(t *testing.T) {
		ep, err := m.ResolveDIDEndpoint("did:web:alice")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ep != "https://alice.example.com" {
			t.Errorf("unexpected endpoint: %s", ep)
		}
	})

	t.Run("resolve_missing", func(t *testing.T) {
		_, err := m.ResolveDIDEndpoint("did:web:unknown")
		if err == nil {
			t.Error("expected error for missing DID")
		}
	})
}

// ============================================================
// ReActEngine
// ============================================================

func TestReActEngine_Run_SuccessImmediate(t *testing.T) {
	mem := NewLayeredMemory()
	engine := NewReActEngine(5, mem)

	thinkFn := func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
		return "I know the answer", "noop", "final answer", true, nil
	}
	actFn := func(_ context.Context, _, _ string) (string, error) {
		t.Error("actFn should not be called when finished=true")
		return "", nil
	}

	result, err := engine.Run(context.Background(), "what is 1+1", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", result.Iterations)
	}
	if result.FinalAnswer != "final answer" {
		t.Errorf("expected 'final answer', got %s", result.FinalAnswer)
	}
	if len(result.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].Observation != "task completed" {
		t.Errorf("unexpected observation: %s", result.Steps[0].Observation)
	}
	// Should have stored in long-term memory
	stats := mem.GetStats()
	if stats["long_term"] != 1 {
		t.Errorf("expected 1 long-term memory entry, got %d", stats["long_term"])
	}
}

func TestReActEngine_Run_MultiStepSuccess(t *testing.T) {
	engine := NewReActEngine(5, nil) // nil memory -> auto-created
	iteration := 0
	thinkFn := func(_ context.Context, _ int, ctx string) (string, string, string, bool, error) {
		iteration++
		if iteration >= 2 {
			return "done", "noop", "final", true, nil
		}
		return "need to search", "search", "query", false, nil
	}
	actFn := func(_ context.Context, action, input string) (string, error) {
		return "search result for " + input, nil
	}

	result, err := engine.Run(context.Background(), "research task", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Iterations != 2 {
		t.Errorf("expected 2 iterations, got %d", result.Iterations)
	}
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Observation != "search result for query" {
		t.Errorf("unexpected observation: %s", result.Steps[0].Observation)
	}
	// History should also have 2 steps
	history := engine.GetHistory()
	if len(history) != 1 {
		// Only the non-finished step goes to thoughtHistory (finished step is appended to result.Steps but not thoughtHistory)
		t.Errorf("expected 1 step in history (only non-finished), got %d", len(history))
	}
}

func TestReActEngine_Run_MaxIterations(t *testing.T) {
	engine := NewReActEngine(3, nil)
	thinkFn := func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
		return "thinking", "search", "q", false, nil
	}
	actFn := func(_ context.Context, _, _ string) (string, error) {
		return "obs", nil
	}

	result, err := engine.Run(context.Background(), "task", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Error("expected failure due to max iterations")
	}
	if result.Iterations != 3 {
		t.Errorf("expected 3 iterations, got %d", result.Iterations)
	}
	if !strings.Contains(result.FinalAnswer, "max iterations") {
		t.Errorf("unexpected final answer: %s", result.FinalAnswer)
	}
	if len(result.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(result.Steps))
	}
}

func TestReActEngine_Run_ThinkError(t *testing.T) {
	engine := NewReActEngine(5, nil)
	thinkFn := func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
		return "", "", "", false, errors.New("think failed")
	}
	actFn := func(_ context.Context, _, _ string) (string, error) {
		t.Error("actFn should not be called on think error")
		return "", nil
	}

	result, err := engine.Run(context.Background(), "task", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error from Run (error in result), got %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if result.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", result.Iterations)
	}
	if !strings.Contains(result.FinalAnswer, "error at iteration 1") {
		t.Errorf("unexpected final answer: %s", result.FinalAnswer)
	}
}

func TestReActEngine_Run_ActError(t *testing.T) {
	engine := NewReActEngine(5, nil)
	thinkFn := func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
		return "thought", "action", "input", false, nil
	}
	actFn := func(_ context.Context, _, _ string) (string, error) {
		return "", errors.New("act failed")
	}

	result, err := engine.Run(context.Background(), "task", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error from Run, got %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if !strings.Contains(result.FinalAnswer, "action error") {
		t.Errorf("unexpected final answer: %s", result.FinalAnswer)
	}
}

func TestReActEngine_Run_ContextCancelled(t *testing.T) {
	engine := NewReActEngine(5, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before running

	thinkFn := func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
		t.Error("thinkFn should not be called when ctx already cancelled")
		return "", "", "", false, nil
	}
	actFn := func(_ context.Context, _, _ string) (string, error) { return "", nil }

	result, err := engine.Run(ctx, "task", thinkFn, actFn)
	if err == nil {
		t.Error("expected ctx.Err() returned")
	}
	if result.Success {
		t.Error("expected failure")
	}
	if result.FinalAnswer != "context cancelled" {
		t.Errorf("unexpected final answer: %s", result.FinalAnswer)
	}
}

func TestReActEngine_Run_InvalidArgs(t *testing.T) {
	engine := NewReActEngine(5, nil)

	t.Run("nil_thinkFn", func(t *testing.T) {
		_, err := engine.Run(context.Background(), "task", nil, func(_ context.Context, _, _ string) (string, error) { return "", nil })
		if err == nil || !strings.Contains(err.Error(), "think function") {
			t.Errorf("expected think function error, got %v", err)
		}
	})

	t.Run("nil_actFn", func(t *testing.T) {
		_, err := engine.Run(context.Background(), "task",
			func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
				return "", "", "", false, nil
			}, nil)
		if err == nil || !strings.Contains(err.Error(), "act function") {
			t.Errorf("expected act function error, got %v", err)
		}
	})

	t.Run("empty_task", func(t *testing.T) {
		_, err := engine.Run(context.Background(), "",
			func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
				return "", "", "", false, nil
			},
			func(_ context.Context, _, _ string) (string, error) { return "", nil })
		if err == nil || !strings.Contains(err.Error(), "task is required") {
			t.Errorf("expected task required error, got %v", err)
		}
	})

	t.Run("task_too_long", func(t *testing.T) {
		longTask := strings.Repeat("a", maxPromptLength+1)
		_, err := engine.Run(context.Background(), longTask,
			func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
				return "", "", "", false, nil
			},
			func(_ context.Context, _, _ string) (string, error) { return "", nil })
		if err == nil || !strings.Contains(err.Error(), "task too long") {
			t.Errorf("expected task too long error, got %v", err)
		}
	})
}

func TestReActEngine_Run_ObservationTruncation(t *testing.T) {
	engine := NewReActEngine(5, nil)
	// First iteration produces an oversized observation; second iteration finishes.
	thinkFn := func(_ context.Context, iter int, _ string) (string, string, string, bool, error) {
		if iter == 1 {
			return "thought", "action", "input", false, nil
		}
		return "done", "noop", "final", true, nil
	}
	longObs := strings.Repeat("x", maxObservationLength+100)
	actFn := func(_ context.Context, _, _ string) (string, error) {
		return longObs, nil
	}

	result, err := engine.Run(context.Background(), "task", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	obs := result.Steps[0].Observation
	if !strings.HasSuffix(obs, "... (truncated)") {
		t.Errorf("expected truncation suffix, got: ...%s", obs[len(obs)-30:])
	}
	if len(obs) > maxObservationLength+50 { // allow room for the truncation suffix
		t.Errorf("observation not truncated: len=%d", len(obs))
	}
}

func TestReActEngine_Run_ObservationTruncation_RuneBoundary(t *testing.T) {
	engine := NewReActEngine(5, nil)
	thinkFn := func(_ context.Context, iter int, _ string) (string, string, string, bool, error) {
		if iter == 1 {
			return "thought", "action", "input", false, nil
		}
		return "done", "noop", "final", true, nil
	}
	// Build observation full of multi-byte UTF-8 chars (3 bytes each).
	// Truncation must respect rune boundaries to avoid producing invalid UTF-8.
	const char = "世" // 3 bytes
	repeats := maxObservationLength/3 + 10
	longObs := strings.Repeat(char, repeats)
	actFn := func(_ context.Context, _, _ string) (string, error) {
		return longObs, nil
	}

	result, err := engine.Run(context.Background(), "task", thinkFn, actFn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	obs := result.Steps[0].Observation
	if !strings.HasSuffix(obs, "... (truncated)") {
		t.Errorf("expected truncation suffix")
	}
}

func TestReActEngine_Run_LongTermMemoryAcrossRuns(t *testing.T) {
	engine := NewReActEngine(5, nil)

	// First run: task text "remember this" is part of the final answer, so recall will match.
	task := "remember this"
	thinkFn1 := func(_ context.Context, _ int, _ string) (string, string, string, bool, error) {
		return "I know", "noop", "ok remember this final", true, nil
	}
	actFn := func(_ context.Context, _, _ string) (string, error) { return "", nil }

	if _, err := engine.Run(context.Background(), task, thinkFn1, actFn); err != nil {
		t.Fatal(err)
	}

	// Second run: thinkFn captures the context, which should include "[Related memory:".
	var capturedCtx string
	thinkFn2 := func(_ context.Context, _ int, ctx string) (string, string, string, bool, error) {
		capturedCtx = ctx
		return "done", "noop", "ok remember this final", true, nil
	}
	if _, err := engine.Run(context.Background(), task, thinkFn2, actFn); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedCtx, "[Related memory:") {
		t.Errorf("expected context to include related memory, got: %s", capturedCtx)
	}
	if !strings.Contains(capturedCtx, "remember this") {
		t.Errorf("expected context to include 'remember this', got: %s", capturedCtx)
	}
}

func TestReActEngine_GetHistory_ReturnsCopy(t *testing.T) {
	engine := NewReActEngine(5, nil)
	// First iteration produces a step (added to history); second iteration finishes.
	thinkFn := func(_ context.Context, iter int, _ string) (string, string, string, bool, error) {
		if iter == 1 {
			return "thought", "action", "input", false, nil
		}
		return "done", "noop", "final", true, nil
	}
	actFn := func(_ context.Context, _, _ string) (string, error) { return "obs", nil }

	if _, err := engine.Run(context.Background(), "task", thinkFn, actFn); err != nil {
		t.Fatal(err)
	}

	h1 := engine.GetHistory()
	if len(h1) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(h1))
	}
	// Mutate the returned slice - should not affect internal state
	h1[0].Thought = "mutated"
	h2 := engine.GetHistory()
	if h2[0].Thought == "mutated" {
		t.Error("GetHistory should return a copy, but mutation affected internal state")
	}
}

func TestNewReActEngine_Defaults(t *testing.T) {
	t.Run("maxIterations_zero_defaults_to_10", func(t *testing.T) {
		e := NewReActEngine(0, nil)
		if e.maxIterations != 10 {
			t.Errorf("expected default 10, got %d", e.maxIterations)
		}
	})
	t.Run("maxIterations_negative_defaults_to_10", func(t *testing.T) {
		e := NewReActEngine(-5, nil)
		if e.maxIterations != 10 {
			t.Errorf("expected default 10, got %d", e.maxIterations)
		}
	})
	t.Run("maxIterations_too_high_defaults_to_10", func(t *testing.T) {
		e := NewReActEngine(100, nil)
		if e.maxIterations != 10 {
			t.Errorf("expected default 10, got %d", e.maxIterations)
		}
	})
	t.Run("nil_memory_auto_created", func(t *testing.T) {
		e := NewReActEngine(5, nil)
		if e.memory == nil {
			t.Error("expected auto-created memory")
		}
	})
	t.Run("valid_args_preserved", func(t *testing.T) {
		mem := NewLayeredMemory()
		e := NewReActEngine(7, mem)
		if e.maxIterations != 7 {
			t.Errorf("expected 7, got %d", e.maxIterations)
		}
		if e.memory != mem {
			t.Error("expected memory to be preserved")
		}
	})
}

// ============================================================
// LayeredMemory
// ============================================================

func TestLayeredMemory_StoreAndRecall(t *testing.T) {
	m := NewLayeredMemory()

	// Store at each level
	if err := m.Store("k1", "short-term content here", MemoryLevelShortTerm); err != nil {
		t.Fatalf("short-term store failed: %v", err)
	}
	if err := m.Store("k2", "working content here", MemoryLevelWorking); err != nil {
		t.Fatalf("working store failed: %v", err)
	}
	if err := m.Store("k3", "long-term content here", MemoryLevelLongTerm); err != nil {
		t.Fatalf("long-term store failed: %v", err)
	}

	stats := m.GetStats()
	if stats["short_term"] != 1 || stats["working"] != 1 || stats["long_term"] != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}

	// Recall by matching content
	results := m.Recall("content here")
	// All three contain "content here"
	if len(results) < 3 {
		t.Errorf("expected at least 3 results, got %d: %v", len(results), results)
	}

	// Verify prefix ordering: long-term first, then working, then short-term
	if len(results) >= 1 && !strings.HasPrefix(results[0], "[long-term]") {
		t.Errorf("expected long-term first, got %s", results[0])
	}
}

func TestLayeredMemory_StoreValidation(t *testing.T) {
	m := NewLayeredMemory()

	t.Run("empty_key", func(t *testing.T) {
		if err := m.Store("", "content", MemoryLevelShortTerm); err == nil {
			t.Error("expected error for empty key")
		}
	})

	t.Run("key_too_long", func(t *testing.T) {
		longKey := strings.Repeat("k", 257)
		if err := m.Store(longKey, "content", MemoryLevelShortTerm); err == nil {
			t.Error("expected error for too long key")
		}
	})

	t.Run("empty_content", func(t *testing.T) {
		if err := m.Store("k", "", MemoryLevelShortTerm); err == nil {
			t.Error("expected error for empty content")
		}
	})

	t.Run("invalid_level", func(t *testing.T) {
		if err := m.Store("k", "content", MemoryLevel("invalid")); err == nil {
			t.Error("expected error for invalid level")
		}
	})
}

func TestLayeredMemory_LongContentTruncation(t *testing.T) {
	m := NewLayeredMemory()
	longContent := strings.Repeat("a", maxMemoryContentLength+100)
	if err := m.Store("k", longContent, MemoryLevelShortTerm); err != nil {
		t.Fatal(err)
	}
	// Recall should return truncated content
	results := m.Recall("a")
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !strings.HasSuffix(results[0], "... (truncated)") {
		t.Errorf("expected truncation suffix, got: ...%s", results[0][len(results[0])-30:])
	}
}

func TestLayeredMemory_RecallQueryTruncation(t *testing.T) {
	m := NewLayeredMemory()
	if err := m.Store("k", "abcdefghij", MemoryLevelLongTerm); err != nil {
		t.Fatal(err)
	}
	// Very long query - should not panic, gets truncated to maxRecallQueryLength
	longQuery := "abcdefghij" + strings.Repeat("x", maxRecallQueryLength+100)
	// After truncation, query becomes "abcdefghij" + lots of x's, which is longer than content "abcdefghij"
	// so the contains check fails -> 0 long-term results
	results := m.Recall(longQuery)
	// The long query likely won't match short content; just verify no panic and slice returned.
	_ = results
}

func TestLayeredMemory_Clear(t *testing.T) {
	m := NewLayeredMemory()
	_ = m.Store("k1", "short content", MemoryLevelShortTerm)
	_ = m.Store("k2", "working content", MemoryLevelWorking)
	_ = m.Store("k3", "long content", MemoryLevelLongTerm)

	m.Clear(MemoryLevelShortTerm)
	stats := m.GetStats()
	if stats["short_term"] != 0 {
		t.Errorf("expected 0 short_term, got %d", stats["short_term"])
	}
	if stats["working"] != 1 || stats["long_term"] != 1 {
		t.Errorf("unexpected clearing of other levels: %+v", stats)
	}

	m.Clear(MemoryLevelWorking)
	m.Clear(MemoryLevelLongTerm)
	stats = m.GetStats()
	if stats["working"] != 0 || stats["long_term"] != 0 {
		t.Errorf("expected all cleared, got %+v", stats)
	}
}

func TestLayeredMemory_LongTermPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// First memory instance stores to disk
	m1 := NewLayeredMemory()
	if err := m1.SetStorePath(tmpDir); err != nil {
		t.Fatal(err)
	}
	content := "persistent long-term content for recall"
	if err := m1.Store("key1", content, MemoryLevelLongTerm); err != nil {
		t.Fatal(err)
	}

	// Verify a .json file was created
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in store dir, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Errorf("expected .json file, got %s", entries[0].Name())
	}

	// Second memory instance loads from disk
	m2 := NewLayeredMemory()
	if err := m2.SetStorePath(tmpDir); err != nil {
		t.Fatal(err)
	}
	stats := m2.GetStats()
	if stats["long_term"] != 1 {
		t.Errorf("expected 1 long-term entry after reload, got %d", stats["long_term"])
	}

	// Recall should find the persisted content
	results := m2.Recall("persistent")
	if len(results) == 0 {
		t.Fatal("expected recall to find persisted content")
	}
	if !strings.Contains(results[0], content) {
		t.Errorf("expected to find content, got: %s", results[0])
	}

	// Clear should remove the file
	m2.Clear(MemoryLevelLongTerm)
	entries, err = os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files after clear, got %d", len(entries))
	}
}

func TestLayeredMemory_Eviction(t *testing.T) {
	m := NewLayeredMemory()
	// Lower maxEntries for faster testing (white-box)
	m.mu.Lock()
	m.maxEntries = 3
	m.mu.Unlock()

	// Use Working level: Recall updates LastAccess for working/long-term entries
	// (but NOT for short-term entries — short-term only sorts by CreatedAt).
	for i := 0; i < 3; i++ {
		if err := m.Store(fmt.Sprintf("k%d", i), fmt.Sprintf("content-%d", i), MemoryLevelWorking); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	// Touch k0 to make it most-recently-accessed (so k1 is oldest)
	_ = m.Recall("content-0")
	time.Sleep(time.Millisecond)
	// Touch k2
	_ = m.Recall("content-2")
	// Now k1 should be oldest (never re-touched)

	// Adding a 4th should evict the oldest
	if err := m.Store("k3", "new content", MemoryLevelWorking); err != nil {
		t.Fatal(err)
	}
	stats := m.GetStats()
	if stats["working"] != 3 {
		t.Errorf("expected 3 entries (cap), got %d", stats["working"])
	}
	// k1 should be evicted (oldest)
	results := m.Recall("content-1")
	for _, r := range results {
		if strings.Contains(r, "content-1") {
			t.Errorf("expected content-1 to be evicted, but found: %s", r)
		}
	}
}

func TestLayeredMemory_SetStorePathInvalid(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{"relative", "relative/path", "must be absolute"},
		{"dotdot", "/tmp/../etc", "cannot contain '..'"},
		{"root", "/", "cannot be root directory"},
		{"too_long", "/" + strings.Repeat("a", 600), "path too long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewLayeredMemory()
			err := m.SetStorePath(tt.path)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestLayeredMemory_SetStorePath_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewLayeredMemory()
	if err := m.SetStorePath(tmpDir); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// store long-term memory, should persist to disk
	if err := m.Store("k", "content on disk", MemoryLevelLongTerm); err != nil {
		t.Fatal(err)
	}
	// Verify file exists
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
	// Verify file permissions are 0600
	info, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected file perm 0600, got %v", perm)
	}
}

func TestLayeredMemory_LongTermEviction_DeletesFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewLayeredMemory()
	if err := m.SetStorePath(tmpDir); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.maxEntries = 2
	m.mu.Unlock()

	// Store 3 long-term entries -> 1 should be evicted, and its file deleted
	for i := 0; i < 3; i++ {
		if err := m.Store(fmt.Sprintf("k%d", i), fmt.Sprintf("content-%d", i), MemoryLevelLongTerm); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // ensure distinct LastAccess
	}
	stats := m.GetStats()
	if stats["long_term"] != 2 {
		t.Errorf("expected 2 entries after eviction, got %d", stats["long_term"])
	}
	// Verify only 2 files remain on disk (evicted one should be deleted)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 files on disk after eviction, got %d", len(entries))
	}
}

func TestLayeredMemory_Recall_Limits(t *testing.T) {
	m := NewLayeredMemory()
	// Add more than maxRecallResults matching entries
	for i := 0; i < maxRecallResults+5; i++ {
		if err := m.Store(fmt.Sprintf("k%d", i), "common-content", MemoryLevelLongTerm); err != nil {
			t.Fatal(err)
		}
	}
	results := m.Recall("common-content")
	if len(results) > maxRecallResults {
		t.Errorf("expected at most %d results, got %d", maxRecallResults, len(results))
	}
}

// ============================================================
// Helper functions
// ============================================================

func TestToJSON(t *testing.T) {
	t.Run("valid_struct", func(t *testing.T) {
		s, err := ToJSON(struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{Name: "alice", Age: 30})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s, `"name": "alice"`) {
			t.Errorf("expected name field, got %s", s)
		}
		if !strings.Contains(s, "  ") { // 2-space indent
			t.Errorf("expected indented output, got %s", s)
		}
	})

	t.Run("invalid_value", func(t *testing.T) {
		_, err := ToJSON(make(chan int))
		if err == nil {
			t.Error("expected error for unmarshalable value")
		}
	})
}

func TestGenerateSecureID(t *testing.T) {
	id1 := GenerateSecureID()
	if id1 == "" {
		t.Error("expected non-empty ID")
	}
	id2 := GenerateSecureID()
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	// Default base64 URLEncoding of 16 bytes -> 24 chars
	if len(id1) != 24 {
		t.Errorf("expected 24-char ID, got %d chars: %s", len(id1), id1)
	}
}

func TestSanitizeKey(t *testing.T) {
	k1 := sanitizeKey("hello")
	k2 := sanitizeKey("hello")
	if k1 != k2 {
		t.Error("expected deterministic output")
	}
	k3 := sanitizeKey("world")
	if k1 == k3 {
		t.Error("expected different inputs to produce different outputs")
	}
	// SHA-256 first 16 bytes -> 32 hex chars
	if len(k1) != 32 {
		t.Errorf("expected 32-char hex, got %d: %s", len(k1), k1)
	}
}

func TestHashMemoryKey(t *testing.T) {
	k1 := hashMemoryKey("task1")
	k2 := hashMemoryKey("task1")
	if k1 != k2 {
		t.Error("expected deterministic output")
	}
	k3 := hashMemoryKey("task2")
	if k1 == k3 {
		t.Error("expected different inputs to produce different outputs")
	}
	if len(k1) != 32 {
		t.Errorf("expected 32-char hex, got %d: %s", len(k1), k1)
	}
}

func TestRecommendLocalModel(t *testing.T) {
	tests := []struct {
		name string
		cap  DeviceCapability
		want string
	}{
		{"gpu_16gb", DeviceCapability{HasGPU: true, MemoryGB: 16}, "llama3:70b"},
		{"gpu_8gb", DeviceCapability{HasGPU: true, MemoryGB: 8}, "llama3:8b"},
		{"gpu_4gb_low_mem", DeviceCapability{HasGPU: true, MemoryGB: 4}, "llama3:2b-q4"},
		{"no_gpu_4gb", DeviceCapability{HasGPU: false, MemoryGB: 4}, "llama3:2b-q4"},
		{"mobile", DeviceCapability{HasGPU: false, MemoryGB: 16, IsMobile: true}, "llama3:2b-q4"},
		{"no_gpu_8gb", DeviceCapability{HasGPU: false, MemoryGB: 8}, "llama3:7b"},
		{"no_gpu_16gb", DeviceCapability{HasGPU: false, MemoryGB: 16}, "llama3:7b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecommendLocalModel(tt.cap)
			if got != tt.want {
				t.Errorf("RecommendLocalModel(%+v) = %s, want %s", tt.cap, got, tt.want)
			}
		})
	}
}

func TestDetectPlatformType_Valid(t *testing.T) {
	pt := DetectPlatformType()
	valid := map[string]bool{
		"harmony": true,
		"android": true,
		"ios":     true,
		"desktop": true,
	}
	if !valid[pt] {
		t.Errorf("expected one of harmony/android/ios/desktop, got %s", pt)
	}
}

func TestDetectCapability(t *testing.T) {
	cap := DetectCapability()
	if cap.CPUCount <= 0 {
		t.Errorf("expected positive CPU count, got %d", cap.CPUCount)
	}
	if cap.MemoryGB <= 0 {
		t.Errorf("expected positive memory, got %f", cap.MemoryGB)
	}
	// Platform should be a non-empty string
	if cap.Platform == "" {
		t.Error("expected non-empty platform")
	}
}

func TestMinHelper(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("min(1,2) should be 1")
	}
	if min(5, 5) != 5 {
		t.Error("min(5,5) should be 5")
	}
	if min(-1, 0) != -1 {
		t.Error("min(-1,0) should be -1")
	}
}

// ============================================================
// Concurrency safety
// ============================================================

func TestLayeredMemory_ConcurrentAccess(t *testing.T) {
	m := NewLayeredMemory()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = m.Store(fmt.Sprintf("k%d", idx), fmt.Sprintf("content-%d", idx), MemoryLevelShortTerm)
			_ = m.Recall(fmt.Sprintf("content-%d", idx))
			_ = m.GetStats()
		}(i)
	}
	wg.Wait()
}

func TestEdgeRouter_ConcurrentRouteAndStats(t *testing.T) {
	cfg := validConfig()
	cfg.PrivacyLevel = PrivacyStrict
	r, err := NewEdgeRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.localModel = &mockLocalModel{available: true}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			task := TaskRequest{ID: fmt.Sprintf("t%d", idx), Prompt: "hello"}
			_, _ = r.Execute(context.Background(), task)
		}(i)
		go func() {
			defer wg.Done()
			_ = r.GetStats()
		}()
	}
	wg.Wait()
}

// TestEdgeRouterHelpers_PathSafety ensures path-safe helpers produce expected file paths.
func TestEdgeRouterHelpers_PathSafety(t *testing.T) {
	// sanitizeKey should produce safe filenames (no path separators)
	k := sanitizeKey("../../../etc/passwd")
	if strings.Contains(k, "/") || strings.Contains(k, "..") {
		t.Errorf("sanitizeKey produced unsafe filename: %s", k)
	}
	// Verify the sanitized key actually works as a filename
	tmpDir := t.TempDir()
	safe := filepath.Join(tmpDir, k+".json")
	if err := os.WriteFile(safe, []byte("{}"), 0600); err != nil {
		t.Fatalf("could not write file with sanitized key: %v", err)
	}
}
