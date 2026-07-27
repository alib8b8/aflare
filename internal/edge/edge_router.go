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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxPromptLength          = 32768
	maxTaskIDLength          = 100
	maxProviderNameLen       = 50
	maxAgentCapabilities     = 50
	maxAgentMetadataEntries  = 50
	maxAgentMetadataKeyLen   = 64
	maxAgentMetadataValueLen = 512
	maxPayloadLength         = 65536
	maxRegistrySize          = 10000
)

type EdgeRouter struct {
	config      EdgeConfig
	localModel  LocalModel
	cloudModels map[string]CloudModel
	stats       RouterStats
	mu          sync.RWMutex
}

type EdgeConfig struct {
	LocalModelEndpoint string                      `json:"local_model_endpoint"`
	LocalModelName     string                      `json:"local_model_name"`
	CloudModels        map[string]CloudModelConfig `json:"cloud_models"`
	PrivacyLevel       PrivacyLevel                `json:"privacy_level"`
	LocalThreshold     int                         `json:"local_threshold"`
	EnableFallback     bool                        `json:"enable_fallback"`
	MaxLatency         int                         `json:"max_latency"`
}

type PrivacyLevel string

const (
	PrivacyStrict     PrivacyLevel = "strict"
	PrivacyBalanced   PrivacyLevel = "balanced"
	PrivacyPermissive PrivacyLevel = "permissive"
)

type CloudModelConfig struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"-"`
	Model    string `json:"model"`
	Priority int    `json:"priority"`
}

type LocalModel interface {
	IsAvailable() bool
	GetModelName() string
	Execute(ctx context.Context, prompt string, opts map[string]string) (string, error)
	GetMetrics() LocalMetrics
}

type CloudModel interface {
	GetProviderName() string
	Execute(ctx context.Context, prompt string, opts map[string]string) (string, error)
	GetMetrics() CloudMetrics
}

type LocalMetrics struct {
	Available    bool    `json:"available"`
	ModelName    string  `json:"model_name"`
	MemoryUsedMB float64 `json:"memory_used_mb"`
	GPUUsed      bool    `json:"gpu_used"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
}

type CloudMetrics struct {
	Provider     string `json:"provider"`
	Available    bool   `json:"available"`
	AvgLatencyMs int64  `json:"avg_latency_ms"`
	TotalCalls   int    `json:"total_calls"`
}

type RouterStats struct {
	LocalCalls   int     `json:"local_calls"`
	CloudCalls   int     `json:"cloud_calls"`
	LocalSuccess int     `json:"local_success"`
	CloudSuccess int     `json:"cloud_success"`
	AvgLocalLat  int64   `json:"avg_local_latency_ms"`
	AvgCloudLat  int64   `json:"avg_cloud_latency_ms"`
	TotalTokens  int     `json:"total_tokens"`
	SavingsPct   float64 `json:"savings_pct"`
}

func NewEdgeRouter(config EdgeConfig) (*EdgeRouter, error) {
	if err := validateEdgeConfig(config); err != nil {
		return nil, err
	}

	return &EdgeRouter{
		config:      config,
		cloudModels: make(map[string]CloudModel),
		stats:       RouterStats{},
	}, nil
}

func validateEdgeConfig(config EdgeConfig) error {
	if config.LocalThreshold < 0 || config.LocalThreshold > 10 {
		return fmt.Errorf("local_threshold must be between 0 and 10")
	}

	if config.MaxLatency < 0 {
		return fmt.Errorf("max_latency cannot be negative")
	}

	switch config.PrivacyLevel {
	case PrivacyStrict, PrivacyBalanced, PrivacyPermissive:
	default:
		return fmt.Errorf("invalid privacy_level: %s", config.PrivacyLevel)
	}

	for name, cm := range config.CloudModels {
		if len(name) > maxProviderNameLen {
			return fmt.Errorf("cloud model provider name too long: %s", name)
		}
		if cm.Endpoint == "" {
			return fmt.Errorf("cloud model %s requires endpoint", name)
		}
		if cm.Model == "" {
			return fmt.Errorf("cloud model %s requires model", name)
		}
		if err := validateEndpoint(cm.Endpoint); err != nil {
			return fmt.Errorf("invalid endpoint for %s: %w", name, err)
		}
	}

	return nil
}

func validateEndpoint(endpoint string) error {
	if len(endpoint) > 2048 {
		return fmt.Errorf("endpoint too long")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("endpoint must use http or https")
	}

	if u.User != nil {
		return fmt.Errorf("endpoint cannot contain credentials")
	}

	return nil
}

