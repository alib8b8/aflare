package stats

import (
	"testing"
	"time"
)

func TestNewStatsCollector(t *testing.T) {
	collector := NewStatsCollector()
	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}
}

func TestStartEndWorkflow(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("test-workflow")

	stats := collector.GetCurrentStats()
	if stats == nil {
		t.Fatal("Expected non-nil stats after StartWorkflow")
	}
	if stats.WorkflowName != "test-workflow" {
		t.Errorf("Expected workflow name 'test-workflow', got '%s'", stats.WorkflowName)
	}

	collector.EndWorkflow("test-workflow")

	stats = collector.GetStats("test-workflow")
	if stats == nil {
		t.Fatal("Expected non-nil stats after EndWorkflow")
	}
	if stats.Duration == 0 {
		t.Error("Expected non-zero duration after EndWorkflow")
	}
}

func TestRecordStep(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("test-workflow")

	collector.RecordStep("step1", "ollama", "llama2", 100, 50, 100*time.Millisecond)

	stats := collector.GetStats("test-workflow")
	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}

	if len(stats.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(stats.Steps))
	}

	if stats.TotalInput != 100 {
		t.Errorf("Expected total input 100, got %d", stats.TotalInput)
	}

	if stats.TotalOutput != 50 {
		t.Errorf("Expected total output 50, got %d", stats.TotalOutput)
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model        string
		inputTokens  int
		outputTokens int
		wantCost     bool // true if cost > 0
	}{
		{"gpt-4o", 1000, 500, true},
		{"llama2", 1000, 500, false},       // local model, free
		{"unknown-model", 1000, 500, true}, // uses default pricing
	}

	for _, tt := range tests {
		cost := calculateCost(tt.model, tt.inputTokens, tt.outputTokens)
		if tt.wantCost && cost <= 0 {
			t.Errorf("Expected positive cost for %s, got %f", tt.model, cost)
		}
		if !tt.wantCost && cost != 0 {
			t.Errorf("Expected zero cost for %s (local model), got %f", tt.model, cost)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		minToken int
		maxToken int
	}{
		{"Hello, world!", 3, 5},
		{"", 0, 0},
		{"This is a longer piece of text for testing.", 10, 15},
	}

	for _, tt := range tests {
		tokens := EstimateTokens(tt.text)
		if tokens < tt.minToken || tokens > tt.maxToken {
			t.Errorf("EstimateTokens(%q) = %d, want between %d and %d",
				tt.text, tokens, tt.minToken, tt.maxToken)
		}
	}
}

func TestFormatReport(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("test-workflow")
	collector.RecordStep("step1", "ollama", "llama2", 100, 50, 100*time.Millisecond)
	collector.EndWorkflow("test-workflow")

	report := collector.FormatReport("test-workflow")
	if report == "" {
		t.Error("Expected non-empty report")
	}

	// Check key elements
	if !contains(report, "Token Usage Report") {
		t.Error("Expected 'Token Usage Report' in report")
	}
	if !contains(report, "test-workflow") {
		t.Error("Expected workflow name in report")
	}
}

func TestFormatCompactReport(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("test-workflow")
	collector.RecordStep("step1", "ollama", "llama2", 100, 50, 100*time.Millisecond)
	collector.EndWorkflow("test-workflow")

	report := collector.FormatCompactReport("test-workflow")
	if report == "" {
		t.Error("Expected non-empty compact report")
	}
}

func TestGlobalStatsCollector(t *testing.T) {
	StartWorkflow("global-test")
	RecordStep("step1", "ollama", "llama2", 100, 50, 100*time.Millisecond)
	EndWorkflow("global-test")

	stats := GetStats("global-test")
	if stats == nil {
		t.Fatal("Expected non-nil stats from global collector")
	}

	report := FormatReport("global-test")
	if report == "" {
		t.Error("Expected non-empty report from global collector")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
