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

func TestGetModelPricing(t *testing.T) {
	tests := []struct {
		model  string
		wantOk bool
		free   bool
	}{
		{"gpt-4o", true, false},
		{"gpt-4o-mini", true, false},
		{"claude-3-5-sonnet", true, false},
		{"llama2", true, true},
		{"llama3.1", true, true},
		{"deepseek-r1", true, true},
		{"nonexistent-model", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing, ok := GetModelPricing(tt.model)
			if ok != tt.wantOk {
				t.Errorf("GetModelPricing(%q) ok = %v, want %v", tt.model, ok, tt.wantOk)
			}
			if ok && tt.free {
				if pricing.InputPricePer1K != 0 || pricing.OutputPricePer1K != 0 {
					t.Errorf("Expected free pricing for %s, got input=%f output=%f",
						tt.model, pricing.InputPricePer1K, pricing.OutputPricePer1K)
				}
			}
		})
	}
}

func TestRecordStepNoWorkflow(t *testing.T) {
	collector := NewStatsCollector()
	collector.RecordStep("step1", "ollama", "llama2", 100, 50, 100*time.Millisecond)

	stats := collector.GetCurrentStats()
	if stats != nil {
		t.Error("Expected nil stats when no workflow started")
	}
}

func TestGetStatsNonexistent(t *testing.T) {
	collector := NewStatsCollector()
	stats := collector.GetStats("nonexistent")
	if stats != nil {
		t.Error("Expected nil stats for nonexistent workflow")
	}
}

func TestGetCurrentStatsNone(t *testing.T) {
	collector := NewStatsCollector()
	stats := collector.GetCurrentStats()
	if stats != nil {
		t.Error("Expected nil current stats when none active")
	}
}

func TestFormatReportNonexistent(t *testing.T) {
	collector := NewStatsCollector()
	report := collector.FormatReport("nonexistent")
	if report != "No statistics available" {
		t.Errorf("Expected 'No statistics available', got %q", report)
	}
}

func TestFormatCompactReportNonexistent(t *testing.T) {
	collector := NewStatsCollector()
	report := collector.FormatCompactReport("nonexistent")
	if report != "" {
		t.Errorf("Expected empty compact report, got %q", report)
	}
}

func TestModelCallsTracking(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("test-workflow")
	collector.RecordStep("step1", "ollama", "gpt-4o", 100, 50, 10*time.Millisecond)
	collector.RecordStep("step2", "ollama", "gpt-4o", 200, 100, 20*time.Millisecond)
	collector.RecordStep("step3", "ollama", "llama2", 50, 25, 5*time.Millisecond)
	collector.EndWorkflow("test-workflow")

	stats := collector.GetStats("test-workflow")
	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}
	if stats.ModelCalls["gpt-4o"] != 2 {
		t.Errorf("Expected 2 calls for gpt-4o, got %d", stats.ModelCalls["gpt-4o"])
	}
	if stats.ModelCalls["llama2"] != 1 {
		t.Errorf("Expected 1 call for llama2, got %d", stats.ModelCalls["llama2"])
	}
}

func TestCostAggregation(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("cost-test")
	collector.RecordStep("s1", "ollama", "gpt-4o-mini", 1000, 500, 10*time.Millisecond)
	collector.RecordStep("s2", "ollama", "gpt-4o-mini", 2000, 1000, 10*time.Millisecond)
	collector.EndWorkflow("cost-test")

	stats := collector.GetStats("cost-test")
	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}
	if stats.TotalCost <= 0 {
		t.Error("Expected positive total cost")
	}
	if stats.TotalInput != 3000 {
		t.Errorf("Expected total input 3000, got %d", stats.TotalInput)
	}
	if stats.TotalOutput != 1500 {
		t.Errorf("Expected total output 1500, got %d", stats.TotalOutput)
	}
}

func TestGlobalFormatCompactReport(t *testing.T) {
	StartWorkflow("compact-test")
	RecordStep("step1", "ollama", "llama2", 100, 50, 10*time.Millisecond)
	EndWorkflow("compact-test")

	report := FormatCompactReport("compact-test")
	if report == "" {
		t.Error("Expected non-empty compact report from global")
	}
}

func TestEndWorkflowNotCurrent(t *testing.T) {
	collector := NewStatsCollector()
	collector.StartWorkflow("wf1")
	collector.StartWorkflow("wf2")
	collector.EndWorkflow("wf1")

	stats := collector.GetCurrentStats()
	if stats == nil {
		t.Fatal("Expected current stats to still be wf2")
	}
	if stats.WorkflowName != "wf2" {
		t.Errorf("Expected current workflow 'wf2', got '%s'", stats.WorkflowName)
	}
}

func TestCopyStatsNil(t *testing.T) {
	result := copyStats(nil)
	if result != nil {
		t.Error("Expected nil for copyStats(nil)")
	}
}

func TestSecurityStats(t *testing.T) {
	stats := GetSecurityStats()
	if stats == nil {
		t.Fatal("Expected non-nil security stats")
	}
}