func (r *EdgeRouter) Route(ctx context.Context, task TaskRequest) (RouteDecision, error) {
	if err := validateTaskRequest(task); err != nil {
		return RouteDecision{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	decision := RouteDecision{
		TaskID:    task.ID,
		Timestamp: time.Now(),
	}

	if r.config.PrivacyLevel == PrivacyStrict {
		decision.Target = TargetLocal
		decision.Reason = "privacy_strict_mode"
		return decision, nil
	}

	if task.ContainsSensitiveData && r.config.PrivacyLevel == PrivacyBalanced {
		decision.Target = TargetLocal
		decision.Reason = "contains_sensitive_data"
		return decision, nil
	}

	complexity := r.analyzeComplexity(task)

	if complexity <= r.config.LocalThreshold {
		if r.localModel != nil && r.localModel.IsAvailable() {
			decision.Target = TargetLocal
			decision.Reason = fmt.Sprintf("low_complexity_%d", complexity)
			return decision, nil
		}
	}

	if r.config.PrivacyLevel == PrivacyPermissive || !task.ContainsSensitiveData {
		decision.Target = TargetCloud
		decision.Reason = fmt.Sprintf("high_complexity_%d", complexity)
		decision.Provider = r.selectBestCloudProvider()
		return decision, nil
	}

	decision.Target = TargetLocal
	decision.Reason = "privacy_balanced_fallback"
	return decision, nil
}

func validateTaskRequest(task TaskRequest) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if len(task.ID) > maxTaskIDLength {
		return fmt.Errorf("task ID too long")
	}
	if len(task.Prompt) > maxPromptLength {
		return fmt.Errorf("prompt too long (max %d characters)", maxPromptLength)
	}
	if len(task.Prompt) == 0 {
		return fmt.Errorf("prompt cannot be empty")
	}
	return nil
}

func (r *EdgeRouter) Execute(ctx context.Context, task TaskRequest) (TaskResult, error) {
	decision, err := r.Route(ctx, task)
	if err != nil {
		return TaskResult{}, err
	}

	result := TaskResult{
		TaskID:  task.ID,
		Success: false,
	}

	var output string
	var execErr error
	startTime := time.Now()

	switch decision.Target {
	case TargetLocal:
		output, execErr = r.executeLocal(ctx, task)
		r.mu.Lock()
		r.stats.LocalCalls++
		if execErr == nil {
			r.stats.LocalSuccess++
			r.stats.AvgLocalLat = (r.stats.AvgLocalLat + time.Since(startTime).Milliseconds()) / 2
		}
		r.mu.Unlock()

	case TargetCloud:
		output, execErr = r.executeCloud(ctx, task, decision.Provider)
		r.mu.Lock()
		r.stats.CloudCalls++
		if execErr == nil {
			r.stats.CloudSuccess++
			r.stats.AvgCloudLat = (r.stats.AvgCloudLat + time.Since(startTime).Milliseconds()) / 2
		}
		r.mu.Unlock()
	}

	if execErr != nil && r.config.EnableFallback && decision.Target == TargetLocal {
		fallbackStart := time.Now()
		output, execErr = r.executeCloud(ctx, task, r.selectBestCloudProvider())
		result.FallbackUsed = true
		// Fallback cloud call must update cloud stats, mirroring the
		// TargetCloud branch. Without this, CloudCalls/CloudSuccess are
		// undercounted and SavingsPct is skewed.
		r.mu.Lock()
		r.stats.CloudCalls++
		if execErr == nil {
			r.stats.CloudSuccess++
			r.stats.AvgCloudLat = (r.stats.AvgCloudLat + time.Since(fallbackStart).Milliseconds()) / 2
		}
		r.mu.Unlock()
	}

	result.Output = output
	result.ErrorMsg = ""
	if execErr != nil {
		result.ErrorMsg = execErr.Error()
	}
	result.LatencyMs = time.Since(startTime).Milliseconds()
	result.Decision = decision
	result.Success = execErr == nil

	return result, execErr
}

func (r *EdgeRouter) executeLocal(ctx context.Context, task TaskRequest) (string, error) {
	if r.localModel == nil || !r.localModel.IsAvailable() {
		return "", fmt.Errorf("local model not available")
	}
	return r.localModel.Execute(ctx, task.Prompt, task.Options)
}

func (r *EdgeRouter) executeCloud(ctx context.Context, task TaskRequest, provider string) (string, error) {
	model, ok := r.cloudModels[provider]
	if !ok {
		return "", fmt.Errorf("cloud model %s not configured", provider)
	}
	return model.Execute(ctx, task.Prompt, task.Options)
}

func (r *EdgeRouter) analyzeComplexity(task TaskRequest) int {
	complexity := 5

	promptLen := len(task.Prompt)
	if promptLen > 1000 {
		complexity += 2
	} else if promptLen > 500 {
		complexity += 1
	}

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

	if complexity < 0 {
		complexity = 0
	}
	if complexity > 10 {
		complexity = 10
	}

	return complexity
}

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
		return "openai"
	}
	return bestProvider
}

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

type TaskRequest struct {
	ID                    string            `json:"id"`
	Prompt                string            `json:"prompt"`
	Options               map[string]string `json:"options"`
	ContainsSensitiveData bool              `json:"contains_sensitive_data"`
	Metadata              map[string]string `json:"metadata"`
}

type TaskResult struct {
	TaskID       string        `json:"task_id"`
	Output       string        `json:"output"`
	Success      bool          `json:"success"`
	ErrorMsg     string        `json:"error,omitempty"`
	LatencyMs    int64         `json:"latency_ms"`
	FallbackUsed bool          `json:"fallback_used"`
	Decision     RouteDecision `json:"decision"`
}

