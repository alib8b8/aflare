package edge

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
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
	APIKey   string `json:"api_key,omitempty"`
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
		output, execErr = r.executeCloud(ctx, task, r.selectBestCloudProvider())
		result.FallbackUsed = true
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
			regexp.MustCompile(`(?i)\b密码\b`),
			regexp.MustCompile(`(?i)\bpassword\b`),
			regexp.MustCompile(`(?i)\bsecret\b`),
			regexp.MustCompile(`(?i)\btoken\b`),
			regexp.MustCompile(`(?i)\b身份证\b`),
			regexp.MustCompile(`(?i)\bid[_-]?card\b`),
			regexp.MustCompile(`(?i)\bssn\b`),
			regexp.MustCompile(`(?i)\bsocial[_-]?security\b`),
			regexp.MustCompile(`(?i)\b银行卡\b`),
			regexp.MustCompile(`(?i)\bcredit[_-]?card\b`),
			regexp.MustCompile(`(?i)\bcard[_-]?number\b`),
			regexp.MustCompile(`(?i)\b私钥\b`),
			regexp.MustCompile(`(?i)\bprivate[_-]?key\b`),
			regexp.MustCompile(`(?i)\bapi[_-]?key\b`),
			regexp.MustCompile(`(?i)\b地址\b`),
			regexp.MustCompile(`(?i)\baddress\b`),
			regexp.MustCompile(`(?i)\blocation\b`),
			regexp.MustCompile(`(?i)\b手机号\b`),
			regexp.MustCompile(`(?i)\bphone\b`),
			regexp.MustCompile(`(?i)\btelephone\b`),
			regexp.MustCompile(`(?i)\b邮箱\b`),
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
	if len(task) > maxPromptLength {
		return nil, fmt.Errorf("task too long (max %d)", maxPromptLength)
	}

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
			// 存入记忆
			e.memory.Store(task, actionInput, MemoryLevelLongTerm)
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

		// 限制 observation 长度
		if len(observation) > 4096 {
			observation = observation[:4096] + "... (truncated)"
		}

		step.Observation = observation
		result.Steps = append(result.Steps, step)
		e.thoughtHistory = append(e.thoughtHistory, step)

		// 更新上下文：加入新的观察结果
		currentContext = fmt.Sprintf("%s\n\n[Step %d] Thought: %s\nAction: %s\nInput: %s\nObservation: %s",
			task, i, thought, action, actionInput, observation)

		// 存入短期记忆
		e.memory.Store(fmt.Sprintf("step_%d_observation", i), observation, MemoryLevelShortTerm)
	}

	// 超过最大迭代次数
	result.Success = false
	result.Iterations = e.maxIterations
	result.FinalAnswer = "max iterations reached without completion"
	return result, nil
}

// GetHistory 返回推理历史
func (e *ReActEngine) GetHistory() []ReActStep {
	return e.thoughtHistory
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storePath = path
	// 尝试加载已有的长期记忆
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
			m.evictOldest(m.shortTerm)
		}
		m.shortTerm[key] = entry
	case MemoryLevelWorking:
		if len(m.working) >= m.maxEntries {
			m.evictOldest(m.working)
		}
		m.working[key] = entry
	case MemoryLevelLongTerm:
		if len(m.longTerm) >= m.maxEntries {
			m.evictOldest(m.longTerm)
		}
		m.longTerm[key] = entry
		// 持久化到文件
		if m.storePath != "" {
			m.persistLongTerm(key, entry)
		}
	default:
		return fmt.Errorf("invalid memory level: %s", level)
	}
	return nil
}

// Recall 检索与查询相关的记忆
func (m *LayeredMemory) Recall(query string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []string

	// 优先检索长期记忆（权重最高）
	for _, entry := range m.longTerm {
		if strings.Contains(strings.ToLower(entry.Content), queryLower) {
			entry.AccessCount++
			entry.LastAccess = time.Now()
			results = append(results, "[long-term] "+entry.Content)
		}
	}
	// 工作记忆
	for _, entry := range m.working {
		if strings.Contains(strings.ToLower(entry.Content), queryLower) {
			entry.AccessCount++
			entry.LastAccess = time.Now()
			results = append(results, "[working] "+entry.Content)
		}
	}
	// 短期记忆（仅最近几条）
	count := 0
	for _, entry := range m.shortTerm {
		if count >= 5 {
			break
		}
		results = append(results, "[short-term] "+entry.Content)
		count++
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
			os.RemoveAll(m.storePath)
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

// evictOldest 淘汰最旧的记忆条目
func (m *LayeredMemory) evictOldest(store map[string]*MemoryEntry) {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range store {
		if oldestKey == "" || v.CreatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.CreatedAt
		}
	}
	delete(store, oldestKey)
}

// persistLongTerm 将长期记忆持久化到文件
func (m *LayeredMemory) persistLongTerm(key string, entry *MemoryEntry) {
	if m.storePath == "" {
		return
	}
	// 安全地创建目录
	if err := os.MkdirAll(m.storePath, 0700); err != nil {
		return
	}
	// 安全的文件名
	safeKey := sanitizeKey(key)
	filePath := m.storePath + "/" + safeKey + ".json"
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// 限制文件大小
	if len(data) > maxMemoryContentLength*2 {
		return
	}
	os.WriteFile(filePath, data, 0600)
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
		data, err := os.ReadFile(m.storePath + "/" + entry.Name())
		if err != nil {
			continue
		}
		var memEntry MemoryEntry
		if err := json.Unmarshal(data, &memEntry); err != nil {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		m.longTerm[key] = &memEntry
		count++
	}
}

// sanitizeKey 将 key 转为安全的文件名
func sanitizeKey(key string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, key)
	if len(result) > 128 {
		result = result[:128]
	}
	if result == "" {
		result = "memory"
	}
	return result
}
