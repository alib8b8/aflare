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

package core

import (
	"testing"
)

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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