type RouteDecision struct {
	TaskID    string    `json:"task_id"`
	Target    Target    `json:"target"`
	Provider  string    `json:"provider,omitempty"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

type Target string

const (
	TargetLocal Target = "local"
	TargetCloud Target = "cloud"
)

type OllamaModel struct {
	endpoint  string
	modelName string
	available bool
	metrics   LocalMetrics
}

func NewOllamaModel(endpoint, modelName string) *OllamaModel {
	return &OllamaModel{
		endpoint:  endpoint,
		modelName: modelName,
	}
}

func (o *OllamaModel) IsAvailable() bool {
	return o.available
}

func (o *OllamaModel) GetModelName() string {
	return o.modelName
}

func (o *OllamaModel) Execute(ctx context.Context, prompt string, opts map[string]string) (string, error) {
	return fmt.Sprintf("Local model %s response for: %s", o.modelName, prompt[:min(50, len(prompt))]), nil
}

func (o *OllamaModel) GetMetrics() LocalMetrics {
	return o.metrics
}

type DeviceCapability struct {
	CPUCount   int     `json:"cpu_count"`
	MemoryGB   float64 `json:"memory_gb"`
	GPUModel   string  `json:"gpu_model,omitempty"`
	HasGPU     bool    `json:"has_gpu"`
	StorageGB  float64 `json:"storage_gb"`
	IsMobile   bool    `json:"is_mobile"`
	Platform   string  `json:"platform"`
	BatteryPct int     `json:"battery_pct,omitempty"`
	OnWiFi     bool    `json:"on_wifi"`
	OnCellular bool    `json:"on_cellular"`
}

func DetectCapability() DeviceCapability {
	cap := DeviceCapability{
		CPUCount: runtime.NumCPU(),
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	cap.MemoryGB = float64(m.Sys) / 1024 / 1024 / 1024

	if os.Getenv("CUDA_VISIBLE_DEVICES") != "" || os.Getenv("NVIDIA_VISIBLE_DEVICES") != "" {
		cap.HasGPU = true
	}

	// 检测移动端平台：Android、鸿蒙
	cap.IsMobile = os.Getenv("ANDROID_ROOT") != "" ||
		os.Getenv("TERMUX_VERSION") != "" ||
		os.Getenv("OHOS_ROOT") != "" ||
		os.Getenv("HOS_ROOT") != "" ||
		isHarmonyOS()
	cap.Platform = DetectPlatformType()

	return cap
}

// isHarmonyOS 检测是否运行在鸿蒙系统上
func isHarmonyOS() bool {
	if _, err := os.Stat("/system/etc/param/ohos.para"); err == nil {
		return true
	}
	return false
}

// DetectPlatformType 返回当前平台类型字符串
func DetectPlatformType() string {
	if os.Getenv("OHOS_ROOT") != "" || os.Getenv("HOS_ROOT") != "" || isHarmonyOS() {
		return "harmony"
	}
	if os.Getenv("ANDROID_ROOT") != "" || os.Getenv("TERMUX_VERSION") != "" {
		return "android"
	}
	if os.Getenv("CFFIXED_USER_HOME") != "" && os.Getenv("HOME") == "/var/mobile" {
		return "ios"
	}
	return "desktop"
}

func RecommendLocalModel(cap DeviceCapability) string {
	if cap.HasGPU && cap.MemoryGB >= 16 {
		return "llama3:70b"
	}
	if cap.HasGPU && cap.MemoryGB >= 8 {
		return "llama3:8b"
	}
	if cap.MemoryGB < 8 || cap.IsMobile {
		return "llama3:2b-q4"
	}
	return "llama3:7b"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type PrivacyAnalyzer struct {
	sensitivePatterns []*regexp.Regexp
}

func NewPrivacyAnalyzer() *PrivacyAnalyzer {
	return &PrivacyAnalyzer{
		sensitivePatterns: []*regexp.Regexp{
			// 中文敏感词不使用 \b：Go RE2 的 \b 是 ASCII 词边界，
			// 仅在 [a-zA-Z0-9_] word 字符与非 word 字符之间匹配，
			// 中文相邻时不存在 ASCII 词边界，\b中文\b 永远不会命中。
			regexp.MustCompile(`(?i)密码`),
			regexp.MustCompile(`(?i)\bpassword\b`),
			regexp.MustCompile(`(?i)\bsecret\b`),
			regexp.MustCompile(`(?i)\btoken\b`),
			regexp.MustCompile(`(?i)身份证`),
			regexp.MustCompile(`(?i)\bid[_-]?card\b`),
			regexp.MustCompile(`(?i)\bssn\b`),
			regexp.MustCompile(`(?i)\bsocial[_-]?security\b`),
			regexp.MustCompile(`(?i)银行卡`),
			regexp.MustCompile(`(?i)\bcredit[_-]?card\b`),
			regexp.MustCompile(`(?i)\bcard[_-]?number\b`),
			regexp.MustCompile(`(?i)私钥`),
			regexp.MustCompile(`(?i)\bprivate[_-]?key\b`),
			regexp.MustCompile(`(?i)\bapi[_-]?key\b`),
			regexp.MustCompile(`(?i)地址`),
			regexp.MustCompile(`(?i)\baddress\b`),
			regexp.MustCompile(`(?i)\blocation\b`),
			regexp.MustCompile(`(?i)手机号`),
			regexp.MustCompile(`(?i)\bphone\b`),
			regexp.MustCompile(`(?i)\btelephone\b`),
			regexp.MustCompile(`(?i)邮箱`),
			regexp.MustCompile(`(?i)\bemail\b`),
			regexp.MustCompile(`(?i)\bmail\b`),
			regexp.MustCompile(`(?i)\bghp_[A-Za-z0-9]{20,}\b`),
			regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9]{20,}\b`),
			regexp.MustCompile(`(?i)\bxox[baprs]-[A-Za-z0-9-]{10,}\b`),
			regexp.MustCompile(`(?i)\bearer\s+[A-Za-z0-9-_]{10,}`),
			regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			regexp.MustCompile(`(?i)\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
			regexp.MustCompile(`(?i)\b\d{11}\b`),
			regexp.MustCompile(`(?i)\b\d{6}[-]?\d{4}[-]?\d{4}[-]?\d{4}\b`),
		},
	}
}

func (p *PrivacyAnalyzer) Analyze(text string) bool {
	textLower := strings.ToLower(text)
	for _, pattern := range p.sensitivePatterns {
		if pattern.MatchString(textLower) {
			return true
		}
	}
	return false
}

func ToJSON(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func GenerateSecureID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("edge-%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}

// AgentInfo holds the metadata for a discoverable cross-domain agent,
// including its DID identity, service endpoints, and declared capabilities.
type AgentInfo struct {
	DID          string            `json:"did"`
	Endpoint     string            `json:"endpoint"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	Trusted      bool              `json:"trusted"`
}

