// Package edge provides edge-side AI agent capabilities for llm-box.
// It enables local-first execution with cloud enhancement, similar to
// Apple Intelligence's Private Cloud Compute approach.
package edge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Edge Router - Local-first AI task routing
// =============================================================================

// EdgeRouter routes tasks between local and cloud models
type EdgeRouter struct {
	config      EdgeConfig
	localModel  LocalModel
	cloudModels map[string]CloudModel
	stats       RouterStats
	mu          sync.RWMutex
}

// EdgeConfig configures the edge router
type EdgeConfig struct {
	// LocalModelEndpoint is the local model endpoint (e.g., http://localhost:11434)
	LocalModelEndpoint string `json:"local_model_endpoint"`

	// LocalModelName is the default local model (e.g., llama3, qwen2)
	LocalModelName string `json:"local_model_name"`

	// CloudModels maps provider names to endpoints
	CloudModels map[string]CloudModelConfig `json:"cloud_models"`

	// PrivacyLevel controls data routing
	// "strict" - never send to cloud
	// "balanced" - send non-sensitive to cloud
	// "permissive" - send all to best model
	PrivacyLevel PrivacyLevel `json:"privacy_level"`

	// LocalThreshold is the complexity threshold for local execution (0-10)
	LocalThreshold int `json:"local_threshold"`

	// EnableFallback enables fallback to cloud if local fails
	EnableFallback bool `json:"enable_fallback"`

	// MaxLatency is the maximum acceptable latency (ms)
	MaxLatency int `json:"max_latency"`
}

// PrivacyLevel defines privacy level for task routing
type PrivacyLevel string

const (
	PrivacyStrict     PrivacyLevel = "strict"     // Never send to cloud
	PrivacyBalanced   PrivacyLevel = "balanced"   // Send non-sensitive to cloud
	PrivacyPermissive PrivacyLevel = "permissive" // Send all to best model
)

// CloudModelConfig configures a cloud model
type CloudModelConfig struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Priority int    `json:"priority"` // Higher = preferred
}

// LocalModel represents a local model (e.g., Ollama)
type LocalModel interface {
	IsAvailable() bool
	GetModelName() string
	Execute(ctx context.Context, prompt string, opts map[string]string) (string, error)
	GetMetrics() LocalMetrics
}

// CloudModel represents a cloud model (e.g., OpenAI, DeepSeek)
type CloudModel interface {
	GetProviderName() string
	Execute(ctx context.Context, prompt string, opts map[string]string) (string, error)
	GetMetrics() CloudMetrics
}