func TestSecurityRecordBlock(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}

	stats.RecordBlock(BlockPathTraversal, "/etc/passwd attempt", "file_read")
	stats.RecordBlock(BlockCommandInjection, "; rm -rf", "execute")
	stats.RecordBlock(BlockSSRF, "http://169.254.169.254", "fetch_url")

	if stats.TotalBlocks != 3 {
		t.Errorf("Expected 3 total blocks, got %d", stats.TotalBlocks)
	}
	if stats.ByType[BlockPathTraversal] != 1 {
		t.Errorf("Expected 1 path traversal block, got %d", stats.ByType[BlockPathTraversal])
	}
	if stats.ByNode["file_read"] != 1 {
		t.Errorf("Expected 1 file_read node block, got %d", stats.ByNode["file_read"])
	}
	if len(stats.RecentEvents) != 3 {
		t.Errorf("Expected 3 recent events, got %d", len(stats.RecentEvents))
	}
}

func TestSecurityRecordBlockNoNode(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}

	stats.RecordBlock(BlockSafeMode, "action blocked by safe mode", "")
	if stats.ByNode[""] != 0 {
		t.Error("Expected empty node to not be tracked")
	}
}

func TestSecurityRecordSecurityLevel(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}

	stats.RecordSecurityLevel("strict")
	stats.RecordSecurityLevel("strict")
	stats.RecordSecurityLevel("permissive")

	if stats.SecurityLevelUsed["strict"] != 2 {
		t.Errorf("Expected 2 strict uses, got %d", stats.SecurityLevelUsed["strict"])
	}
	if stats.SecurityLevelUsed["permissive"] != 1 {
		t.Errorf("Expected 1 permissive use, got %d", stats.SecurityLevelUsed["permissive"])
	}
}

func TestSecurityRecordCodeInterpreterRun(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}

	stats.RecordCodeInterpreterRun(100, false, false)
	stats.RecordCodeInterpreterRun(200, true, false)
	stats.RecordCodeInterpreterRun(150, false, true)

	if stats.CodeInterpreter.TotalRuns != 3 {
		t.Errorf("Expected 3 total runs, got %d", stats.CodeInterpreter.TotalRuns)
	}
	if stats.CodeInterpreter.BlockedCount != 1 {
		t.Errorf("Expected 1 blocked, got %d", stats.CodeInterpreter.BlockedCount)
	}
	if stats.CodeInterpreter.TimeoutCount != 1 {
		t.Errorf("Expected 1 timeout, got %d", stats.CodeInterpreter.TimeoutCount)
	}
	if stats.CodeInterpreter.AvgMs != 150 {
		t.Errorf("Expected avg 150ms, got %d", stats.CodeInterpreter.AvgMs)
	}
}

func TestSecurityRecordHTTPRequest(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}

	stats.RecordHTTPRequest(false, false)
	stats.RecordHTTPRequest(true, false)
	stats.RecordHTTPRequest(false, true)
	stats.RecordHTTPRequest(true, true)

	if stats.HTTPRequests.TotalRequests != 4 {
		t.Errorf("Expected 4 total requests, got %d", stats.HTTPRequests.TotalRequests)
	}
	if stats.HTTPRequests.BlockedSSRF != 2 {
		t.Errorf("Expected 2 SSRF blocks, got %d", stats.HTTPRequests.BlockedSSRF)
	}
	if stats.HTTPRequests.BlockedNonHTTP != 2 {
		t.Errorf("Expected 2 non-HTTP blocks, got %d", stats.HTTPRequests.BlockedNonHTTP)
	}
}

func TestSecurityToJSON(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}
	stats.RecordBlock(BlockPathTraversal, "test", "node1")

	jsonStr, err := stats.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if jsonStr == "" {
		t.Error("Expected non-empty JSON")
	}
	if !contains(jsonStr, "total_blocks") {
		t.Error("Expected JSON to contain 'total_blocks'")
	}
}

func TestSecurityFormatReport(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}
	stats.RecordBlock(BlockPathTraversal, "test", "node1")
	stats.RecordSecurityLevel("strict")

	report := stats.FormatReport()
	if report == "" {
		t.Error("Expected non-empty security report")
	}
	if !contains(report, "Security Stats Report") {
		t.Error("Expected 'Security Stats Report' in report")
	}
	if !contains(report, "Code Interpreter") {
		t.Error("Expected 'Code Interpreter' section in report")
	}
	if !contains(report, "HTTP Requests") {
		t.Error("Expected 'HTTP Requests' section in report")
	}
}

func TestSecurityRecentEventsCap(t *testing.T) {
	stats := &SecurityStats{
		ByType:            make(map[SecurityBlockType]int64),
		ByNode:            make(map[string]int64),
		SecurityLevelUsed: make(map[string]int64),
		RecentEvents:      make([]SecurityEvent, 0, 100),
	}

	for i := 0; i < 150; i++ {
		stats.RecordBlock(BlockSafeMode, "test", "")
	}

	if len(stats.RecentEvents) != 100 {
		t.Errorf("Expected recent events capped at 100, got %d", len(stats.RecentEvents))
	}
}