// Validate checks if the agent info is well-formed.
func (a *AgentInfo) Validate() error {
	if a.DID == "" {
		return fmt.Errorf("agent DID is required")
	}
	if len(a.DID) > 256 {
		return fmt.Errorf("agent DID too long")
	}
	if !strings.HasPrefix(a.DID, "did:") {
		return fmt.Errorf("invalid agent DID format")
	}
	parts := strings.Split(a.DID, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid DID format: expected did:method:identifier")
	}
	if a.Endpoint == "" {
		return fmt.Errorf("agent endpoint is required")
	}
	if err := validateEndpoint(a.Endpoint); err != nil {
		return fmt.Errorf("invalid agent endpoint: %w", err)
	}
	if len(a.Capabilities) > maxAgentCapabilities {
		return fmt.Errorf("too many capabilities (max %d)", maxAgentCapabilities)
	}
	for _, cap := range a.Capabilities {
		if len(cap) > 100 {
			return fmt.Errorf("capability name too long")
		}
	}
	if len(a.Metadata) > maxAgentMetadataEntries {
		return fmt.Errorf("too many metadata entries (max %d)", maxAgentMetadataEntries)
	}
	for k, v := range a.Metadata {
		if len(k) > maxAgentMetadataKeyLen {
			return fmt.Errorf("metadata key too long: %s", k)
		}
		if len(v) > maxAgentMetadataValueLen {
			return fmt.Errorf("metadata value too long for key: %s", k)
		}
	}
	return nil
}

// AgentRegistry maintains a directory of known cross-domain agents.
type AgentRegistry struct {
	agents map[string]*AgentInfo
	mu     sync.RWMutex
}

// NewAgentRegistry creates a new agent registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]*AgentInfo),
	}
}

// RegisterAgent adds or updates an agent in the registry.
func (r *AgentRegistry) RegisterAgent(agent *AgentInfo) error {
	if err := agent.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.agents[agent.DID]; !exists && len(r.agents) >= maxRegistrySize {
		return fmt.Errorf("agent registry is full (max %d)", maxRegistrySize)
	}
	agent.LastSeen = time.Now()
	r.agents[agent.DID] = agent
	return nil
}

// GetAgent retrieves an agent by its DID.
func (r *AgentRegistry) GetAgent(did string) (*AgentInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.agents[did]
	return agent, ok
}

// ListAgents returns all registered agents.
func (r *AgentRegistry) ListAgents() []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		result = append(result, agent)
	}
	return result
}

// DiscoverAgentsByCapability returns agents that support a given capability.
func (r *AgentRegistry) DiscoverAgentsByCapability(capability string) []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*AgentInfo
	for _, agent := range r.agents {
		for _, cap := range agent.Capabilities {
			if cap == capability {
				result = append(result, agent)
				break
			}
		}
	}
	return result
}

// RemoveAgent removes an agent from the registry.
func (r *AgentRegistry) RemoveAgent(did string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, did)
}

// CrossDomainMessenger handles routing messages between agents across
// different domains, inspired by awiki.ai's cross-domain messaging.
type CrossDomainMessenger struct {
	registry  *AgentRegistry
	sentCount int
	errCount  int
	mu        sync.RWMutex
}

// NewCrossDomainMessenger creates a new cross-domain messenger.
func NewCrossDomainMessenger(registry *AgentRegistry) *CrossDomainMessenger {
	return &CrossDomainMessenger{
		registry: registry,
	}
}