// LocalMetrics contains local model metrics
type LocalMetrics struct {
	Available    bool    `json:"available"`
	ModelName    string  `json:"model_name"`
	MemoryUsedMB float64 `json:"memory_used_mb"`
	GPUUsed      bool    `json:"gpu_used"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
}

// CloudMetrics contains cloud model metrics
type CloudMetrics struct {
	Provider     string `json:"provider"`
	Available    bool   `json:"available"`
	AvgLatencyMs int64  `json:"avg_latency_ms"`
	TotalCalls   int    `json:"total_calls"`
}

// RouterStats contains routing statistics
type RouterStats struct {
	LocalCalls   int     `json:"local_calls"`
	CloudCalls   int     `json:"cloud_calls"`
	LocalSuccess int     `json:"local_success"`
	CloudSuccess int     `json:"cloud_success"`
	AvgLocalLat  int64   `json:"avg_local_latency_ms"`
	AvgCloudLat  int64   `json:"avg_cloud_latency_ms"`
	TotalTokens  int     `json:"total_tokens"`
	SavingsPct   float64 `json:"savings_pct"` // % saved by local
}

// NewEdgeRouter creates a new edge router
func NewEdgeRouter(config EdgeConfig) *EdgeRouter {
	return &EdgeRouter{
		config:      config,
		cloudModels: make(map[string]CloudModel),
		stats:       RouterStats{},
	}
}

// Route decides where to execute a task
func (r *EdgeRouter) Route(ctx context.Context, task TaskRequest) RouteDecision {
	r.mu.RLock()
	defer r.mu.RUnlock()

	decision := RouteDecision{
		TaskID:    task.ID,
		Timestamp: time.Now(),
	}

	// Check privacy level
	if r.config.PrivacyLevel == PrivacyStrict {
		decision.Target = TargetLocal
		decision.Reason = "privacy_strict_mode"
		return decision
	}

	// Check if task contains sensitive data
	if task.ContainsSensitiveData && r.config.PrivacyLevel == PrivacyBalanced {
		decision.Target = TargetLocal
		decision.Reason = "contains_sensitive_data"
		return decision
	}

	// Analyze task complexity
	complexity := r.analyzeComplexity(task)

	// Route based on complexity and threshold
	if complexity <= r.config.LocalThreshold {
		// Check if local model is available
		if r.localModel != nil && r.localModel.IsAvailable() {
			decision.Target = TargetLocal
			decision.Reason = fmt.Sprintf("low_complexity_%d", complexity)
			return decision
		}
	}

	// Route to cloud
	if r.config.PrivacyLevel == PrivacyPermissive || !task.ContainsSensitiveData {
		decision.Target = TargetCloud
		decision.Reason = fmt.Sprintf("high_complexity_%d", complexity)
		decision.Provider = r.selectBestCloudProvider()
		return decision
	}

	// Fallback to local
	decision.Target = TargetLocal
	decision.Reason = "privacy_balanced_fallback"
	return decision
}

// Execute executes a task on the decided target
func (r *EdgeRouter) Execute(ctx context.Context, task TaskRequest) (TaskResult, error) {
	decision := r.Route(ctx, task)

	result := TaskResult{
		TaskID:  task.ID,
		Success: false,
	}

	var output string
	var err error
	startTime := time.Now()

	switch decision.Target {
	case TargetLocal:
		output, err = r.executeLocal(ctx, task)
		r.mu.Lock()
		r.stats.LocalCalls++
		if err == nil {
			r.stats.LocalSuccess++
			r.stats.AvgLocalLat = (r.stats.AvgLocalLat + time.Since(startTime).Milliseconds()) / 2
		}
		r.mu.Unlock()

	case TargetCloud:
		output, err = r.executeCloud(ctx, task, decision.Provider)
		r.mu.Lock()
		r.stats.CloudCalls++
		if err == nil {
			r.stats.CloudSuccess++
			r.stats.AvgCloudLat = (r.stats.AvgCloudLat + time.Since(startTime).Milliseconds()) / 2
		}
		r.mu.Unlock()
	}

	if err != nil && r.config.EnableFallback && decision.Target == TargetLocal {
		// Fallback to cloud
		output, err = r.executeCloud(ctx, task, r.selectBestCloudProvider())
		result.FallbackUsed = true
	}

	result.Output = output
	result.Error = err
	result.LatencyMs = time.Since(startTime).Milliseconds()
	result.Decision = decision
	result.Success = err == nil

	return result, err
}

// executeLocal executes on local model
func (r *EdgeRouter) executeLocal(ctx context.Context, task TaskRequest) (string, error) {
	if r.localModel == nil || !r.localModel.IsAvailable() {
		return "", fmt.Errorf("local model not available")
	}
	return r.localModel.Execute(ctx, task.Prompt, task.Options)
}

// executeCloud executes on cloud model
func (r *EdgeRouter) executeCloud(ctx context.Context, task TaskRequest, provider string) (string, error) {
	model, ok := r.cloudModels[provider]
	if !ok {
		return "", fmt.Errorf("cloud model %s not configured", provider)
	}
	return model.Execute(ctx, task.Prompt, task.Options)
}

// analyzeComplexity analyzes task complexity (0-10)
func (r *EdgeRouter) analyzeComplexity(task TaskRequest) int {
	complexity := 5 // Default medium complexity

	// Analyze prompt length
	promptLen := len(task.Prompt)
	if promptLen > 1000 {
		complexity += 2
	} else if promptLen > 500 {
		complexity += 1
	}

	// Analyze keywords
	complexKeywords := []string{
		"分析", "比较", "评估", "研究", "深度",
		"analyze", "compare", "evaluate", "research", "complex",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(strings.ToLower(task.Prompt), kw) {
			complexity += 2
			break
		}
	}

	simpleKeywords := []string{
		"总结", "翻译", "转换", "提取",
		"summarize", "translate", "convert", "extract",
	}
	for _, kw := range simpleKeywords {
		if strings.Contains(strings.ToLower(task.Prompt), kw) {
			complexity -= 2
			break
		}
	}

	// Cap at 0-10
	if complexity < 0 {
		complexity = 0
	}
	if complexity > 10 {
		complexity = 10
	}

	return complexity
}

// selectBestCloudProvider selects the best cloud provider
func (r *EdgeRouter) selectBestCloudProvider() string {
	bestProvider := ""
	bestPriority := -1

	for name, config := range r.config.CloudModels {
		if config.Priority > bestPriority {
			bestPriority = config.Priority
			bestProvider = name
		}
	}

	if bestProvider == "" {
		return "openai" // Default
	}
	return bestProvider
}

// GetStats returns routing statistics
func (r *EdgeRouter) GetStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := r.stats
	total := stats.LocalCalls + stats.CloudCalls
	if total > 0 {
		stats.SavingsPct = float64(stats.LocalCalls) / float64(total) * 100
	}

	return stats
}

// =============================================================================
// Task Types
// =============================================================================

// TaskRequest represents a task to be routed
type TaskRequest struct {
	ID                    string            `json:"id"`
	Prompt                string            `json:"prompt"`
	Options               map[string]string `json:"options"`
	ContainsSensitiveData bool              `json:"contains_sensitive_data"`
	Metadata              map[string]string `json:"metadata"`
}

// TaskResult represents the result of a task
type TaskResult struct {
	TaskID       string        `json:"task_id"`
	Output       string        `json:"output"`
	Success      bool          `json:"success"`
	Error        error         `json:"error,omitempty"`
	LatencyMs    int64         `json:"latency_ms"`
	FallbackUsed bool          `json:"fallback_used"`
	Decision     RouteDecision `json:"decision"`
}

// RouteDecision represents a routing decision
type RouteDecision struct {
	TaskID    string    `json:"task_id"`
	Target    Target    `json:"target"`
	Provider  string    `json:"provider,omitempty"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Target represents where to execute
type Target string

const (
	TargetLocal Target = "local"
	TargetCloud Target = "cloud"
)

// =============================================================================
// Ollama Local Model Implementation
// =============================================================================

// OllamaModel is a local Ollama model
type OllamaModel struct {
	endpoint  string
	modelName string
	available bool
	metrics   LocalMetrics
}

// NewOllamaModel creates a new Ollama model
func NewOllamaModel(endpoint, modelName string) *OllamaModel {
	return &OllamaModel{
		endpoint:  endpoint,
		modelName: modelName,
	}
}

// IsAvailable checks if Ollama is available
func (o *OllamaModel) IsAvailable() bool {
	// Check if endpoint is reachable (simplified)
	return o.available
}

// GetModelName returns the model name
func (o *OllamaModel) GetModelName() string {
	return o.modelName
}

// Execute executes a prompt
func (o *OllamaModel) Execute(ctx context.Context, prompt string, opts map[string]string) (string, error) {
	// Simplified: actual implementation would call Ollama API
	return fmt.Sprintf("Local model %s response for: %s", o.modelName, prompt[:min(50, len(prompt))]), nil
}

// GetMetrics returns local metrics
func (o *OllamaModel) GetMetrics() LocalMetrics {
	return o.metrics
}

// =============================================================================
// Device Capability Detection
// =============================================================================

// DeviceCapability represents device capabilities for model selection
type DeviceCapability struct {
	CPUCount   int     `json:"cpu_count"`
	MemoryGB   float64 `json:"memory_gb"`
	GPUModel   string  `json:"gpu_model,omitempty"`
	HasGPU     bool    `json:"has_gpu"`
	StorageGB  float64 `json:"storage_gb"`
	IsMobile   bool    `json:"is_mobile"`
	BatteryPct int     `json:"battery_pct,omitempty"`
	OnWiFi     bool    `json:"on_wifi"`
	OnCellular bool    `json:"on_cellular"`
}

// DetectCapability detects device capabilities
func DetectCapability() DeviceCapability {
	cap := DeviceCapability{
		CPUCount: runtime.NumCPU(),
	}

	// Get memory info
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cap.MemoryGB = float64(m.Sys) / 1024 / 1024 / 1024

	// Check for GPU (simplified)
	if os.Getenv("CUDA_VISIBLE_DEVICES") != "" || os.Getenv("NVIDIA_VISIBLE_DEVICES") != "" {
		cap.HasGPU = true
	}

	// Check if mobile (simplified - would need platform-specific detection)
	cap.IsMobile = os.Getenv("ANDROID_ROOT") != "" || os.Getenv("TERMUX_VERSION") != ""

	return cap
}

// RecommendLocalModel recommends a local model based on device capability
func RecommendLocalModel(cap DeviceCapability) string {
	// High-end device with GPU
	if cap.HasGPU && cap.MemoryGB >= 16 {
		return "llama3:70b"
	}

	// Medium device with GPU
	if cap.HasGPU && cap.MemoryGB >= 8 {
		return "llama3:8b"
	}

	// Low-end device or mobile
	if cap.MemoryGB < 8 || cap.IsMobile {
		return "llama3:2b-q4" // Quantized for efficiency
	}

	// Default
	return "llama3:7b"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// Privacy Analyzer - Detect sensitive data
// =============================================================================

// PrivacyAnalyzer analyzes text for sensitive content
type PrivacyAnalyzer struct {
	sensitivePatterns []string
}

// NewPrivacyAnalyzer creates a privacy analyzer
func NewPrivacyAnalyzer() *PrivacyAnalyzer {
	return &PrivacyAnalyzer{
		sensitivePatterns: []string{
			"密码", "password", "secret", "token",
			"身份证", "id_card", "ssn", "social_security",
			"银行卡", "credit_card", "card_number",
			"私钥", "private_key", "api_key",
			"地址", "address", "location",
			"手机号", "phone", "telephone",
			"邮箱", "email", "mail",
		},
	}
}

// Analyze checks if text contains sensitive data
func (p *PrivacyAnalyzer) Analyze(text string) bool {
	textLower := strings.ToLower(text)
	for _, pattern := range p.sensitivePatterns {
		if strings.Contains(textLower, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for potential patterns (credit card, email, etc.)
	// This is simplified - would use regex in production
	if strings.Contains(text, "@") && strings.Contains(text, ".") {
		return true // Possible email
	}

	return false
}

// =============================================================================
// JSON helpers
// =============================================================================

// ToJSON converts to JSON string
func ToJSON(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
