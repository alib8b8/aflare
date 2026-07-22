// Package stats provides token usage tracking and cost calculation for workflows.
package stats

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ModelPricing defines pricing for a specific model.
type ModelPricing struct {
	InputPricePer1K  float64 // USD per 1K input tokens
	OutputPricePer1K float64 // USD per 1K output tokens
}

// PricingTable maps model names to their pricing.
// Prices are approximate and may vary by provider.
var PricingTable = map[string]ModelPricing{
	// OpenAI models
	"gpt-4o":           {0.0025, 0.01},
	"gpt-4o-mini":      {0.00015, 0.0006},
	"gpt-4-turbo":      {0.01, 0.03},
	"gpt-4":            {0.03, 0.06},
	"gpt-3.5-turbo":    {0.0005, 0.0015},
	"o1-preview":       {0.015, 0.06},
	"o1-mini":          {0.003, 0.012},

	// Anthropic models
	"claude-3-5-sonnet": {0.003, 0.015},
	"claude-3-opus":     {0.015, 0.075},
	"claude-3-sonnet":   {0.003, 0.015},
	"claude-3-haiku":    {0.00025, 0.00125},

	// DeepSeek models
	"deepseek-chat":     {0.00014, 0.00028},
	"deepseek-coder":    {0.00014, 0.00028},
	"deepseek-reasoner": {0.00055, 0.00219},

	// Qwen models
	"qwen-max":          {0.0004, 0.0012},
	"qwen-plus":         {0.00008, 0.0002},
	"qwen-turbo":        {0.00002, 0.00006},

	// GLM models
	"glm-4":             {0.001, 0.001},
	"glm-4-flash":       {0.00001, 0.00001},

	// Kimi models
	"moonshot-v1-8k":    {0.012, 0.012},
	"moonshot-v1-32k":   {0.024, 0.024},
	"moonshot-v1-128k":  {0.06, 0.06},

	// Local models (free)
	"llama2":      {0, 0},
	"llama3":      {0, 0},
	"llama3.1":    {0, 0},
	"mistral":     {0, 0},
	"qwen2":       {0, 0},
	"deepseek-r1": {0, 0},
}