// SendMessage routes a message to the target agent identified by DID.
// This is a placeholder implementation; in production it would perform
// an actual HTTP POST to the agent's endpoint with proper auth.
func (m *CrossDomainMessenger) SendMessage(senderDID, receiverDID, payload string) error {
	if senderDID == "" || receiverDID == "" {
		return fmt.Errorf("sender and receiver DIDs are required")
	}
	if len(senderDID) > 256 || !strings.HasPrefix(senderDID, "did:") {
		return fmt.Errorf("invalid sender DID format")
	}
	if len(receiverDID) > 256 || !strings.HasPrefix(receiverDID, "did:") {
		return fmt.Errorf("invalid receiver DID format")
	}
	if senderDID == receiverDID {
		return fmt.Errorf("sender and receiver DIDs must be different")
	}
	if len(payload) > maxPayloadLength {
		return fmt.Errorf("payload too long (max %d bytes)", maxPayloadLength)
	}
	agent, ok := m.registry.GetAgent(receiverDID)
	if !ok {
		return fmt.Errorf("agent %s not found in registry", receiverDID)
	}
	if !agent.Trusted {
		return fmt.Errorf("agent %s is not trusted", receiverDID)
	}

	m.mu.Lock()
	m.sentCount++
	m.mu.Unlock()

	// Placeholder: real implementation would POST to agent.Endpoint
	_ = agent.Endpoint
	_ = payload
	return nil
}

// ResolveDIDEndpoint looks up the service endpoint for a given DID.
func (m *CrossDomainMessenger) ResolveDIDEndpoint(did string) (string, error) {
	agent, ok := m.registry.GetAgent(did)
	if !ok {
		return "", fmt.Errorf("DID %s not found", did)
	}
	return agent.Endpoint, nil
}

// GetStats returns messenger statistics.
func (m *CrossDomainMessenger) GetStats() (sent, errors int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sentCount, m.errCount
}

// ============================================================
// ReAct 推理循环引擎（借鉴 JiuwenClaw-on-OpenHarmony）
// ReAct = Reasoning + Acting，交替进行推理和行动，
// 直到达成目标或超出最大迭代次数。
// ============================================================

// ReActStep 表示 ReAct 循环中的单步
type ReActStep struct {
	Iteration   int       `json:"iteration"`
	Thought     string    `json:"thought"`
	Action      string    `json:"action"`
	Input       string    `json:"input"`
	Observation string    `json:"observation"`
	Timestamp   time.Time `json:"timestamp"`
}

// ReActEngine ReAct 推理循环引擎
type ReActEngine struct {
	mu             sync.Mutex
	maxIterations  int
	thoughtHistory []ReActStep
	memory         *LayeredMemory
}

// NewReActEngine 创建新的 ReAct 引擎
func NewReActEngine(maxIterations int, memory *LayeredMemory) *ReActEngine {
	if maxIterations <= 0 || maxIterations > 20 {
		maxIterations = 10
	}
	if memory == nil {
		memory = NewLayeredMemory()
	}
	return &ReActEngine{
		maxIterations:  maxIterations,
		thoughtHistory: make([]ReActStep, 0, maxIterations),
		memory:         memory,
	}
}

// ReActResult 是 ReAct 循环的最终结果
type ReActResult struct {
	Steps       []ReActStep `json:"steps"`
	FinalAnswer string      `json:"final_answer"`
	Iterations  int         `json:"iterations"`
	Success     bool        `json:"success"`
}