// StepStats tracks statistics for a single step.
type StepStats struct {
	StepName     string        `json:"step_name"`
	NodeType     string        `json:"node_type"`
	Model        string        `json:"model,omitempty"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	TotalTokens  int           `json:"total_tokens"`
	Cost         float64       `json:"cost"`
	Duration     time.Duration `json:"duration"`
	Timestamp    time.Time     `json:"timestamp"`
}

// WorkflowStats tracks overall workflow statistics.
type WorkflowStats struct {
	mu            sync.RWMutex
	WorkflowName  string     `json:"workflow_name"`
	Steps         []StepStats `json:"steps"`
	TotalInput    int        `json:"total_input_tokens"`
	TotalOutput   int        `json:"total_output_tokens"`
	TotalCost     float64    `json:"total_cost"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       time.Time  `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	ProviderCalls map[string]int `json:"provider_calls"` // Provider -> call count
}

// StatsCollector collects and aggregates workflow statistics.
type StatsCollector struct {
	mu     sync.RWMutex
	stats  map[string]*WorkflowStats
	current *WorkflowStats
}

// NewStatsCollector creates a new stats collector.
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		stats: make(map[string]*WorkflowStats),
	}
}

// StartWorkflow begins tracking a new workflow.
func (s *StatsCollector) StartWorkflow(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := &WorkflowStats{
		WorkflowName:  name,
		Steps:         make([]StepStats, 0),
		StartTime:     time.Now(),
		ProviderCalls: make(map[string]int),
	}
	s.stats[name] = stats
	s.current = stats
}

// RecordStep records statistics for a completed step.
func (s *StatsCollector) RecordStep(stepName, nodeType, model string, inputTokens, outputTokens int, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current == nil {
		return
	}

	cost := calculateCost(model, inputTokens, outputTokens)

	stepStats := StepStats{
		StepName:     stepName,
		NodeType:     nodeType,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Cost:         cost,
		Duration:     duration,
		Timestamp:    time.Now(),
	}

	s.current.Steps = append(s.current.Steps, stepStats)
	s.current.TotalInput += inputTokens
	s.current.TotalOutput += outputTokens
	s.current.TotalCost += cost

	// Track provider calls
	if model != "" {
		s.current.ProviderCalls[model]++
	}
}

// EndWorkflow finalizes the workflow statistics.
func (s *StatsCollector) EndWorkflow(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if stats, ok := s.stats[name]; ok {
		stats.EndTime = time.Now()
		stats.Duration = stats.EndTime.Sub(stats.StartTime)
	}
}

// GetStats returns statistics for a specific workflow.
func (s *StatsCollector) GetStats(name string) *WorkflowStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stats[name]
}

// GetCurrentStats returns the current workflow statistics.
func (s *StatsCollector) GetCurrentStats() *WorkflowStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.current
}

// FormatReport generates a human-readable report.
func (s *StatsCollector) FormatReport(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.stats[name]
	if !ok {
		return "No statistics available"
	}

	var sb strings.Builder

	sb.WriteString("📊 Token Usage Report\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	sb.WriteString(fmt.Sprintf("Workflow: %s\n", stats.WorkflowName))
	sb.WriteString(fmt.Sprintf("Duration: %v\n", stats.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Steps: %d\n\n", len(stats.Steps)))

	// Token summary
	sb.WriteString("📈 Token Summary\n")
	sb.WriteString("─────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Input tokens:  %d\n", stats.TotalInput))
	sb.WriteString(fmt.Sprintf("  Output tokens: %d\n", stats.TotalOutput))
	sb.WriteString(fmt.Sprintf("  Total tokens:  %d\n", stats.TotalInput+stats.TotalOutput))
	sb.WriteString(fmt.Sprintf("  Estimated cost: $%.6f\n\n", stats.TotalCost))

	// Provider breakdown
	if len(stats.ProviderCalls) > 0 {
		sb.WriteString("🔌 Provider Calls\n")
		sb.WriteString("─────────────────────────────────────────\n")
		for model, count := range stats.ProviderCalls {
			pricing := PricingTable[model]
			if pricing.InputPricePer1K == 0 && pricing.OutputPricePer1K == 0 {
				sb.WriteString(fmt.Sprintf("  %s: %d calls (local/free)\n", model, count))
			} else {
				sb.WriteString(fmt.Sprintf("  %s: %d calls\n", model, count))
			}
		}
		sb.WriteString("\n")
	}

	// Step breakdown
	if len(stats.Steps) > 0 {
		sb.WriteString("📝 Step Breakdown\n")
		sb.WriteString("─────────────────────────────────────────\n")
		for _, step := range stats.Steps {
			sb.WriteString(fmt.Sprintf("  %s (%s)\n", step.StepName, step.NodeType))
			if step.Model != "" {
				sb.WriteString(fmt.Sprintf("    Model: %s\n", step.Model))
			}
			if step.TotalTokens > 0 {
				sb.WriteString(fmt.Sprintf("    Tokens: %d in / %d out\n", step.InputTokens, step.OutputTokens))
				if step.Cost > 0 {
					sb.WriteString(fmt.Sprintf("    Cost: $%.6f\n", step.Cost))
				}
			}
			sb.WriteString(fmt.Sprintf("    Duration: %v\n", step.Duration.Round(time.Millisecond)))
		}
	}

	return sb.String()
}

// FormatCompactReport generates a compact one-line report.
func (s *StatsCollector) FormatCompactReport(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.stats[name]
	if !ok {
		return ""
	}

	return fmt.Sprintf("📊 Tokens: %d in / %d out | Cost: $%.4f | Time: %v",
		stats.TotalInput,
		stats.TotalOutput,
		stats.TotalCost,
		stats.Duration.Round(time.Millisecond),
	)
}

// calculateCost calculates the cost for a given model and token usage.
func calculateCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := PricingTable[model]
	if !ok {
		// Default to GPT-4o-mini pricing for unknown models
		pricing = ModelPricing{0.00015, 0.0006}
	}

	inputCost := float64(inputTokens) / 1000 * pricing.InputPricePer1K
	outputCost := float64(outputTokens) / 1000 * pricing.OutputPricePer1K

	return inputCost + outputCost
}

// EstimateTokens estimates token count from text.
// This is a rough estimate: ~4 characters per token for English.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Average: 1 token ≈ 4 characters for English
	// For code: 1 token ≈ 3 characters
	return len(text) / 4
}

// GlobalStatsCollector is the default stats collector.
var GlobalStatsCollector = NewStatsCollector()

// StartWorkflow starts tracking a workflow using the global collector.
func StartWorkflow(name string) {
	GlobalStatsCollector.StartWorkflow(name)
}

// RecordStep records a step using the global collector.
func RecordStep(stepName, nodeType, model string, inputTokens, outputTokens int, duration time.Duration) {
	GlobalStatsCollector.RecordStep(stepName, nodeType, model, inputTokens, outputTokens, duration)
}

// EndWorkflow ends tracking using the global collector.
func EndWorkflow(name string) {
	GlobalStatsCollector.EndWorkflow(name)
}

// GetStats returns stats using the global collector.
func GetStats(name string) *WorkflowStats {
	return GlobalStatsCollector.GetStats(name)
}

// FormatReport formats a report using the global collector.
func FormatReport(name string) string {
	return GlobalStatsCollector.FormatReport(name)
}

// FormatCompactReport formats a compact report using the global collector.
func FormatCompactReport(name string) string {
	return GlobalStatsCollector.FormatCompactReport(name)
}