// Run 执行 ReAct 推理循环
// thinkFn: 推理函数，输入当前状态，输出 thought + action + actionInput
// actFn: 行动函数，输入 action + actionInput，输出 observation
func (e *ReActEngine) Run(ctx context.Context, task string, thinkFn ThinkFunc, actFn ActFunc) (*ReActResult, error) {
	if thinkFn == nil {
		return nil, fmt.Errorf("think function is required")
	}
	if actFn == nil {
		return nil, fmt.Errorf("act function is required")
	}
	if len(task) == 0 {
		return nil, fmt.Errorf("task is required")
	}
	if len(task) > maxPromptLength {
		return nil, fmt.Errorf("task too long (max %d)", maxPromptLength)
	}

	// Run 开始时重置 thoughtHistory，避免跨 Run 调用无限增长
	e.mu.Lock()
	e.thoughtHistory = make([]ReActStep, 0, e.maxIterations)
	e.mu.Unlock()

	result := &ReActResult{Steps: []ReActStep{}}
	currentContext := task

	// 从记忆中检索相关历史
	relevantMemories := e.memory.Recall(task)
	if len(relevantMemories) > 0 {
		contextBuilder := currentContext
		for _, m := range relevantMemories {
			contextBuilder += "\n[Related memory: " + m + "]"
		}
		currentContext = contextBuilder
	}

	for i := 1; i <= e.maxIterations; i++ {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Iterations = i - 1
			result.FinalAnswer = "context cancelled"
			return result, ctx.Err()
		default:
		}

		// 推理阶段：生成 thought + action
		thought, action, actionInput, finished, err := thinkFn(ctx, i, currentContext)
		if err != nil {
			result.Success = false
			result.Iterations = i
			result.FinalAnswer = fmt.Sprintf("error at iteration %d: %v", i, err)
			return result, nil
		}

		// 限制 thought / action / actionInput 长度，防止内存膨胀
		if len(thought) > maxThoughtLength {
			thought = thought[:maxThoughtLength]
		}
		if len(action) > maxActionLength {
			action = action[:maxActionLength]
		}
		if len(actionInput) > maxActionInputLength {
			actionInput = actionInput[:maxActionInputLength]
		}

		step := ReActStep{
			Iteration: i,
			Thought:   thought,
			Action:    action,
			Input:     actionInput,
			Timestamp: time.Now(),
		}

		// 如果推理已完成，直接返回最终答案
		if finished {
			step.Observation = "task completed"
			result.Steps = append(result.Steps, step)
			result.FinalAnswer = actionInput
			result.Success = true
			result.Iterations = i
			// 存入长期记忆：对 task 做 hash 作为 key，避免 key 过长或与 task 不一致
			longTermKey := hashMemoryKey(task)
			if err := e.memory.Store(longTermKey, actionInput, MemoryLevelLongTerm); err != nil {
				log.Printf("warning: failed to store long-term memory: %v", err)
			}
			return result, nil
		}

		// 行动阶段：执行 action
		observation, err := actFn(ctx, action, actionInput)
		if err != nil {
			step.Observation = fmt.Sprintf("action error: %v", err)
			result.Steps = append(result.Steps, step)
			result.Success = false
			result.Iterations = i
			result.FinalAnswer = step.Observation
			return result, nil
		}

		// 限制 observation 长度，按 rune 边界截断避免产生无效 UTF-8
		if len(observation) > maxObservationLength {
			cut := maxObservationLength
			for cut > 0 && !utf8.RuneStart(observation[cut]) {
				cut--
			}
			observation = observation[:cut] + "... (truncated)"
		}

		step.Observation = observation
		result.Steps = append(result.Steps, step)
		e.mu.Lock()
		e.thoughtHistory = append(e.thoughtHistory, step)
		e.mu.Unlock()

		// 更新上下文：加入新的观察结果
		currentContext = fmt.Sprintf("%s\n\n[Step %d] Thought: %s\nAction: %s\nInput: %s\nObservation: %s",
			task, i, thought, action, actionInput, observation)

		// 存入短期记忆
		if err := e.memory.Store(fmt.Sprintf("step_%d_observation", i), observation, MemoryLevelShortTerm); err != nil {
			// short-term memory store failure is non-fatal; observation already recorded in result
		}
	}

	// 超过最大迭代次数
	result.Success = false
	result.Iterations = e.maxIterations
	result.FinalAnswer = "max iterations reached without completion"
	return result, nil
}

// GetHistory 返回推理历史（返回副本，避免外部修改内部切片）
func (e *ReActEngine) GetHistory() []ReActStep {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]ReActStep, len(e.thoughtHistory))
	copy(result, e.thoughtHistory)
	return result
}

// ThinkFunc 推理函数类型
// 返回: thought, action, actionInput, finished, error
type ThinkFunc func(ctx context.Context, iteration int, context string) (string, string, string, bool, error)

// ActFunc 行动函数类型
// 返回: observation, error
type ActFunc func(ctx context.Context, action string, input string) (string, error)

// ============================================================
// 分层持久化记忆（借鉴 JiuwenClaw-on-OpenHarmony 的分层记忆系统）
// 三层记忆：短期（会话级）、工作（任务级）、长期（持久化）
// ============================================================

// MemoryLevel 记忆层级
type MemoryLevel string

const (
	MemoryLevelShortTerm MemoryLevel = "short_term" // 会话级，当前对话上下文
	MemoryLevelWorking   MemoryLevel = "working"    // 任务级，跨步骤但非持久
	MemoryLevelLongTerm  MemoryLevel = "long_term"  // 持久化，跨会话保留
)

// MemoryEntry 单条记忆
type MemoryEntry struct {
	ID          string      `json:"id"`
	Content     string      `json:"content"`
	Level       MemoryLevel `json:"level"`
	Tags        []string    `json:"tags,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	AccessCount int         `json:"access_count"`
	LastAccess  time.Time   `json:"last_access"`
}

// LayeredMemory 分层记忆存储
type LayeredMemory struct {
	shortTerm  map[string]*MemoryEntry // 短期记忆（内存）
	working    map[string]*MemoryEntry // 工作记忆（内存）
	longTerm   map[string]*MemoryEntry // 长期记忆（持久化文件）
	mu         sync.RWMutex
	storePath  string // 长期记忆持久化路径
	maxEntries int    // 每层最大条目数
}

const (
	defaultMaxMemoryEntries = 500
	maxMemoryContentLength  = 8192
	maxMemoryTags           = 10
	maxRecallResults        = 10
	maxRecallTotalBytes     = 64 * 1024 // 64KB
	maxLongTermFileSize     = 64 * 1024 // 64KB
	maxRecallQueryLength    = 1024
	maxThoughtLength        = 4096
	maxActionLength         = 256
	maxActionInputLength    = 8192
	maxObservationLength    = 4096
)

// NewLayeredMemory 创建分层记忆存储
func NewLayeredMemory() *LayeredMemory {
	return &LayeredMemory{
		shortTerm:  make(map[string]*MemoryEntry),
		working:    make(map[string]*MemoryEntry),
		longTerm:   make(map[string]*MemoryEntry),
		maxEntries: defaultMaxMemoryEntries,
	}
}

// SetStorePath 设置长期记忆持久化路径
func (m *LayeredMemory) SetStorePath(path string) error {
	if len(path) > 512 {
		return fmt.Errorf("path too long")
	}
	// 路径安全校验：必须是绝对路径、不能是根目录、不包含 ".."
	if !filepath.IsAbs(path) {
		return fmt.Errorf("store path must be absolute")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("store path cannot contain '..'")
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == "/" || cleanPath == filepath.VolumeName(cleanPath)+string(filepath.Separator) {
		return fmt.Errorf("store path cannot be root directory")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storePath = cleanPath
	// 注意：loadLongTerm 中的磁盘 I/O 在持锁状态下执行。
	// 这是为了保证 storePath 与 longTerm map 的一致性，避免并发 SetStorePath / Store 期间出现竞态。
	// SetStorePath 通常在初始化阶段调用，此处持锁加载可接受。
	m.loadLongTerm()
	return nil
}

// Store 存储一条记忆
func (m *LayeredMemory) Store(key, content string, level MemoryLevel) error {
	if key == "" {
		return fmt.Errorf("memory key is required")
	}
	if len(key) > 256 {
		return fmt.Errorf("memory key too long")
	}
	if content == "" {
		return fmt.Errorf("content is required")
	}
	if len(content) > maxMemoryContentLength {
		content = content[:maxMemoryContentLength] + "... (truncated)"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &MemoryEntry{
		ID:         GenerateSecureID(),
		Content:    content,
		Level:      level,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
	}

	switch level {
	case MemoryLevelShortTerm:
		if len(m.shortTerm) >= m.maxEntries {
			m.evictOldest(m.shortTerm, MemoryLevelShortTerm)
		}
		m.shortTerm[key] = entry
	case MemoryLevelWorking:
		if len(m.working) >= m.maxEntries {
			m.evictOldest(m.working, MemoryLevelWorking)
		}
		m.working[key] = entry
	case MemoryLevelLongTerm:
		// 对 long-term key 做 sanitize（SHA-256 hex）后再作为 map key 与文件名，
		// 保证磁盘条目与内存条目使用同一 key，evictOldest 删除文件时也能匹配。
		storeKey := sanitizeKey(key)
		if len(m.longTerm) >= m.maxEntries {
			m.evictOldest(m.longTerm, MemoryLevelLongTerm)
		}
		m.longTerm[storeKey] = entry
		// 持久化到文件。注意：persistLongTerm 中的磁盘 I/O 在持锁状态下执行，
		// 这是为了保证内存 map 与磁盘文件的一致性（避免并发淘汰导致文件与 map 不同步）。
		// 若长期记忆写入频繁成为瓶颈，可考虑改为异步队列写入，但需额外处理一致性。
		if m.storePath != "" {
			m.persistLongTerm(storeKey, entry)
		}
	default:
		return fmt.Errorf("invalid memory level: %s", level)
	}
	return nil
}

// Recall 检索与查询相关的记忆
func (m *LayeredMemory) Recall(query string) []string {
	// 使用写锁：Recall 会更新 entry.AccessCount 和 entry.LastAccess
	m.mu.Lock()
	defer m.mu.Unlock()

	// 限制 query 长度，避免超长 query 造成 DoS
	if len(query) > maxRecallQueryLength {
		query = query[:maxRecallQueryLength]
	}
	queryLower := strings.ToLower(query)
	var results []string
	totalBytes := 0

	// tryAdd 在 top-K 与总字节数上限内追加结果
	tryAdd := func(s string) bool {
		if len(results) >= maxRecallResults {
			return false
		}
		if totalBytes+len(s) > maxRecallTotalBytes {
			return false
		}
		results = append(results, s)
		totalBytes += len(s)
		return true
	}

	// 优先检索长期记忆（权重最高）
	for _, entry := range m.longTerm {
		if len(results) >= maxRecallResults {
			break
		}
		if strings.Contains(strings.ToLower(entry.Content), queryLower) {
			entry.AccessCount++
			entry.LastAccess = time.Now()
			if !tryAdd("[long-term] " + entry.Content) {
				break
			}
		}
	}
	// 工作记忆
	for _, entry := range m.working {
		if len(results) >= maxRecallResults {
			break
		}
		if strings.Contains(strings.ToLower(entry.Content), queryLower) {
			entry.AccessCount++
			entry.LastAccess = time.Now()
			if !tryAdd("[working] " + entry.Content) {
				break
			}
		}
	}
	// 短期记忆：按 CreatedAt 排序后取最近 5 条，避免依赖 map 随机迭代
	shortTermList := make([]*MemoryEntry, 0, len(m.shortTerm))
	for _, entry := range m.shortTerm {
		shortTermList = append(shortTermList, entry)
	}
	sort.Slice(shortTermList, func(i, j int) bool {
		return shortTermList[i].CreatedAt.After(shortTermList[j].CreatedAt)
	})
	shortTermLimit := 5
	for idx, entry := range shortTermList {
		if idx >= shortTermLimit {
			break
		}
		if len(results) >= maxRecallResults {
			break
		}
		if !tryAdd("[short-term] " + entry.Content) {
			break
		}
	}

	return results
}

// Clear 清除指定层级的记忆
func (m *LayeredMemory) Clear(level MemoryLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch level {
	case MemoryLevelShortTerm:
		m.shortTerm = make(map[string]*MemoryEntry)
	case MemoryLevelWorking:
		m.working = make(map[string]*MemoryEntry)
	case MemoryLevelLongTerm:
		m.longTerm = make(map[string]*MemoryEntry)
		if m.storePath != "" {
			// 安全清理：仅删除目录下的 *.json 文件，绝不删除整个目录，
			// 避免误删 storePath 自身或其兄弟目录。
			entries, err := os.ReadDir(m.storePath)
			if err != nil {
				return
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				fullPath := filepath.Join(m.storePath, entry.Name())
				// 跳过符号链接，避免跟随链接删除其他文件
				if li, err := os.Lstat(fullPath); err == nil {
					if li.Mode()&os.ModeSymlink != 0 {
						continue
					}
				} else {
					continue
				}
				if err := os.Remove(fullPath); err != nil {
					log.Printf("warning: failed to remove long-term memory file %s: %v", fullPath, err)
				}
			}
		}
	}
}

// GetStats 返回各层记忆统计
func (m *LayeredMemory) GetStats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]int{
		"short_term": len(m.shortTerm),
		"working":    len(m.working),
		"long_term":  len(m.longTerm),
	}
}

// evictOldest 淘汰最旧的记忆条目（按 LastAccess 比较）。
// 对 longTerm 层级会同时删除对应的 .json 文件，保证内存与磁盘一致。
func (m *LayeredMemory) evictOldest(store map[string]*MemoryEntry, level MemoryLevel) {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range store {
		if oldestKey == "" || v.LastAccess.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.LastAccess
		}
	}
	if oldestKey == "" {
		return
	}
	delete(store, oldestKey)
	// 长期记忆淘汰时同时删除对应的 .json 文件
	if level == MemoryLevelLongTerm && m.storePath != "" {
		filePath := filepath.Join(m.storePath, oldestKey+".json")
		if err := os.Remove(filePath); err != nil {
			log.Printf("warning: failed to remove evicted long-term memory file %s: %v", filePath, err)
		}
	}
}

// persistLongTerm 将长期记忆持久化到文件。
// 注意：调用方传入的 key 已经过 sanitizeKey 处理（SHA-256 hex），
// 此处直接用作文件名，不再二次 sanitize，保证与 loadLongTerm/evictOldest 的文件名一致。
func (m *LayeredMemory) persistLongTerm(key string, entry *MemoryEntry) {
	if m.storePath == "" {
		return
	}
	// 安全地创建目录
	if err := os.MkdirAll(m.storePath, 0700); err != nil {
		log.Printf("warning: failed to create long-term memory dir %s: %v", m.storePath, err)
		return
	}
	filePath := filepath.Join(m.storePath, key+".json")
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("warning: failed to marshal long-term memory %s: %v", filePath, err)
		return
	}
	// 限制文件大小
	if len(data) > maxMemoryContentLength*2 {
		return
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		log.Printf("warning: failed to write long-term memory file %s: %v", filePath, err)
		return
	}
	// 文件已存在时 os.WriteFile 不会更改其权限，强制重置为 0600
	if err := os.Chmod(filePath, 0600); err != nil {
		log.Printf("warning: failed to chmod long-term memory file %s: %v", filePath, err)
	}
}

// loadLongTerm 从文件加载长期记忆
func (m *LayeredMemory) loadLongTerm() {
	if m.storePath == "" {
		return
	}
	entries, err := os.ReadDir(m.storePath)
	if err != nil {
		return
	}
	count := 0
	for _, entry := range entries {
		if count >= m.maxEntries {
			break
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		fullPath := filepath.Join(m.storePath, entry.Name())
		// 读取前用 os.Lstat 检查：跳过符号链接，避免跟随链接读取任意文件
		li, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		if li.Mode()&os.ModeSymlink != 0 {
			continue
		}
		// 文件大小限制：超过 64KB 则跳过，防止读取超大文件造成内存膨胀
		if li.Size() > maxLongTermFileSize {
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		var memEntry MemoryEntry
		if err := json.Unmarshal(data, &memEntry); err != nil {
			continue
		}
		// 反序列化后检查 Content 长度，避免加载超长内容
		if len(memEntry.Content) > maxMemoryContentLength {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		m.longTerm[key] = &memEntry
		count++
	}
}

// sanitizeKey 将 key 转为安全的文件名。
// 使用 SHA-256 hash 前 16 字节的 hex 编码（共 32 个十六进制字符），
// 避免不同 key 经字符过滤后碰撞到同一文件名。
func sanitizeKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16])
}

// hashMemoryKey 对 task 做哈希生成长期记忆的 key，
// 保证 key 长度固定且与 task 内容一致，避免 task 过长或含特殊字符。
func hashMemoryKey(task string) string {
	h := sha256.Sum256([]byte(task))
	return hex.EncodeToString(h[:16])
}
